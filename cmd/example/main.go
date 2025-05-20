package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/opencode-ai/opencode/internal/message"
	tuiMessage "github.com/opencode-ai/opencode/internal/tui/components/chat/message"
	"github.com/opencode-ai/opencode/internal/tui/components/core/list"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

func randomItems(num int) []util.Model {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	items := make([]util.Model, num)
	for i := 0; i < num; i++ {
		// Generate a random height between 1 and 5 for additional description lines
		randomHeight := rand.Intn(20) + 1

		// Create the item header
		content := []string{fmt.Sprintf("# Item %d", i+1)}

		// Add random number of description lines
		for j := 0; j < randomHeight; j++ {
			content = append(content, fmt.Sprintf("Description **line** %d", j+1))
		}

		joinedContent := strings.Join(content, "\n")

		parts := []message.ContentPart{
			message.TextContent{
				Text: joinedContent,
			},
		}
		if i%2 == 0 {
			parts = append(parts, message.BinaryContent{
				Path: "Test.jpg",
			}, message.BinaryContent{
				Path: "Test2.jpg",
			},
			)
		}
		items[i] = tuiMessage.New(message.Message{
			Role:  message.User,
			Parts: parts,
		})
	}
	return items
}

func main() {
	f, err := tea.LogToFile("debug.log", "debug")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	defer f.Close()
	exampleItems := randomItems(50)
	program := tea.NewProgram(
		list.New(exampleItems, list.WithGapSize(1), list.WithPadding(1)),
		tea.WithAltScreen(),
	)

	program.Run()
}
