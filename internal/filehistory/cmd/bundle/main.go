// Bundle prepares official filesnap artifacts for a standalone Crush build.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/charmbracelet/crush/internal/filehistory/binary"
	"os"
	"runtime"
)

func main() {
	platform := flag.String("platform", "", "Target GOOS/GOARCH, host, or empty for all release targets")
	output := flag.String("output", "internal/filehistory/binary/assets", "Archive output directory")
	flag.Parse()
	if *platform == "host" {
		*platform = runtime.GOOS + "/" + runtime.GOARCH
	}
	if err := binary.PrepareBundle(context.Background(), *output, *platform); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
