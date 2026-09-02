package xchroma

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/x/ansi"
)

func TestZodCase(t *testing.T) {
	src := "// Schema for user application preferences, validated on settings save\nconst UserPreferences = z.object({\n  theme: z.enum([\"light\"]).default(\"system\"),\n});\n"
	lexer := chroma.Coalesce(MatchLexer("file.ts"))
	it, err := lexer.Tokenise(nil, src)
	if err != nil {
		t.Fatal(err)
	}
	for tok := it(); tok != chroma.EOF; tok = it() {
		fmt.Printf("  %-30s %q\n", tok.Type, tok.Value)
	}

	it2, _ := lexer.Tokenise(nil, src)
	var buf bytes.Buffer
	if err := Formatter(color.Black, nil).Format(&buf, styles.Get("charm"), it2); err != nil {
		t.Fatal(err)
	}
	fmt.Println("--- stripped output ---")
	for i, line := range strings.Split(ansi.Strip(buf.String()), "\n") {
		fmt.Printf("%d|%s|\n", i, strings.ReplaceAll(line, " ", "·"))
	}
}
