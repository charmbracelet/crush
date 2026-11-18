// Command fakeproc sleeps for the number of seconds given as the first
// argument. Tests build it under names like "pinentry-curses" or "gpg" to
// emulate processes in the process table.
package main

import (
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		return
	}
	secs, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return
	}
	time.Sleep(time.Duration(secs) * time.Second)
}
