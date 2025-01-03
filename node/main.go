package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"

	pb "github.com/andrew-r-thomas/balsa/node/grpc"
)

func main() {
	listener, err := net.Listen("tcp", "localhost:50051")
	if err != nil {
		log.Fatalf("error starting lisener: %v\n", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterRaftServer(grpcServer, &raftServer{})
	grpcServer.Serve(listener)
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
