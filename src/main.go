package main

import (
	"context"
	pb "github.com/andrew-r-thomas/balsa/raft"
)

func main() {

}

type raftServer struct{}

func (*raftServer) Hello(ctx context.Context, request pb.HelloRequest) {}
