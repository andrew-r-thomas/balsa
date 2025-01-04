package main

import (
	"log"
	"os"
	"strconv"
)

func main() {
	// get address info for self and siblings from command line args
	args := os.Args[1:]
	servePort := args[0]
	nodes, err := strconv.Atoi(args[1])
	if err != nil {
		log.Fatalf("invalid command args: %v\n", err)
	}
	siblingPorts := args[2 : 2+nodes-1]
	httpPort := args[2+nodes-1]

	balsa := NewBalsa(servePort, siblingPorts, httpPort)
	balsa.Start()

	log.Printf("%s shut down\n", servePort)
}
