package main

import (
	"fmt"
	"os"

	"toolkit/internal/config"
	"toolkit/internal/fileutil"
)

func main() {
	cfg, err := config.Load("toolkit.conf")
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	fmt.Println("workspace:", cfg.Workspace)
	fmt.Println("copied:", fileutil.DescribeCopy("a.txt", "b.txt"))
}
