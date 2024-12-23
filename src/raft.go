package main

import (
	"context"
	"log"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	logger "balsa/logger"
	raft "balsa/raft"
)

// TODO: ok so we are getting an issue of converting to a follower when our
// leader is ""

// election timeout range in ms
const eltoS = 1000
const eltoE = 2000

const hbS = 150
const hbE = 300

// TODO: maybe actual ids for nodes
// TODO: also technically some of this needs to be persistent,
// but we'd like to be able to spawn sims quickly, so maybe we have an option
// to start up an old system, or kill a node for a bit, and then we can also
// just reset everything
type RaftServiceServer struct {
	raft.UnimplementedRaftServiceServer
	logger.UnimplementedLogServiceServer

	// these two don't change, access them whenever
	addr  string
	nodes map[string]raft.RaftServiceClient

	// these change and need to be persistent,
	// reads should happen through lock.RLock() and lock.Unlock()
	// writes should happen through calling functions
	currentTermLock sync.RWMutex
	currentTerm     uint32
	leaderLock      sync.RWMutex
	leader          string

	// TODO: need to double check that this is triggering things correctly
	followChan chan<- follow

	streamingLogs bool
	// TODO: maybe make this buffered
	logChan chan *logger.Log
}

type follow struct{}

// TODO: add building from persistent data option
func NewRaftServiceServer(nodeAddrs []string, addr string) RaftServiceServer {
	nodes := make(map[string]raft.RaftServiceClient)
	for _, na := range nodeAddrs {
		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}

		conn, err := grpc.NewClient(na, opts...)
		if err != nil {
			log.Fatalf("error creating client conn: %v", err)
		}

		nodes[na] = raft.NewRaftServiceClient(conn)
	}

	return RaftServiceServer{
		nodes:       nodes,
		currentTerm: 0,
		leader:      "",
		addr:        addr,
		followChan:  nil,
	}
}

func (s *RaftServiceServer) Start() {
	// TODO: figure out if this should be buffered
	followChan := make(chan follow)
	s.followChan = followChan

	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		log.Fatalf("error listening: %v", err)
	}

	grpcServer := grpc.NewServer()
	raft.RegisterRaftServiceServer(grpcServer, s)

	go s.startFollowing(followChan)
	grpcServer.Serve(lis)
}

func (s *RaftServiceServer) startFollowing(followChan <-chan follow) {
	if s.streamingLogs {
		s.leaderLock.RLock()
		s.logChan <- &logger.Log{
			Addr:      s.addr,
			Event:     logger.Event_STARTED_FOLLOWING,
			EventData: &logger.Log_Leader{Leader: s.leader},
		}
		s.leaderLock.RUnlock()
	}

	ms := rand.IntN(eltoE-eltoS) + eltoS
	dur := time.Millisecond * time.Duration(ms)
	select {
	case <-time.After(dur):
		go s.startElection(followChan)
	case <-followChan:
		go s.startFollowing(followChan)
	}
}

// this switches us to a candidate state
func (s *RaftServiceServer) startElection(followChan <-chan follow) {
	if s.streamingLogs {
		s.logChan <- &logger.Log{
			Addr:  s.addr,
			Event: logger.Event_STARTED_ELECTION,
		}
	}

	s.updateCurrentTerm(0)
	s.updateCurrentLeader("self")

	elms := rand.IntN(eltoE-eltoS) + eltoS
	eldur := time.Millisecond * time.Duration(elms)
	elTo := time.After(eldur)

	won := make(chan bool, 1)
	go func() {
		s.currentTermLock.RLock()
		var wg sync.WaitGroup
		var resps []chan *raft.RequestVoteResponse
		for _, sibNode := range s.nodes {
			wg.Add(1)
			respChan := make(chan *raft.RequestVoteResponse, 1)
			resps = append(resps, respChan)
			go func() {
				resp, rvErr := sibNode.RequestVote(
					context.Background(),
					&raft.RequestVoteRequest{
						Term: s.currentTerm,
						Addr: s.addr,
					},
				)

				if rvErr != nil {
					// TODO:
				} else {
					respChan <- resp
				}
				wg.Done()
			}()
		}
		s.currentTermLock.RUnlock()
		wg.Wait()

		totalVotes := 0
		for _, respChan := range resps {
			resp := <-respChan
			if resp.Granted {
				totalVotes++
			}
		}

		won <- totalVotes >= len(s.nodes)/2

	}()

	select {
	case <-elTo:
		go s.startElection(followChan)
	case <-followChan:
		go s.startFollowing(followChan)
	case <-won:
		go s.startLeading(followChan)
	}

}

