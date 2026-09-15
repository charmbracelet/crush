package main

import (
	"fmt"
	"os"

	"taskapi/internal/client"
	"taskapi/internal/config"
	"taskapi/internal/retry"
	"taskapi/internal/server"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}

	srv := server.New(cfg)
	cli := client.New(cfg)
	policy := retry.DefaultPolicy(cfg)

	fmt.Println("endpoint:", cfg.Endpoint)
	fmt.Println("timeout_ms:", cfg.TimeoutMS)
	fmt.Println("server read timeout:", srv.ReadTimeout())
	fmt.Println("client timeout:", cli.Timeout())
	fmt.Println("retry budget:", policy.Budget)
}
