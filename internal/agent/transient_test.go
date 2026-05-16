package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsTransientNetErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"context canceled", context.Canceled, false},
		{"deadline exceeded", context.DeadlineExceeded, false},
		{"unrelated error", errors.New("something else"), false},

		{"ECONNRESET sentinel", syscall.ECONNRESET, true},
		{"EPIPE sentinel", syscall.EPIPE, true},
		{"unexpected EOF", io.ErrUnexpectedEOF, true},

		{
			name: "wrapped ECONNRESET via net.OpError",
			err: &net.OpError{
				Op:  "read",
				Net: "tcp",
				Err: syscall.ECONNRESET,
			},
			want: true,
		},
		{
			name: "fmt-wrapped ECONNRESET",
			err:  fmt.Errorf("stream failed: %w", syscall.ECONNRESET),
			want: true,
		},
		{
			name: "text fallback only",
			err:  errors.New("read tcp 1.2.3.4:443: connection reset by peer"),
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isTransientNetErr(tc.err))
		})
	}
}
