package main

import (
	"os"

	"github.com/donbader/agent-fleet/cmd/agent-fleet/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
