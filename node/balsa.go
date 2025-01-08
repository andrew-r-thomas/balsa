/*

NOTE:
log index starts at 1 logically

*/

package main

import (
	"context"
	"log"
	"math/rand/v2"
	"net"
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
}

func NewBalsa(id string, siblingPorts map[string]string) Balsa {
	sibClients := make(map[string]pb.RaftClient, len(siblingPorts))
	resChan := make(chan sibResp, len(siblingPorts))
	for sibId, sibPort := range siblingPorts {
		opts := grpc.WithTransportCredentials(insecure.NewCredentials())
		conn, err := grpc.NewClient(":"+sibPort, opts)
		if err != nil {
			log.Fatalf("failed to create sibling connection: %v\n", err)
		}
		client := pb.NewRaftClient(conn)
		sibClients[sibId] = client
	}
	sibs := siblings{
		clients: sibClients,
		resChan: resChan,
	}

	elChan := make(chan struct{}, 1)

	return Balsa{id: id, sibs: sibs, elChan: elChan}
}
func (balsa *Balsa) Start(grpcPort string) {
	go balsa.serveGrpc(grpcPort)
	time.Sleep(time.Second)
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
		if balsa.votedFor == "" || balsa.votedFor == request.GetCandidateId() {
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
		log.Fatalf("error starting lisener: %v\n", err)
	}
	log.Printf("lisenting on %s\n", port)
	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatalf("failed to serve grpc: %v\n", err)
	}
}

type elState int

const (
	follower elState = iota
	candidate
	leader
)

func (balsa *Balsa) startElLoop() {
	state := follower
elLoop:
	for {
		switch state {
		case follower:
			to := getTimeout()
			select {
			case <-to:
				state = candidate
				continue
			case <-balsa.elChan:
				continue
			}
		case candidate:
			balsa.currentTerm += 1
			balsa.votedFor = balsa.id

			to := getTimeout()

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
						log.Printf("error requesting vote: %v\n", err)
					} else {
						balsa.sibs.resChan <- sibResp{rvResp: res, id: id}
					}
				}()
			}

			votes := 0
			for {
				select {
				case <-to:
					continue elLoop
				case res := <-balsa.sibs.resChan:
					if res.rvResp.GetVoteGranted() {
						votes += 1
						if votes >= len(balsa.sibs.clients) {
							state = leader
							continue elLoop
						}
					} else {
						log.Printf("vote not grated\n")
					}
				case <-balsa.elChan:
					state = follower
					continue elLoop
				}
			}
		case leader:
			to := getTimeout()
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
						log.Printf("append entries failed: %v\n", err)
					} else {
						balsa.sibs.resChan <- sibResp{id: id, aeResp: res}
					}

				}()
			}

			for {
				select {
				case <-to:
					continue elLoop
				case res := <-balsa.sibs.resChan:
					if !res.aeResp.GetSuccess() {
						if res.aeResp.GetTerm() > balsa.currentTerm {
							balsa.currentTerm = res.aeResp.GetTerm()
							balsa.votedFor = res.id
							state = follower
							continue elLoop
						}
					}
				case <-balsa.elChan:
					state = follower
					continue elLoop
				}
			}
		}
	}
}

func getTimeout() <-chan time.Time {
	return time.After(
		time.Duration(rand.IntN(toMax-toMin)+toMin) * time.Millisecond,
	)
}
