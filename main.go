package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"

	_ "net/http/pprof" //nolint:gosec

	_ "github.com/joho/godotenv/autoload" // automatically load .env files

	"github.com/charmbracelet/crush/internal/cmd"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/lipgloss/v2"
)

func main() {
	if runtime.GOOS == "windows" {
		showWindowsWarning()
	}

	defer log.RecoverPanic("main", func() {
		slog.Error("Application terminated due to unhandled panic")
	})

	if os.Getenv("CRUSH_PROFILE") != "" {
		go func() {
			slog.Info("Serving pprof at localhost:6060")
			if httpErr := http.ListenAndServe("localhost:6060", nil); httpErr != nil { //nolint:gosec
				slog.Error(fmt.Sprintf("Failed to pprof listen: %v", httpErr))
			}
		}()
	}

	cmd.Execute()
}

func showWindowsWarning() {
	content := strings.Join([]string{
		lipgloss.NewStyle().Bold(true).Render("WARNING:") + " Crush is experimental on Windows!",
		"While we work on it, we recommend WSL2 for a better experience.",
		lipgloss.NewStyle().Italic(true).Render("Press Enter to continue..."),
	}, "\n")
	fmt.Print(content)

	var input string
	_, _ = fmt.Scanln(&input)
}
