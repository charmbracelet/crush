package jq

import (
	"context"
	"testing"

	"github.com/itchyny/gojq"
	"github.com/stretchr/testify/require"
)

func TestRunCancelsEvaluation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	query, err := gojq.Parse(`cancel | 1`)
	require.NoError(t, err)
	code, err := gojq.Compile(query, gojq.WithFunction("cancel", 0, 0, func(any, []any) any {
		cancel()
		return nil
	}))
	require.NoError(t, err)
	program := &Program{code: code}
	_, err = program.Run(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
}
