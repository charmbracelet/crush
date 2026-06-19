package shell

import (
	"bytes"
	"context"
	"log/slog"
	"os"
)

// progressWriter wraps an io.Writer and calls onProgress with each write.
type progressWriter struct {
	buf        bytes.Buffer
	onProgress func(string)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.buf.Write(p)
	if n > 0 && w.onProgress != nil {
		slog.Debug("Shell stream progress", "bytes", n)
		w.onProgress(string(p[:n]))
	}
	return n, err
}

// RunAndCaptureStream executes a shell command and streams output chunks
// to onProgress as they arrive. Returns the complete output and exit code.
func RunAndCaptureStream(ctx context.Context, opts RunOptions, onProgress func(string)) (CaptureResult, error) {
	if opts.Env == nil {
		opts.Env = os.Environ()
	}
	opts.Env = append(opts.Env, ptyColorEnvVars...)

	stdout := &progressWriter{onProgress: onProgress}
	stderr := &progressWriter{onProgress: onProgress}
	opts.Stdout = stdout
	opts.Stderr = stderr

	runErr := Run(ctx, opts)

	exitCode := 0
	if runErr != nil {
		exitCode = ExitCode(runErr)
	}

	output := stdout.buf.String()
	if stderr.buf.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.buf.String()
	}

	return CaptureResult{
		Output:   output,
		ExitCode: exitCode,
	}, nil
}
