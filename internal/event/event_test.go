package event

// These tests verify that the Error function correctly handles various
// scenarios. These tests will not log anything.

import (
	"reflect"
	"testing"

	"github.com/posthog/posthog-go"
)

func TestSetNonInteractive(t *testing.T) {
	originalNonInteractive := baseProps[nonInteractiveAttrName]
	originalNonInteractiveNested := baseProps[nonInteractiveNestedAttrName]
	t.Cleanup(func() {
		baseProps = baseProps.
			Set(nonInteractiveAttrName, originalNonInteractive).
			Set(nonInteractiveNestedAttrName, originalNonInteractiveNested)
	})

	tests := []struct {
		name                     string
		nonInteractive           bool
		crush                    string
		wantNonInteractiveNested bool
	}{
		{
			name: "interactive direct invocation",
		},
		{
			name:           "non-interactive direct invocation",
			nonInteractive: true,
		},
		{
			name:  "interactive nested invocation",
			crush: "1",
		},
		{
			name:                     "non-interactive nested invocation",
			nonInteractive:           true,
			crush:                    "1",
			wantNonInteractiveNested: true,
		},
		{
			name:           "non-interactive invocation with unrecognized marker",
			nonInteractive: true,
			crush:          "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CRUSH", tt.crush)
			SetNonInteractive(tt.nonInteractive)

			if got := baseProps[nonInteractiveAttrName]; got != tt.nonInteractive {
				t.Errorf("%s = %v, want %v", nonInteractiveAttrName, got, tt.nonInteractive)
			}
			if got := baseProps[nonInteractiveNestedAttrName]; got != tt.wantNonInteractiveNested {
				t.Errorf("%s = %v, want %v", nonInteractiveNestedAttrName, got, tt.wantNonInteractiveNested)
			}
		})
	}
}

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
		Error(
			"test error",
			"type", "test",
			"severity", "high",
			"source", "unit-test",
		)
	})
}

func TestPairsToProps(t *testing.T) {
	t.Run("sets valid key value pairs", func(t *testing.T) {
		got := pairsToProps("foo", "bar", "count", 3)
		want := posthog.NewProperties().
			Set("foo", "bar").
			Set("count", 3)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("pairsToProps() = %#v, want %#v", got, want)
		}
	})

	t.Run("returns empty properties for odd pairs", func(t *testing.T) {
		got := pairsToProps("foo", "bar", "count")
		if len(got) != 0 {
			t.Fatalf("pairsToProps() should return empty properties, got %#v", got)
		}
	})

	t.Run("ignores non-string key and continues", func(t *testing.T) {
		got := pairsToProps(123, "bad", "ok", true)
		want := posthog.NewProperties().Set("ok", true)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("pairsToProps() = %#v, want %#v", got, want)
		}
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

func TestSetFirstLaunch(t *testing.T) {
	original := baseProps[firstLaunchAttrName]
	t.Cleanup(func() { baseProps = baseProps.Set(firstLaunchAttrName, original) })

	SetFirstLaunch(true)
	if !IsFirstLaunch() {
		t.Fatal("IsFirstLaunch() = false after SetFirstLaunch(true)")
	}
	SetFirstLaunch(false)
	if IsFirstLaunch() {
		t.Fatal("IsFirstLaunch() = true after SetFirstLaunch(false)")
	}
}

func TestOnboardingProps(t *testing.T) {
	t.Parallel()

	t.Run("leaves out what is not known yet", func(t *testing.T) {
		t.Parallel()
		got := Onboarding{ID: "a1", Via: "card"}.props("method", "sign in")
		want := []any{"onboarding id", "a1", "via", "card", "method", "sign in"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("props() = %#v, want %#v", got, want)
		}
	})

	t.Run("carries everything known", func(t *testing.T) {
		t.Parallel()
		o := Onboarding{ID: "a1", Launch: 2, Provider: "anthropic", Via: "card key", Model: "claude", Auth: "api key"}
		want := []any{
			"onboarding id", "a1",
			"launch", 2,
			"provider", "anthropic",
			"via", "card key",
			"model", "claude",
			"auth", "api key",
		}
		if got := o.props(); !reflect.DeepEqual(got, want) {
			t.Fatalf("props() = %#v, want %#v", got, want)
		}
	})
}
