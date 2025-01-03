package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	"google.golang.org/grpc"

	pb "github.com/andrew-r-thomas/balsa/node/grpc"
)

func main() {
	args := os.Args[1:]
	servePort := args[0]
	nodes, err := strconv.Atoi(args[1])
	siblingPorts := args[2 : 2+nodes-1]

	fmt.Printf("starting node at %s\n", servePort)

	listener, err := net.Listen("tcp", "localhost:"+servePort)
	if err != nil {
		log.Fatalf("error starting lisener: %v\n", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterRaftServer(grpcServer, &raftServer{})
	go func() {
		err := grpcServer.Serve(listener)
		if err != nil {
			log.Fatalf("failed to serve grpc: %v\n", err)
		}
	}()

	siblings := make([]pb.RaftClient, nodes)
	for i, sibPort := range siblingPorts {
		var opts []grpc.DialOption
		conn, err := grpc.NewClient("localhost:"+sibPort, opts...)
		if err != nil {
			log.Fatalf("error creating client: %v\n", err)
		}
		client := pb.NewRaftClient(conn)
		siblings[i] = client
	}

	fmt.Printf("%s is running...\n", servePort)
}

type raftServer struct {
	pb.UnimplementedRaftServer
}

func (*raftServer) Hello(ctx context.Context, request *pb.HelloRequest) (*pb.HelloResponse, error) {
	resp := pb.HelloResponse{
		Response: "Hello " + request.GetName(),
	}
	return &resp, nil
}
