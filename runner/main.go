package main

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
)

type nodeInfo struct {
	port string
	sibs []string
}

var nodes = [][]string{
	{
		"35c022aa-129a-4181-a43f-11ee05917262",
		"50051",
		"3",
		"6914fb2e-68c4-4223-9e30-10c9a5da2e08:50052",
		"e46e4d66-595f-4bc3-a503-f7a6068abeec:50053",
	},
	{
		"6914fb2e-68c4-4223-9e30-10c9a5da2e08",
		"50052",
		"3",
		"35c022aa-129a-4181-a43f-11ee05917262:50051",
		"e46e4d66-595f-4bc3-a503-f7a6068abeec:50053",
	},
	{
		"e46e4d66-595f-4bc3-a503-f7a6068abeec",
		"50053",
		"3",

		"35c022aa-129a-4181-a43f-11ee05917262:50051",
		"6914fb2e-68c4-4223-9e30-10c9a5da2e08:50052",
	},
}

func main() {
	var wg sync.WaitGroup
	wg.Add(len(nodes))

	for _, args := range nodes {
		go func() {
			cmd := exec.Command("../node/node", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Start()
			if err != nil {
				fmt.Printf("error starting node: %v\n", err)
			}
			err = cmd.Wait()
			if err != nil {
				fmt.Printf("error running node: %v\n", err)
			}
			wg.Done()
		}()
	}

	wg.Wait()

	fmt.Printf("all nodes closed\n")
}
