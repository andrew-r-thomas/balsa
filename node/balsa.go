/*

NOTE:
log index starts at 1 logically

*/

package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/andrew-r-thomas/balsa/node/grpc"
)

const toMin int = 150
const toMax int = 300

type Balsa struct {
	id   string
	sibs map[string]pb.RaftClient

	pb.UnimplementedRaftServer

	// TODO: persitent state
	currentTerm uint64
	votedFor    string

	state elState

	followChan chan struct{}
	aeChan     chan struct{}

	ctx    context.Context
	logger *slog.Logger

	store *Store
}

func NewBalsa(
	id string,
	siblingPorts map[string]string,
	logger *slog.Logger,
	store *Store,
) Balsa {
	// set up siblings
	sibs := make(map[string]pb.RaftClient, len(siblingPorts))
	for sibId, sibPort := range siblingPorts {
		opts := grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		)
		conn, err := grpc.NewClient(":"+sibPort, opts)
		if err != nil {
			logger.Error(
				fmt.Sprintf(
					"failed to create sibling connection: %v\n",
					err,
				),
			)
			os.Exit(1)
		}
		client := pb.NewRaftClient(conn)
		sibs[sibId] = client
	}
	followChan := make(chan struct{}, 1)
	aeChan := make(chan struct{}, 1)

	return Balsa{
		id:         id,
		sibs:       sibs,
		followChan: followChan,
		aeChan:     aeChan,
		logger:     logger,
		ctx:        context.Background(),
		state:      follower,
		store:      store,
	}
}
func (balsa *Balsa) Start(grpcPort string) {
	go balsa.serveGrpc(grpcPort)
	balsa.run()
}

func (balsa *Balsa) AddLog(key string, val int) error {
	// TODO:
	return nil
}

func (balsa *Balsa) AppendEntries(
	ctx context.Context,
	request *pb.AppendEntriesRequest,
) (*pb.AppendEntriesResponse, error) {
	term := balsa.getCurrentTerm()
	reqTerm := request.GetTerm()

	if reqTerm < term {
		return &pb.AppendEntriesResponse{
			Success: false,
			Term:    term,
		}, nil
	}

	if reqTerm > term {
		balsa.setCurrentTerm(reqTerm)
		term = reqTerm
	}

	balsa.followChan <- struct{}{}

	return &pb.AppendEntriesResponse{
		Success: true,
		Term:    term,
	}, nil
}
func (balsa *Balsa) RequestVote(
	ctx context.Context,
	request *pb.RequestVoteRequest,
) (*pb.RequestVoteResponse, error) {
	term := balsa.getCurrentTerm()
	votedFor := balsa.getVotedFor()

	if request.Term < term {
		return &pb.RequestVoteResponse{
			VoteGranted: false,
			Term:        term,
		}, nil
	}

	if request.Term > term ||
		(request.Term == term &&
			(votedFor == "" ||
				votedFor == request.CandidateId)) {
		balsa.setCurrentTerm(request.Term)
		balsa.setVotedFor(request.GetCandidateId())

		term = request.Term
		balsa.followChan <- struct{}{}

		return &pb.RequestVoteResponse{
			VoteGranted: true,
			Term:        term,
		}, nil
	}

	return &pb.RequestVoteResponse{
		VoteGranted: false,
		Term:        term,
	}, nil
}

func (balsa *Balsa) serveGrpc(port string) {
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterRaftServer(grpcServer, balsa)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		balsa.logger.Error(
			fmt.Sprintf("error starting lisener: %v\n", err),
		)
		os.Exit(1)
	}
	err = grpcServer.Serve(listener)
	if err != nil {
		balsa.logger.Error(
			fmt.Sprintf("failed to serve grpc: %v\n", err),
		)
	}
}

type elState int

const (
	follower elState = iota
	candidate
	leader
)

func (s elState) String() string {
	switch s {
	case follower:
		return "follower"
	case candidate:
		return "candidate"
	case leader:
		return "leader"
	default:
		return "error parsing elState"
	}
}

