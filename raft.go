package main

import (
	"context"
	"log"
	"math/rand/v2"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	raft "balsa/raft"
)

// election timeout range in ms
const eltoS = 150
const eltoE = 300

const hbS = 100
const hbE = 400

type ServerState int

const (
	StateLeader ServerState = iota
	StateCandidate
	StateFollower
)

// TODO: maybe actual ids for nodes
type RaftServiceServer struct {
	raft.UnimplementedRaftServiceServer

	addr  string
	nodes map[string]raft.RaftServiceClient
	// state ServerState

	currentTermLock sync.RWMutex
	currentTerm     uint32
	leader          string

	eventChan chan<- event
}

type event struct{}

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
		// state:       StateFollower,
		addr:      addr,
		eventChan: nil,
	}
}

func (s *RaftServiceServer) Start() {
	// TODO: figure out if this should be buffered
	eventChan := make(chan event)
	s.eventChan = eventChan
	go s.startFollowing(eventChan)
	// TODO:
}

func (s *RaftServiceServer) startFollowing(eventChan <-chan event) {
	ms := rand.IntN(eltoE-eltoS) + eltoS
	dur := time.Millisecond * time.Duration(ms)
	select {
	case <-time.After(dur):
		s.startElection(eventChan)
	case <-eventChan:
		go s.startFollowing(eventChan)
	}
}

// this switches us to a candidate state
func (s *RaftServiceServer) startElection(eventChan <-chan event) {
	s.incrementCurrentTerm()
	s.updateCurrentLeader("")

	elms := rand.IntN(eltoE-eltoS) + eltoS
	eldur := time.Millisecond * time.Duration(elms)
	elTo := time.After(eldur)

	s.currentTermLock.RLock()
	currentTerm := s.currentTerm
	s.currentTermLock.Unlock()
	won := make(chan bool, 1)
	go func() {
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
						Term: currentTerm,
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
		go s.startElection(eventChan)
	case <-eventChan:
		go s.startFollowing(eventChan)
	case <-won:
		go s.startLeading(eventChan)
	}

}

func (s *RaftServiceServer) startLeading(eventChan <-chan event) {
	hbms := rand.IntN(hbE-hbS) + hbS
	hbdur := time.Millisecond * time.Duration(hbms)
	select {
	case <-time.After(hbdur):
		// TODO: go func style wait group for getting results of
		// append entries calls
	case <-eventChan:
		// TODO:
	}
}

func (s *RaftServiceServer) incrementCurrentTerm() {
	// TODO:
}
func (s *RaftServiceServer) updateCurrentLeader(leader string) {
	// TODO:
}

func (s *RaftServiceServer) AppendEntries(ctx context.Context, r *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {
	if s.currentTerm > r.Term {
		return &raft.AppendEntriesResponse{Term: s.currentTerm, Success: false}, nil
	} else {
		s.currentTerm = r.Term
		return &raft.AppendEntriesResponse{Term: s.currentTerm, Success: true}, nil
	}
}

func (s *RaftServiceServer) RequestVote(ctx context.Context, r *raft.RequestVoteRequest) (*raft.RequestVoteResponse, error) {
	if s.currentTerm > r.Term {
		return &raft.RequestVoteResponse{Term: s.currentTerm, Granted: false}, nil
	} else if s.currentTerm == r.Term && s.leader == "" {
		s.leader = r.Addr
		return &raft.RequestVoteResponse{Term: s.currentTerm, Granted: true}, nil
	} else {
		s.leader = r.Addr
		s.currentTerm = r.Term
		return &raft.RequestVoteResponse{Term: s.currentTerm, Granted: true}, nil
	}
}
