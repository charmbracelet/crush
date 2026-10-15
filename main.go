// Package main is the entry point for the Crush CLI.
package main

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/charmbracelet/crush/internal/cmd"
	_ "github.com/charmbracelet/crush/internal/dns"
	"github.com/charmbracelet/crush/internal/execpolicy"
	"github.com/joho/godotenv"
)

func main() {
	// Chụp env tin cậy (ranh giới provider/model, tệp env, CLI giao việc)
	// TRƯỚC khi nạp `.env` của thư mục hiện tại: tệp đó thuộc repo đang mở
	// và không được nới ranh giới. Rồi nạp `.env` như autoload cũ (không đè
	// biến đã có).
	execpolicy.Snapshot()
	_ = godotenv.Load()

	if os.Getenv("CRUSH_PROFILE") != "" {
		go func() {
			slog.Info("Serving pprof at localhost:6060")
			if httpErr := http.ListenAndServe("localhost:6060", nil); httpErr != nil {
				slog.Error("Failed to pprof listen", "error", httpErr)
			}
		}()
	}

	cmd.Execute()
}
