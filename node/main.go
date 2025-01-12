package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

func main() {
	// set up logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// get address info for self and siblings from command line args
	args := os.Args[1:]
	id := args[0]
	servePort := args[1]
	nodes, err := strconv.Atoi(args[2])
	if err != nil {
		logger.Error(fmt.Sprintf("invalid command args: %v\n", err))
		os.Exit(1)
	}
	siblingPorts := args[3 : 3+nodes-1]
	siblings := make(map[string]string, nodes-1)
	for _, sib := range siblingPorts {
		pair := strings.Split(sib, ":")
		siblings[pair[0]] = pair[1]
	}

	balsa := NewBalsa(id, siblings, logger)
	balsa.Start(servePort)

	logger.Error(fmt.Sprintf("%s shut down\n", id))
}
