package main

import (
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	// get address info for self and siblings from command line args
	args := os.Args[1:]
	id := args[0]
	servePort := args[1]
	nodes, err := strconv.Atoi(args[2])
	if err != nil {
		log.Fatalf("invalid command args: %v\n", err)
	}
	siblingPorts := args[3 : 3+nodes-1]
	siblings := make(map[string]string, nodes-1)
	for _, sib := range siblingPorts {
		pair := strings.Split(sib, ":")
		siblings[pair[0]] = pair[1]
	}

	balsa := NewBalsa(id, servePort, siblings)
	balsa.Start()

	log.Printf("%s shut down\n", servePort)
}
