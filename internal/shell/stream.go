package shell

import (
	"context"
	"os"
)

// progressWriter captures output and calls onProgress with each write.
// stdout and stderr share one, so onProgress may be called concurrently
// and its order can differ from the captured output.
type progressWriter struct {
	buf        *cappedBuffer
	onProgress func(string)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n, err := w.buf.Write(p)
	if n > 0 && w.onProgress != nil {
		w.onProgress(string(p[:n]))
	}
	return n, err
}

// RunAndCaptureStream executes a shell command and streams output chunks
// to onProgress as they arrive. Returns the complete output and exit code.
// onProgress must be safe for concurrent use.
func RunAndCaptureStream(ctx context.Context, opts RunOptions, onProgress func(string)) (CaptureResult, error) {
	if opts.Env == nil {
		opts.Env = os.Environ()
	}
	opts.Env = append(opts.Env, ptyColorEnvVars...)

	buf := &progressWriter{buf: newCappedBuffer(maxKeptOutput), onProgress: onProgress}
	opts.Stdout = buf
	opts.Stderr = buf

	runErr := Run(ctx, opts)

	exitCode := 0
	if runErr != nil {
		exitCode = ExitCode(runErr)
	}

	return CaptureResult{
		Output:   buf.buf.String(),
		ExitCode: exitCode,
	}, nil
}
