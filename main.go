/*
	 TODO:
	- make log based grpc stream out to the cli
*/

package main

import (
	"log"
	"os"
	"strconv"
)

type NodeAddr struct {
	grpcAddr string
	httpAddr string
}

// TODO: this might be better as just an array
var addrs = map[int]NodeAddr{
	0: {grpcAddr: "localhost:50051", httpAddr: "localhost:3000"},
	1: {grpcAddr: "localhost:50052", httpAddr: "localhost:3001"},
	2: {grpcAddr: "localhost:50053", httpAddr: "localhost:3002"},
	3: {grpcAddr: "localhost:50054", httpAddr: "localhost:3003"},
	4: {grpcAddr: "localhost:50055", httpAddr: "localhost:3004"},
}

func main() {
	nodeNum, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("invalid node number")
	}
	nodeAddr := addrs[nodeNum]

	var sibAddrs []string
	for i, addr := range addrs {
		if i == nodeNum {
			continue
		} else {
			sibAddrs = append(sibAddrs, addr.grpcAddr)
		}
	}

	node := NewRaftServiceServer(sibAddrs, nodeAddr.grpcAddr)
	node.Start()
}
