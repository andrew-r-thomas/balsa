package main

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"time"

	pb "balsa/raft"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// dumb that this cant be const
var node_addrs = [5]string{"localhost:50051", "localhost:50052", "localhost:50053", "localhost:50054", "localhost:50055"}

// election timeout range in ms
const el_to_ms_s = 150
const el_to_ms_e = 300

type ServerState int

const (
	StateLeader ServerState = iota
	StateCandidate
	StateFollower
)

type RaftServiceServer struct {
	pb.UnimplementedRaftServiceServer

	nodes            map[string]pb.RaftServiceClient
	current_term     int
	leader           *pb.RaftServiceClient
	state            ServerState
	restart_election chan bool
}

// TODO: we will probably want a way to build these off of a file on disk
func NewRaftServiceServer(addr string) RaftServiceServer {
	nodes := make(map[string]pb.RaftServiceClient)

	for i := range len(node_addrs) {
		na := node_addrs[i]
		if na == addr {
			continue
		}

		var grpc_client_opts = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		client_conn, client_conn_err := grpc.NewClient(node_addrs[i], grpc_client_opts...)
		if client_conn_err != nil {
			log.Fatalf("client conn error: %v", client_conn_err)
		}

		client := pb.NewRaftServiceClient(client_conn)
		nodes[addr] = client
	}

	restart_election := make(chan bool, 1)

	server := RaftServiceServer{
		nodes:            nodes,
		current_term:     0,
		leader:           nil,
		state:            StateFollower,
		restart_election: restart_election,
	}

	go server.StartElectionTimeout()

	return server
}

// TODO: this is where we left off
func (s *RaftServiceServer) StartElectionTimeout() {
	mss := rand.IntN(el_to_ms_e-el_to_ms_s) + el_to_ms_e
	select {
	case <-time.After(time.Millisecond * time.Duration(mss)):
		fmt.Printf("election timeout\n")
		s.current_term += 1
		go s.StartElectionTimeout()
	case restart := <-s.restart_election:
		if restart {
			go s.StartElectionTimeout()
		} else {
		}
	}
}

func (s RaftServiceServer) AppendEntries(ctx context.Context, r *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	return &pb.AppendEntriesResponse{Pong: "world"}, nil
}

func (s RaftServiceServer) RequestVote(ctx context.Context, r *pb.RequestVoteRequest) (*pb.RequestVoteResponse, error) {
	return &pb.RequestVoteResponse{}, nil
}

func main() {
	addr_idx, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("incorrect index for node: %v", err)
	}
	var addr = node_addrs[addr_idx]
	listener, listen_err := net.Listen("tcp", addr)
	if listen_err != nil {
		log.Fatalf("failed to listen for grpc: %v", listen_err)
	}
	defer listener.Close()

	var grpc_server_opts []grpc.ServerOption
	raft_server := grpc.NewServer(grpc_server_opts...)
	pb.RegisterRaftServiceServer(raft_server, NewRaftServiceServer(addr))

	serve_err := raft_server.Serve(listener)
	if serve_err != nil {
		log.Fatalf("failed to serve: %v", serve_err)
	}

}
