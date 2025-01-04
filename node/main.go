package main

import (
	"log"
	"os"
	"strconv"
	"sync"
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
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		balsa.ServeHttp()
	}()
	go func() {
		defer wg.Done()
		balsa.ServeGrpc()
	}()

	wg.Wait()

	log.Printf("%s shut down\n", servePort)
}
