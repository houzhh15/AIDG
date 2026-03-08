package main

import (
	"fmt"
	"os"

	"github.com/houzhh15/aidg-lite/internal/config"
	"github.com/houzhh15/aidg-lite/pkg/server"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	s, err := server.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create server: %v\n", err)
		os.Exit(1)
	}

	s.Start()
}
