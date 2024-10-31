/*
	 TODO:
	- make log based grpc stream out to the cli
*/

package main

import (
	"fmt"
	"net/http"
	"os"
)

type NodeAddr struct {
	grpcAddr string
	httpAddr string
}

func sseLogs(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "you called the logger\n")
}

func main() {
	grpcAddr := os.Args[1]
	httpAddr := os.Args[2]

	var sibAddrs []string
	for _, addr := range os.Args[3:] {
		sibAddrs = append(sibAddrs, addr)
	}

	node := NewRaftServiceServer(sibAddrs, grpcAddr)
	go node.Start()

	http.HandleFunc("/logs", sseLogs)
	http.ListenAndServe(httpAddr, nil)
}
