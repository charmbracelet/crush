package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
)

func ft(name, desc string) fantasy.AgentTool {
	return fantasy.NewAgentTool(name, desc,
		func(ctx context.Context, in struct{}, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse(""), nil
		},
	)
}

func names(tools []fantasy.AgentTool) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.Info().Name
	}
	return out
}

func TestRankTools(t *testing.T) {
	catalog := []fantasy.AgentTool{
		ft("live_bq_query", "Run a BigQuery SQL query"),
		ft("dbt_search", "Search dbt models by name"),
		ft("grep", "Search files with a regex"),
	}

	ranked := rankTools(catalog, "bigquery sql")
	if len(ranked) == 0 || ranked[0].Info().Name != "live_bq_query" {
		t.Fatalf("expected live_bq_query first, got %v", names(ranked))
	}

	// Name-token matches should outrank description-only matches.
	ranked = rankTools(catalog, "search")
	if len(ranked) == 0 || ranked[0].Info().Name != "dbt_search" {
		t.Fatalf("expected dbt_search first for 'search', got %v", names(ranked))
	}

	if got := rankTools(catalog, "nonexistent"); len(got) != 0 {
		t.Fatalf("expected no matches, got %v", names(got))
	}
}

func TestActivateCapsAndSkipsDupes(t *testing.T) {
	var catalog []fantasy.AgentTool
	for _, n := range []string{"t1", "t2", "t3", "t4", "t5", "t6", "t7"} {
		catalog = append(catalog, ft(n, "handles data records"))
	}
	ts := newToolSearch(catalog, nil)

	first := ts.activate("data")
	if len(first) != maxToolSearchResults {
		t.Fatalf("expected %d activated, got %d", maxToolSearchResults, len(first))
	}
	second := ts.activate("data")
	if len(second) != len(catalog)-maxToolSearchResults {
		t.Fatalf("expected remaining %d activated, got %d", len(catalog)-maxToolSearchResults, len(second))
	}
	if third := ts.activate("data"); len(third) != 0 {
		t.Fatalf("expected nothing new to activate, got %d", len(third))
	}
}

func TestEffectiveIsCorePlusActivatedSorted(t *testing.T) {
	ts := newToolSearch([]fantasy.AgentTool{ft("zeta_tool", "d"), ft("alpha_tool", "d")}, nil)
	ts.core = []fantasy.AgentTool{ft("bash", ""), ft("tool_search", "")}

	// Only core before any activation.
	if got := names(ts.effective()); len(got) != 2 {
		t.Fatalf("expected 2 core tools, got %v", got)
	}

	ts.activate("zeta")
	got := names(ts.effective())
	want := []string{"bash", "tool_search", "zeta_tool"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("effective not sorted: got %v want %v", got, want)
		}
	}
}

func TestCarryOverActivationOnlyWhenCatalogUnchanged(t *testing.T) {
	catalog := []fantasy.AgentTool{ft("alpha", "d"), ft("beta", "d")}
	prev := newToolSearch(catalog, nil)
	prev.activate("alpha")

	// Same catalog -> activation carried over.
	same := newToolSearch([]fantasy.AgentTool{ft("alpha", "d"), ft("beta", "d")}, prev)
	if _, ok := same.activated["alpha"]; !ok {
		t.Fatal("expected activation carried over for identical catalog")
	}

	// Different catalog -> activation reset.
	diff := newToolSearch([]fantasy.AgentTool{ft("alpha", "d"), ft("gamma", "d")}, prev)
	if len(diff.activated) != 0 {
		t.Fatalf("expected reset activation for changed catalog, got %d", len(diff.activated))
	}
}

func TestTokenizeDropsTrivialTokens(t *testing.T) {
	got := tokenize("Query BigQuery a b live_bq")
	want := map[string]bool{"query": true, "bigquery": true, "live_bq": true}
	if len(got) != len(want) {
		t.Fatalf("got %v want keys %v", got, want)
	}
	for _, term := range got {
		if !want[term] {
			t.Fatalf("unexpected token %q in %v", term, got)
		}
	}
}