func (s *RaftServiceServer) startLeading(followChan <-chan follow) {
	if s.streamingLogs {
		s.logChan <- &logger.Log{
			Addr:  s.addr,
			Event: logger.Event_STARTED_LEADING,
		}
	}

	hbms := rand.IntN(hbE-hbS) + hbS
	hbdur := time.Millisecond * time.Duration(hbms)
	select {
	case <-time.After(hbdur):
		var wg sync.WaitGroup
		var respChans []<-chan *raft.AppendEntriesResponse
		s.currentTermLock.RLock()
		for _, sibNode := range s.nodes {
			respChan := make(chan *raft.AppendEntriesResponse, 1)
			respChans = append(respChans, respChan)
			wg.Add(1)
			go func() {
				resp, err := sibNode.AppendEntries(
					context.Background(),
					&raft.AppendEntriesRequest{
						Term: s.currentTerm,
						Addr: s.addr,
					},
				)

				if err != nil {
					// TODO:
				} else {
					respChan <- resp
				}
				wg.Done()
			}()
		}
		s.currentTermLock.RUnlock()

		wg.Wait()
		// TODO: check append entries responses
		go s.startLeading(followChan)
	case <-followChan:
		go s.startFollowing(followChan)
	}
}

// TODO: add persistence for these
// TODO: maybe want a term updated log, or we put it in every log
func (s *RaftServiceServer) updateCurrentTerm(term uint32) {
	s.currentTermLock.Lock()
	if term == 0 {
		s.currentTerm++
	} else {
		s.currentTerm = term
	}
	s.currentTermLock.Unlock()
}

func (s *RaftServiceServer) updateCurrentLeader(leader string) {
	s.leaderLock.Lock()
	s.leader = leader
	s.leaderLock.Unlock()
}

func (s *RaftServiceServer) AppendEntries(ctx context.Context, r *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {
	s.currentTermLock.RLock()
	defer s.currentTermLock.RUnlock()

	if s.currentTerm > r.Term {
		return &raft.AppendEntriesResponse{Term: s.currentTerm, Success: false}, nil
	} else {
		go func() {
			s.updateCurrentTerm(r.Term)
			s.updateCurrentLeader(r.Addr)
			s.followChan <- follow{}
		}()

		return &raft.AppendEntriesResponse{Term: r.Term, Success: true}, nil
	}
}

func (s *RaftServiceServer) RequestVote(ctx context.Context, r *raft.RequestVoteRequest) (*raft.RequestVoteResponse, error) {
	s.currentTermLock.RLock()
	defer s.currentTermLock.RUnlock()

	s.leaderLock.RLock()
	defer s.leaderLock.RUnlock()

	if (s.leader == "" || s.leader == r.Addr) && s.currentTerm <= r.Term {
		go func() {
			s.updateCurrentLeader(r.Addr)
			s.updateCurrentTerm(r.Term)
			s.followChan <- follow{}
		}()

		return &raft.RequestVoteResponse{Term: r.Term, Granted: true}, nil
	} else {
		return &raft.RequestVoteResponse{Term: s.currentTerm, Granted: false}, nil
	}
}

func (s *RaftServiceServer) GetLogStream(r *logger.GetLogStreamRequest, stream logger.LogService_GetLogStreamServer) error {
	s.streamingLogs = true
	for {
		log := <-s.logChan
		if err := stream.Send(log); err != nil {
			return err
		}
	}
}
