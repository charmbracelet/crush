package main

import (
	"net/http"
	"os"

	_ "net/http/pprof" // profiling

	"github.com/charmbracelet/crush/cmd"
	"github.com/charmbracelet/crush/internal/logging"
)

func main() {
	defer logging.RecoverPanic("main", func() {
		logging.ErrorPersist("Application terminated due to unhandled panic")
	})

	if os.Getenv("CRUSH_PROFILE") != "" {
		go func() {
			logging.Info("Serving pprof at localhost:6060")
			if httpErr := http.ListenAndServe("localhost:6060", nil); httpErr != nil {
				logging.Error("Failed to pprof listen: %v", httpErr)
			}
		}()
	}

	cmd.Execute()
}
