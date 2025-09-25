package config

import (
	"io"
	"strings"
	"testing"
)

func TestMerge(t *testing.T) {
	data1 := strings.NewReader(`{"foo": "bar"}`)
	data2 := strings.NewReader(`{"baz": "qux"}`)

	merged, err := Merge([]io.Reader{data1, data2})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := `{"foo":"bar","baz":"qux"}`
	got, err := io.ReadAll(merged)
	if err != nil {
		t.Fatalf("expected no error reading merged data, got %v", err)
	}

	if string(got) != expected {
		t.Errorf("expected %s, got %s", expected, string(got))
	}
}
