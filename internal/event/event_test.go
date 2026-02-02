package event

// These tests verify that the Error function correctly handles various
// scenarios. These tests will not log anything.

import (
	"testing"
)

func TestError(t *testing.T) {
	t.Run("returns early when client is nil", func(t *testing.T) {
		// This test verifies that when the PostHog client is not initialized
		// the Error function safely returns early without attempting to
		// enqueue any events. This is important during initialization or when
		// metrics are disabled, as we don't want the error reporting mechanism
		// itself to cause panics.
		originalClient := client
		defer func() {
			client = originalClient
		}()

		client = nil
		Error("test error", "key", "value")
	})

	t.Run("handles nil client without panicking", func(t *testing.T) {
		// This test covers various edge cases where the error value might be
		// nil, a string, or an error type.
		originalClient := client
		defer func() {
			client = originalClient
		}()

		client = nil
		Error(nil)
		Error("some error")
		Error(newDefaultTestError("runtime error"), "key", "value")
	})

	t.Run("handles error with properties", func(t *testing.T) {
		// This test verifies that the Error function can handle additional
		// key-value properties that provide context about the error. These
		// properties are typically passed when recovering from panics (i.e.,
		// panic name, function name).
		//
		// Even with these additional properties, the function should handle
		// them gracefully without panicking.
		originalClient := client
		defer func() {
			client = originalClient
		}()

		client = nil
		Error("test error",
			"type", "test",
			"severity", "high",
			"source", "unit-test",
		)
	})
}

// newDefaultTestError creates a test error that mimics runtime panic
// errors. This helps us testing that the Error function can handle various
// error types, including those that might be passed from a panic recovery
// scenario.
func newDefaultTestError(s string) error {
	return testError(s)
}

type testError string

func (e testError) Error() string {
	return string(e)
}
