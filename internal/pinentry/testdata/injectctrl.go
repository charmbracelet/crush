// Command injectctrl exercises the pinentry terminal handover helpers end
// to end: it configures its terminal via pinentry.SetTerminalRawNoEcho,
// injects Ctrl-L via pinentry.InjectCtrlL, then reads the byte back as if
// it were a pinentry dialog receiving it. A lone byte arriving without a
// newline proves the terminal is in non-canonical mode.
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/crush/internal/pinentry"
)

func main() {
	if err := pinentry.SetTerminalRawNoEcho(); err != nil {
		fmt.Println("MODE-ERROR:", err)
		os.Exit(1)
	}

	if err := pinentry.InjectCtrlL(); err != nil {
		fmt.Println("INJECT-ERROR:", err)
		os.Exit(1)
	}

	var b [1]byte
	if _, err := os.Stdin.Read(b[:]); err != nil {
		fmt.Println("READ-ERROR:", err)
		os.Exit(1)
	}
	fmt.Printf("GOT-%02x\n", b[0])
}
