//go:build integration

package tools

import (
	"context"
	"net/http"
	"testing"
)

func TestExaHostedIntegration(t *testing.T) {
	results, err := searchExa(context.Background(), http.DefaultClient, "", "Go programming language", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected Exa results")
	}
	t.Logf("received %d Exa results, output bytes=%d", len(results), len(formatExaSearchResults(results)))
}
