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

type siblings struct {
	clients map[string]pb.RaftClient
	resChan chan sibResp
}
type sibResp struct {
	id     string
	rvResp *pb.RequestVoteResponse
	aeResp *pb.AppendEntriesResponse
}

type Balsa struct {
	id   string
	sibs siblings

	pb.UnimplementedRaftServer

	// TODO: persitent state
	currentTerm uint64
	votedFor    string

	elChan chan struct{}

	logger *slog.Logger
}

func NewBalsa(
	id string,
	siblingPorts map[string]string,
	logger *slog.Logger,
) Balsa {
	// set up siblings
	sibClients := make(map[string]pb.RaftClient, len(siblingPorts))
	resChan := make(chan sibResp, len(siblingPorts))
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
		sibClients[sibId] = client
	}
	sibs := siblings{
		clients: sibClients,
		resChan: resChan,
	}

	elChan := make(chan struct{}, 1)

	return Balsa{id: id, sibs: sibs, elChan: elChan, logger: logger}
}
func (balsa *Balsa) Start(grpcPort string) {
	go balsa.serveGrpc(grpcPort)
	balsa.startElLoop()
}

func (balsa *Balsa) AppendEntries(
	ctx context.Context,
	request *pb.AppendEntriesRequest,
) (*pb.AppendEntriesResponse, error) {
	reqTerm := request.GetTerm()

	if reqTerm < balsa.currentTerm {
		return &pb.AppendEntriesResponse{
			Term:    balsa.currentTerm,
			Success: false,
		}, nil
	}

	balsa.currentTerm = reqTerm
	balsa.votedFor = request.GetLeaderId()
	balsa.elChan <- struct{}{}

	return &pb.AppendEntriesResponse{
		Term:    balsa.currentTerm,
		Success: true,
	}, nil
}
func (balsa *Balsa) RequestVote(
	ctx context.Context,
	request *pb.RequestVoteRequest,
) (*pb.RequestVoteResponse, error) {
	switch reqTerm := request.GetTerm(); {
	case reqTerm < balsa.currentTerm:
		return &pb.RequestVoteResponse{
			VoteGranted: false,
			Term:        balsa.currentTerm,
		}, nil
	case reqTerm == balsa.currentTerm:
		if balsa.votedFor == "" ||
			balsa.votedFor == request.GetCandidateId() {
			balsa.votedFor = request.GetCandidateId()
			balsa.currentTerm = request.GetTerm()
			balsa.elChan <- struct{}{}
			return &pb.RequestVoteResponse{
				VoteGranted: true,
				Term:        balsa.currentTerm,
			}, nil
		}
	case reqTerm > balsa.currentTerm:
		balsa.votedFor = request.GetCandidateId()
		balsa.currentTerm = request.GetTerm()
		balsa.elChan <- struct{}{}
		return &pb.RequestVoteResponse{
			VoteGranted: true,
			Term:        balsa.currentTerm,
		}, nil

	}

	return &pb.RequestVoteResponse{
		VoteGranted: false,
		Term:        balsa.currentTerm,
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

func (balsa *Balsa) startElLoop() {
	state := follower
	to := getTimeout()
	balsa.logState(state)
elLoop:
	for {
		switch state {
		case follower:
			select {
			case <-time.After(to):
				state = candidate
				continue
			case <-balsa.elChan:
				continue
			}
		case candidate:
			balsa.currentTerm += 1
			balsa.votedFor = balsa.id

			balsa.logState(state)

			to = getTimeout()

			for id, sibClient := range balsa.sibs.clients {
				go func() {
					res, err := sibClient.RequestVote(
						context.Background(),
						&pb.RequestVoteRequest{
							Term:        balsa.currentTerm,
							CandidateId: balsa.id,
						},
					)
					if err != nil {
						balsa.logger.Error(
							fmt.Sprintf(
								"error requesting vote: %v\n",
								err,
							),
						)
					} else {
						balsa.sibs.resChan <- sibResp{
							rvResp: res,
							id:     id,
						}
					}
				}()
			}

			votes := 0
			for {
				select {
				case <-time.After(to):
					continue elLoop
				case res := <-balsa.sibs.resChan:
					if res.rvResp.GetVoteGranted() {
						votes += 1
						if votes >= len(balsa.sibs.clients) {
							to = getTimeout()
							state = leader
							balsa.logState(state)
							continue elLoop
						}
					}
				case <-balsa.elChan:
					to = getTimeout()
					state = follower
					balsa.logState(state)
					continue elLoop
				}
			}
		case leader:
			for id, sibClient := range balsa.sibs.clients {
				go func() {
					res, err := sibClient.AppendEntries(
						context.Background(),
						&pb.AppendEntriesRequest{
							Term:     balsa.currentTerm,
							LeaderId: balsa.id,
						},
					)
					if err != nil {
						balsa.logger.Error(
							fmt.Sprintf(
								"append entries failed: %v\n",
								err,
							),
						)
					} else {
						balsa.sibs.resChan <- sibResp{
							id:     id,
							aeResp: res,
						}
					}

				}()
			}

			for {
				select {
				case <-time.After(to):
					continue elLoop
				case res := <-balsa.sibs.resChan:
					if !res.aeResp.GetSuccess() {
						if res.aeResp.GetTerm() > balsa.currentTerm {
							to = getTimeout()
							balsa.currentTerm = res.aeResp.GetTerm()
							balsa.votedFor = res.id
							state = follower
							balsa.logState(state)
							continue elLoop
						}
					}
				case <-balsa.elChan:
					to = getTimeout()
					state = follower
					balsa.logState(state)
					continue elLoop
				}
			}
		}
	}
}

func getTimeout() time.Duration {
	return time.Duration(rand.IntN(toMax-toMin)+toMin) * time.Millisecond
}

func (balsa *Balsa) logState(state elState) {
	balsa.logger.Info(
		"",
		"node",
		balsa.id,
		"state",
		state.String(),
		"leader",
		balsa.votedFor,
		"term",
		balsa.currentTerm,
	)
}
