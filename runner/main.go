package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func main() {
	node1 := exec.Command("../node/node", "50051", "2", "50052", "3000")
	node2 := exec.Command("../node/node", "50052", "2", "50051", "3001")
	node1.Stdout = os.Stdout
	node2.Stdout = os.Stdout
	node1.Stderr = os.Stderr
	node2.Stderr = os.Stderr

	go func() {
		err := node1.Run()
		if err != nil {
			fmt.Printf("error running node 1: %v\n", err)
		}

	}()
	go func() {
		err := node2.Run()
		if err != nil {
			fmt.Printf("error running node 2: %v\n", err)
		}
	}()

	for {
		time.Sleep(1 * time.Second)
		_, err := http.Get("http://localhost:3000/say-hello")
		if err != nil {
			fmt.Printf("error calling say hello on node 1: %v\n", err)
		}
	}
}
