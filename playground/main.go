package main

import (
	"fmt"
	"log"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/nom-nom-hub/blush/internal/tui/components/playground"
)

func main() {
	// Create a new playground model
	model := playground.New()
	
	// Set a reasonable size for testing
	model.SetSize(100, 30)
	
	// Create a new program with the playground model
	p := tea.NewProgram(model, tea.WithAltScreen())
	
	// Run the program
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
	
	fmt.Println("Playground exited successfully")
}