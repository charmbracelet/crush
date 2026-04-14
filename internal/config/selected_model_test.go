package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelectedModelEqual(t *testing.T) {
	t.Parallel()

	t.Run("equal models", func(t *testing.T) {
		t.Parallel()
		a := SelectedModel{Model: "gpt-4o", Provider: "openai"}
		b := SelectedModel{Model: "gpt-4o", Provider: "openai"}
		require.True(t, a.Equal(b))
	})

	t.Run("different model", func(t *testing.T) {
		t.Parallel()
		a := SelectedModel{Model: "gpt-4o", Provider: "openai"}
		b := SelectedModel{Model: "gpt-4o-mini", Provider: "openai"}
		require.False(t, a.Equal(b))
	})

	t.Run("different provider", func(t *testing.T) {
		t.Parallel()
		a := SelectedModel{Model: "gpt-4o", Provider: "openai"}
		b := SelectedModel{Model: "gpt-4o", Provider: "anthropic"}
		require.False(t, a.Equal(b))
	})

	t.Run("different reasoning effort", func(t *testing.T) {
		t.Parallel()
		a := SelectedModel{Model: "o3", Provider: "openai", ReasoningEffort: "high"}
		b := SelectedModel{Model: "o3", Provider: "openai", ReasoningEffort: "low"}
		require.False(t, a.Equal(b))
	})

	t.Run("different think", func(t *testing.T) {
		t.Parallel()
		a := SelectedModel{Model: "claude-sonnet", Provider: "anthropic", Think: true}
		b := SelectedModel{Model: "claude-sonnet", Provider: "anthropic", Think: false}
		require.False(t, a.Equal(b))
	})

	t.Run("different max tokens", func(t *testing.T) {
		t.Parallel()
		a := SelectedModel{Model: "gpt-4o", Provider: "openai", MaxTokens: 4096}
		b := SelectedModel{Model: "gpt-4o", Provider: "openai", MaxTokens: 8192}
		require.False(t, a.Equal(b))
	})

	t.Run("both nil pointers", func(t *testing.T) {
		t.Parallel()
		a := SelectedModel{Model: "gpt-4o", Provider: "openai"}
		b := SelectedModel{Model: "gpt-4o", Provider: "openai"}
		require.True(t, a.Equal(b))
	})

	t.Run("one nil one non-nil pointer", func(t *testing.T) {
		t.Parallel()
		temp := 0.7
		a := SelectedModel{Model: "gpt-4o", Provider: "openai", Temperature: &temp}
		b := SelectedModel{Model: "gpt-4o", Provider: "openai"}
		require.False(t, a.Equal(b))
		require.False(t, b.Equal(a))
	})

	t.Run("both non-nil equal pointers", func(t *testing.T) {
		t.Parallel()
		temp := 0.7
		a := SelectedModel{Model: "gpt-4o", Provider: "openai", Temperature: &temp}
		b := SelectedModel{Model: "gpt-4o", Provider: "openai", Temperature: &temp}
		require.True(t, a.Equal(b))
	})

	t.Run("both non-nil different pointers", func(t *testing.T) {
		t.Parallel()
		t1 := 0.7
		t2 := 0.9
		a := SelectedModel{Model: "gpt-4o", Provider: "openai", Temperature: &t1}
		b := SelectedModel{Model: "gpt-4o", Provider: "openai", Temperature: &t2}
		require.False(t, a.Equal(b))
	})

	t.Run("nil ProviderOptions", func(t *testing.T) {
		t.Parallel()
		a := SelectedModel{Model: "gpt-4o", Provider: "openai"}
		b := SelectedModel{Model: "gpt-4o", Provider: "openai"}
		require.True(t, a.Equal(b))
	})

	t.Run("empty ProviderOptions", func(t *testing.T) {
		t.Parallel()
		a := SelectedModel{Model: "gpt-4o", Provider: "openai", ProviderOptions: map[string]any{}}
		b := SelectedModel{Model: "gpt-4o", Provider: "openai", ProviderOptions: map[string]any{}}
		require.True(t, a.Equal(b))
	})

	t.Run("different ProviderOptions", func(t *testing.T) {
		t.Parallel()
		a := SelectedModel{Model: "gpt-4o", Provider: "openai", ProviderOptions: map[string]any{"key": "a"}}
		b := SelectedModel{Model: "gpt-4o", Provider: "openai", ProviderOptions: map[string]any{"key": "b"}}
		require.False(t, a.Equal(b))
	})

	t.Run("one nil one non-nil ProviderOptions", func(t *testing.T) {
		t.Parallel()
		a := SelectedModel{Model: "gpt-4o", Provider: "openai", ProviderOptions: map[string]any{"key": "a"}}
		b := SelectedModel{Model: "gpt-4o", Provider: "openai"}
		require.False(t, a.Equal(b))
	})
}

func TestPtrEqual(t *testing.T) {
	t.Parallel()

	t.Run("both nil", func(t *testing.T) {
		t.Parallel()
		require.True(t, ptrEqual[int](nil, nil))
	})

	t.Run("one nil", func(t *testing.T) {
		t.Parallel()
		v := 42
		require.False(t, ptrEqual(&v, nil))
		require.False(t, ptrEqual(nil, &v))
	})

	t.Run("both equal", func(t *testing.T) {
		t.Parallel()
		v := 42
		require.True(t, ptrEqual(&v, &v))
	})

	t.Run("both different", func(t *testing.T) {
		t.Parallel()
		a := 42
		b := 43
		require.False(t, ptrEqual(&a, &b))
	})
}
