//go:build windows

package shell

import "testing"

// TestCommandsBlocker_WindowsNormalization covers the two spellings that
// only exist on Windows: backslash separators and the executable
// extension that PATH resolution appends. Both name the same binary as a
// bare `curl`, so the block list has to see through them.
func TestCommandsBlocker_WindowsNormalization(t *testing.T) {
	tests := []struct {
		name        string
		banned      []string
		input       []string
		shouldBlock bool
	}{
		{
			name:        "backslash path",
			banned:      []string{"curl"},
			input:       []string{`C:\Windows\System32\curl`, "https://example.com"},
			shouldBlock: true,
		},
		{
			name:        "exe extension",
			banned:      []string{"curl"},
			input:       []string{"curl.exe", "https://example.com"},
			shouldBlock: true,
		},
		{
			name:        "backslash path with exe extension",
			banned:      []string{"curl"},
			input:       []string{`C:\Windows\System32\curl.exe`, "https://example.com"},
			shouldBlock: true,
		},
		{
			name:        "bat extension",
			banned:      []string{"scp"},
			input:       []string{"scp.bat", "file", "host:/tmp"},
			shouldBlock: true,
		},
		{
			name:        "unrelated extension is not stripped",
			banned:      []string{"curl"},
			input:       []string{"curl.txt"},
			shouldBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CommandsBlocker(tt.banned)(tt.input); got != tt.shouldBlock {
				t.Errorf("block=%v, want %v for input %v", got, tt.shouldBlock, tt.input)
			}
		})
	}
}
