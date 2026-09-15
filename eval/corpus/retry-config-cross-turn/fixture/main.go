package main

import (
	"fmt"
	"os"

	"fetcher/internal/config"
	"fetcher/internal/fetch"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}
	f := fetch.New(cfg)
	fmt.Println("endpoint:", f.Endpoint())
}
