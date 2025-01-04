package main

import (
	"context"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"

	pb "github.com/andrew-r-thomas/balsa/node/grpc"
)

type Balsa struct {
	id       string
	siblings map[string]sibling

	httpPort string

	pb.UnimplementedRaftServer
	grpcPort string

	// persitent state
	currentTerm uint64
	votedFor    string
	log         []pb.LogEntry

	// volitile state
	commitIndex uint64
	lastApplied uint64

	// leader only volitile state
	nextIndex  []uint64
	matchIndex []uint64

	toChan chan struct{}
}

const etoMin int = 150
const etoMax int = 300

type sibling struct {
	addr   string
	client pb.RaftClient
}

func NewBalsa(grpcPort string, siblingPorts []string, httpPort string) Balsa {
	// make sibling connections
	log.Printf("%s: starting to make siblings\n", grpcPort)
	siblings := make([]sibling, len(siblingPorts))
	for i, sibPort := range siblingPorts {
		var opts []grpc.DialOption
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		conn, err := grpc.NewClient(":"+sibPort, opts...)
		if err != nil {
			log.Fatalf("error creating client: %v\n", err)
		}
		client := pb.NewRaftClient(conn)
		siblings[i] = sibling{client: client, addr: sibPort}
	}

	return Balsa{grpcPort: grpcPort, siblings: siblings, httpPort: httpPort}
}

func (balsa *Balsa) Start() {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		balsa.serveGrpc()
	}()
	go func() {
		defer wg.Done()
		balsa.serveHttp()
	}()

	// TODO: do we need a wait group here?
	go balsa.follow()

	wg.Wait()
}

func (balsa *Balsa) follow() {
	to := time.After(time.Duration(rand.IntN(etoMax-etoMin)+etoMin) * time.Millisecond)

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

	// increment current term
	balsa.currentTerm += 1
	// vote for self
	balsa.votedFor = balsa.id

	// reset election timer
	to := time.After(time.Duration(rand.IntN(etoMax-etoMin)+etoMin) * time.Millisecond)

	// request votes from other servers
	var resultChans []chan bool
	var wg sync.WaitGroup
	wg.Add(len(balsa.siblings))
	for _, sibling := range balsa.siblings {
		resultChan := make(chan bool, 1)
		go func() {
			defer wg.Done()
			resp, err := sibling.client.RequestVote(
				context.Background(),
				&pb.RequestVoteRequest{
					Term:         balsa.currentTerm,
					CandidateId:  balsa.id,
					LastLogIndex: uint64(len(balsa.log)),
					LastLogTerm:  balsa.log[len(balsa.log)-1].Term,
				},
			)
			if err == nil {
				resultChan <- resp.GetVoteGranted()
			}
		}()
		resultChans = append(resultChans, resultChan)
	}
	wgChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgChan)
	}()

	select {
	case <-to:
		// if election timeout elapses -> start new election
		go balsa.startElection()
	case <-balsa.toChan:
		// if recieve append entries from new leader -> convert to follower
		// TODO: gonna need to make sure this is correct
		go balsa.follow()
	case <-wgChan:
		// if recieve majority of votes -> convert to leader
		// TODO: ok so this might not work actually bc we can still get
		// votes from a majority while some siblings just don't respond
		// bc they are down or something, and we would still want to convert
		// to a leader in that case
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

func (balsa *Balsa) Hello(ctx context.Context, request *pb.HelloRequest) (*pb.HelloResponse, error) {
	peer, _ := peer.FromContext(ctx)
	log.Printf("%s: got hello request from %s with name %s\n", balsa.grpcPort, peer.Addr, request.GetName())
	resp := pb.HelloResponse{
		Response: "Hello " + request.GetName(),
	}
	return &resp, nil
}
func (balsa *Balsa) AppendEntries(ctx context.Context, request *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	// TODO:
	return &pb.AppendEntriesResponse{}, nil
}
func (balsa *Balsa) RequestVote(ctx context.Context, request *pb.RequestVoteRequest) (*pb.RequestVoteResponse, error) {
	// TODO:
	return &pb.RequestVoteResponse{}, nil
}

func (balsa *Balsa) serveHttp() {
	// serve http
	// TODO: maybe change addr to node id or something for logging,
	// bc this isn't the address that will be serving the http
	http.HandleFunc("/get", balsa.get)
	http.HandleFunc("/set", balsa.set)
	http.ListenAndServe(":"+balsa.httpPort, nil)
}

func (balsa *Balsa) set(w http.ResponseWriter, r *http.Request) {}
func (balsa *Balsa) get(w http.ResponseWriter, r *http.Request) {}

type httpServer struct {
	addr     string
	siblings *[]sibling
}
