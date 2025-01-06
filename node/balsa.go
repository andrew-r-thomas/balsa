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
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/andrew-r-thomas/balsa/node/grpc"
)

const toMin int = 150
const toMax int = 300

type sibling struct {
	client pb.RaftClient
}

type Balsa struct {
	id       string
	siblings map[string]sibling

	pb.UnimplementedRaftServer
	grpcPort string

	// TODO: persitent state
	currentTerm uint64
	votedFor    string

	toChan chan struct{}
}

func NewBalsa(id string, grpcPort string, siblingPorts map[string]string) Balsa {
	siblings := make(map[string]sibling, len(siblingPorts))
	for id, sibPort := range siblingPorts {
		var opts []grpc.DialOption
		opts = append(
			opts,
			grpc.WithTransportCredentials(
				insecure.NewCredentials(),
			),
		)

		conn, err := grpc.NewClient(":"+sibPort, opts...)
		if err != nil {
			log.Fatalf("error creating client: %v\n", err)
		}

		client := pb.NewRaftClient(conn)
		siblings[id] = sibling{client: client}
	}

	toChan := make(chan struct{})

	return Balsa{
		grpcPort: grpcPort,
		siblings: siblings,
		id:       id,
		toChan:   toChan,
	}
}
func (balsa *Balsa) Start() {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		balsa.serveGrpc()
	}()

	// TODO: do we need a wait group here?
	go balsa.follow()

	wg.Wait()
}

func (balsa *Balsa) follow() {
	log.Printf("%s: following %s", balsa.id, balsa.votedFor)
	to := getTimeout()

	select {
	case <-balsa.toChan:
		go balsa.follow()
	case <-to:
		// start election
		go balsa.startElection()
	}
}
func (balsa *Balsa) startElection() {
	// TODO: locks and persistence oh my
	log.Printf("%s: starting election\n", balsa.id)

	// increment current term
	balsa.currentTerm += 1
	// vote for self
	balsa.votedFor = balsa.id

	// reset election timer
	to := getTimeout()

	// request votes from other servers
	votesChan := make(chan bool)
	for _, sibling := range balsa.siblings {
		go func() {
			resp, err := sibling.client.RequestVote(
				context.Background(),
				&pb.RequestVoteRequest{
					Term:        balsa.currentTerm,
					CandidateId: balsa.id,
				},
			)
			if err == nil {
				votesChan <- resp.GetVoteGranted()
			}
		}()
	}
	majVotesChan := make(chan struct{})
	go func() {
		votes := 0
		for {
			vote := <-votesChan
			if vote {
				votes += 1
				if votes >= len(balsa.siblings)/2 {
					close(majVotesChan)
					break
				}
			}
		}
	}()

	select {
	case <-to:
		// if election timeout elapses -> start new election
		go balsa.startElection()
	case <-balsa.toChan:
		// if recieve append entries from new leader -> convert to follower
		// TODO: gonna need to make sure this is correct
		go balsa.follow()
	case <-majVotesChan:
		// if recieve majority of votes -> convert to leader
		go balsa.lead()
	}
}
func (balsa *Balsa) lead() {
	log.Printf("%s: leading\n", balsa.id)
	for _, sibling := range balsa.siblings {
		go func() {
			resp, err := sibling.client.AppendEntries(
				context.Background(),
				&pb.AppendEntriesRequest{
					Term:     balsa.currentTerm,
					LeaderId: balsa.id,
				})
			if err != nil {
				log.Printf("%s: error sending append entries request: %v", balsa.id, err)
			}

			if !resp.GetSuccess() &&
				resp.GetTerm() > balsa.currentTerm {
				// TODO: deal with new leader
				log.Printf("unsuccussful heartbeat\n")
			}
		}()
	}
	to := getTimeout()

	select {
	case <-to:
		balsa.lead()
	case <-balsa.toChan:
		go balsa.follow()
	}
}

func (balsa *Balsa) serveGrpc() {
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterRaftServer(grpcServer, balsa)

	listener, err := net.Listen("tcp", ":"+balsa.grpcPort)
	if err != nil {
		log.Fatalf("error starting lisener: %v\n", err)
	}
	log.Printf("lisenting on %s\n", balsa.grpcPort)
	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatalf("failed to serve grpc: %v\n", err)
	}
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
	balsa.toChan <- struct{}{}

	return &pb.AppendEntriesResponse{
		Term:    balsa.currentTerm,
		Success: true,
	}, nil
}
func (balsa *Balsa) RequestVote(
	ctx context.Context,
	request *pb.RequestVoteRequest,
) (*pb.RequestVoteResponse, error) {
	if request.GetTerm() < balsa.currentTerm {
		return &pb.RequestVoteResponse{
			VoteGranted: false,
			Term:        balsa.currentTerm,
		}, nil
	}

	if balsa.votedFor == "" || balsa.votedFor == request.GetCandidateId() {
		balsa.votedFor = request.GetCandidateId()
		balsa.currentTerm = request.GetTerm()
		balsa.toChan <- struct{}{}
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

func getTimeout() <-chan time.Time {
	return time.After(
		time.Duration(rand.IntN(toMax-toMin)+toMin) * time.Millisecond,
	)
}
