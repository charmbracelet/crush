package taskgraph

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	t.Run("accepts acyclic graph", func(t *testing.T) {
		err := Validate(TaskGraph{
			Nodes: []TaskNode{
				{ID: "fetch"},
				{ID: "analyze", Dependencies: []string{"fetch"}},
				{ID: "report", Dependencies: []string{"analyze"}},
			},
		})
		require.NoError(t, err)
	})

	t.Run("rejects missing dependency", func(t *testing.T) {
		err := Validate(TaskGraph{
			Nodes: []TaskNode{
				{ID: "report", Dependencies: []string{"analyze"}},
			},
		})
		require.EqualError(t, err, "task \"report\" depends on missing task \"analyze\"")
	})

	t.Run("rejects dependency cycle", func(t *testing.T) {
		err := Validate(TaskGraph{
			Nodes: []TaskNode{
				{ID: "a", Dependencies: []string{"b"}},
				{ID: "b", Dependencies: []string{"c"}},
				{ID: "c", Dependencies: []string{"a"}},
			},
		})
		require.EqualError(t, err, "cycle detected at task \"a\"")
	})
}

func TestTopologicalLayers(t *testing.T) {
	t.Run("builds dependency layers", func(t *testing.T) {
		layers, err := TopologicalLayers(TaskGraph{
			Nodes: []TaskNode{
				{ID: "fetch-metadata"},
				{ID: "fetch-source"},
				{ID: "analyze", Dependencies: []string{"fetch-source", "fetch-metadata"}},
				{ID: "summarize", Dependencies: []string{"analyze"}},
				{ID: "lint", Dependencies: []string{"fetch-source"}},
			},
		})
		require.NoError(t, err)
		require.Len(t, layers, 3)
		require.Equal(t, []TaskNode{{ID: "fetch-metadata"}, {ID: "fetch-source"}}, layers[0])
		require.Equal(t, []TaskNode{{ID: "analyze", Dependencies: []string{"fetch-source", "fetch-metadata"}}, {ID: "lint", Dependencies: []string{"fetch-source"}}}, layers[1])
		require.Equal(t, []TaskNode{{ID: "summarize", Dependencies: []string{"analyze"}}}, layers[2])
	})

	t.Run("fails for invalid graph", func(t *testing.T) {
		_, err := TopologicalLayers(TaskGraph{
			Nodes: []TaskNode{
				{ID: "a", Dependencies: []string{"missing"}},
			},
		})
		require.EqualError(t, err, "task \"a\" depends on missing task \"missing\"")
	})
}