func (balsa *Balsa) run() {
	for {
		switch balsa.state {
		case follower:
			balsa.runFollower()
		case candidate:
			balsa.runCandidate()
		case leader:
			balsa.runLeader()
		}
	}
}

func (balsa *Balsa) runFollower() {
	to := getTimeout()
	for {
		select {
		case <-time.After(to):
			balsa.state = candidate
			balsa.logUpdate("state", balsa.state.String())
			return
		case <-balsa.followChan:
			continue
		}
	}
}
func (balsa *Balsa) runCandidate() {
	to := getTimeout()
	for {
		term := balsa.incCurrentTerm()
		timer := time.After(to)

		votesGranted := make(chan struct{}, 1)
		go func() {
			votes := make(chan bool)
			for _, n := range balsa.sibs {
				go func() {
					resp, err := n.RequestVote(
						balsa.ctx,
						&pb.RequestVoteRequest{
							Term:        term,
							CandidateId: balsa.id,
						},
					)
					if err != nil {
						balsa.logger.Error("...but thats...IMPOSSIBLE!\n")
						os.Exit(1)
					}

					if resp.Term > term {
						balsa.setCurrentTerm(resp.Term)
						balsa.followChan <- struct{}{}
					} else {
						votes <- resp.VoteGranted
					}

				}()
			}

			numVotes := 0
			for vote := range votes {
				if vote {
					numVotes += 1
					if numVotes >= len(balsa.sibs)/2 {
						votesGranted <- struct{}{}
					}
				}
			}
		}()

		select {
		case <-timer:
			continue
		case <-balsa.followChan:
			balsa.state = follower
			balsa.logUpdate("state", balsa.state.String())
			return
		case <-votesGranted:
			balsa.state = leader
			balsa.logUpdate("state", balsa.state.String())
			return
		}
	}
}
func (balsa *Balsa) runLeader() {
	to := getTimeout()
	term := balsa.getCurrentTerm()
	for {
		for _, n := range balsa.sibs {
			// TODO: we need to add a context for these kinds of things
			go func() {
				resp, err := n.AppendEntries(
					balsa.ctx,
					&pb.AppendEntriesRequest{
						Term:     term,
						LeaderId: balsa.id,
					},
				)
				if err != nil {
					balsa.logger.Error("...but thats...IMPOSSIBLErrr!\n")
					os.Exit(1)
				}

				if !resp.Success {
					if resp.Term > term {
						balsa.setCurrentTerm(resp.Term)
						balsa.followChan <- struct{}{}
					}
				}
			}()
		}

		select {
		case <-time.After(to):
			continue
		case <-balsa.aeChan:
			continue
		case <-balsa.followChan:
			balsa.state = follower
			balsa.logUpdate("state", balsa.state.String())
			return
		}
	}
}

func (balsa *Balsa) getCurrentTerm() uint64 {
	return balsa.currentTerm
}
func (balsa *Balsa) setCurrentTerm(term uint64) {
	balsa.setVotedFor("")
	balsa.currentTerm = term
	balsa.logUpdate("term", balsa.currentTerm)
}
func (balsa *Balsa) incCurrentTerm() uint64 {
	balsa.setVotedFor(balsa.id)
	balsa.currentTerm += 1
	balsa.logUpdate("term", balsa.currentTerm)
	return balsa.currentTerm
}

func (balsa *Balsa) getVotedFor() string {
	return balsa.votedFor
}
func (balsa *Balsa) setVotedFor(id string) {
	balsa.votedFor = id
	balsa.logUpdate("leader", balsa.votedFor)
}

func getTimeout() time.Duration {
	return time.Duration(rand.IntN(toMax-toMin)+toMin) * time.Millisecond
}

func (balsa *Balsa) logUpdate(msg string, val interface{}) {
	balsa.logger.Info(msg, "node", balsa.id, msg, val)
}
