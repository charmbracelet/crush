// Package glamour lets you render markdown documents & templates on ANSI
// compatible terminals. You can create your own stylesheet or simply use one of
// the stylish defaults
package glamour

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"

	"github.com/charmbracelet/glamour/v2/ansi"
	styles "github.com/charmbracelet/glamour/v2/styles"
)

const (
	defaultWidth = 80
	highPriority = 1000
)

// A TermRendererOption sets an option on a TermRenderer.
type TermRendererOption func(*TermRenderer) error

// TermRenderer can be used to render markdown content, posing a depth of
// customization and styles to fit your needs.
type TermRenderer struct {
	md          goldmark.Markdown
	ansiOptions ansi.Options
	buf         bytes.Buffer
	renderBuf   bytes.Buffer
}

// Render initializes a new TermRenderer and renders a markdown with a specific
// style.
func Render(in string, stylePath string) (string, error) {
	b, err := RenderBytes([]byte(in), stylePath)
	return string(b), err
}

// RenderWithEnvironmentConfig initializes a new TermRenderer and renders a
// markdown with a specific style defined by the GLAMOUR_STYLE environment variable.
func RenderWithEnvironmentConfig(in string) (string, error) {
	b, err := RenderBytes([]byte(in), getEnvironmentStyle())
	return string(b), err
}

// RenderBytes initializes a new TermRenderer and renders a markdown with a
// specific style.
func RenderBytes(in []byte, stylePath string) ([]byte, error) {
	r, err := NewTermRenderer(
		WithStylePath(stylePath),
	)
	if err != nil {
		return nil, err
	}
	return r.RenderBytes(in)
}

// NewTermRenderer returns a new TermRenderer the given options.
func NewTermRenderer(options ...TermRendererOption) (*TermRenderer, error) {
	tr := &TermRenderer{
		md: goldmark.New(
			goldmark.WithExtensions(
				extension.GFM,
				extension.DefinitionList,
			),
			goldmark.WithParserOptions(
				parser.WithAutoHeadingID(),
			),
		),
		ansiOptions: ansi.Options{
			WordWrap: defaultWidth,
		},
	}
	for _, o := range options {
		if err := o(tr); err != nil {
			return nil, err
		}
	}
	ar := ansi.NewRenderer(tr.ansiOptions)
	tr.md.SetRenderer(
		renderer.NewRenderer(
			renderer.WithNodeRenderers(
				util.Prioritized(ar, highPriority),
			),
		),
	)
	return tr, nil
}

// WithBaseURL sets a TermRenderer's base URL.
func WithBaseURL(baseURL string) TermRendererOption {
	return func(tr *TermRenderer) error {
		tr.ansiOptions.BaseURL = baseURL
		return nil
	}
}

// WithStandardStyle sets a TermRenderer's styles with a standard (builtin)
// style.
func WithStandardStyle(style string) TermRendererOption {
	return func(tr *TermRenderer) error {
		styles, err := getDefaultStyle(style)
		if err != nil {
			return err
		}
		tr.ansiOptions.Styles = *styles
		return nil
	}
}

// WithEnvironmentConfig sets a TermRenderer's styles based on the
// GLAMOUR_STYLE environment variable.
func WithEnvironmentConfig() TermRendererOption {
	return WithStylePath(getEnvironmentStyle())
}

// WithStylePath sets a TermRenderer's style from stylePath. stylePath is first
// interpreted as a filename. If no such file exists, it is re-interpreted as a
// standard style.
func WithStylePath(stylePath string) TermRendererOption {
	return func(tr *TermRenderer) error {
		styles, err := getDefaultStyle(stylePath)
		if err != nil {
			jsonBytes, err := os.ReadFile(stylePath)
			if err != nil {
				return fmt.Errorf("glamour: error reading file: %w", err)
			}

			return json.Unmarshal(jsonBytes, &tr.ansiOptions.Styles)
		}
		tr.ansiOptions.Styles = *styles
		return nil
	}
}

// WithStyles sets a TermRenderer's styles.
func WithStyles(styles ansi.StyleConfig) TermRendererOption {
	return func(tr *TermRenderer) error {
		tr.ansiOptions.Styles = styles
		return nil
	}
}

// WithStylesFromJSONBytes sets a TermRenderer's styles by parsing styles from
// jsonBytes.
func WithStylesFromJSONBytes(jsonBytes []byte) TermRendererOption {
	return func(tr *TermRenderer) error {
		return json.Unmarshal(jsonBytes, &tr.ansiOptions.Styles)
	}
}

// WithStylesFromJSONFile sets a TermRenderer's styles from a JSON file.
func WithStylesFromJSONFile(filename string) TermRendererOption {
	return func(tr *TermRenderer) error {
		jsonBytes, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("glamour: error reading file: %w", err)
		}
		return json.Unmarshal(jsonBytes, &tr.ansiOptions.Styles)
	}
}

// WithWordWrap sets a TermRenderer's word wrap.
func WithWordWrap(wordWrap int) TermRendererOption {
	return func(tr *TermRenderer) error {
		tr.ansiOptions.WordWrap = wordWrap
		return nil
	}
}

// WithTableWrap controls whether table content will wrap if too long.
// This is true by default. If false, table content will be truncated with an
// ellipsis if too long to fit.
func WithTableWrap(tableWrap bool) TermRendererOption {
	return func(tr *TermRenderer) error {
		tr.ansiOptions.TableWrap = &tableWrap
		return nil
	}
}

// WithInlineTableLinks forces tables to render links inline. By default,links
// are rendered as a list of links at the bottom of the table.
func WithInlineTableLinks(inlineTableLinks bool) TermRendererOption {
	return func(tr *TermRenderer) error {
		tr.ansiOptions.InlineTableLinks = inlineTableLinks
		return nil
	}
}

// WithPreservedNewLines preserves newlines from being replaced.
func WithPreservedNewLines() TermRendererOption {
	return func(tr *TermRenderer) error {
		tr.ansiOptions.PreserveNewLines = true
		return nil
	}
}

// WithEmoji sets a TermRenderer's emoji rendering.
func WithEmoji() TermRendererOption {
	return func(tr *TermRenderer) error {
		emoji.New().Extend(tr.md)
		return nil
	}
}

// WithChromaFormatter sets a TermRenderer's chroma formatter used for code blocks.
func WithChromaFormatter(formatter string) TermRendererOption {
	return func(tr *TermRenderer) error {
		tr.ansiOptions.ChromaFormatter = formatter
		return nil
	}
}

// WithOptions sets multiple TermRenderer options within a single TermRendererOption.
func WithOptions(options ...TermRendererOption) TermRendererOption {
	return func(tr *TermRenderer) error {
		for _, o := range options {
			if err := o(tr); err != nil {
				return err
			}
		}
		return nil
	}
}

func (tr *TermRenderer) Read(b []byte) (int, error) {
	n, err := tr.renderBuf.Read(b)
	if err == io.EOF {
		return n, io.EOF
	}
	if err != nil {
		return 0, fmt.Errorf("glamour: error reading from buffer: %w", err)
	}
	return n, nil
}

func (tr *TermRenderer) Write(b []byte) (int, error) {
	n, err := tr.buf.Write(b)
	if err != nil {
		return 0, fmt.Errorf("glamour: error writing bytes: %w", err)
	}
	return n, nil
}

// Close must be called after writing to TermRenderer. You can then retrieve
// the rendered markdown by calling Read.
func (tr *TermRenderer) Close() error {
	err := tr.md.Convert(tr.buf.Bytes(), &tr.renderBuf)
	if err != nil {
		return fmt.Errorf("glamour: error converting markdown: %w", err)
	}

	tr.buf.Reset()
	return nil
}

// Render returns the markdown rendered into a string.
func (tr *TermRenderer) Render(in string) (string, error) {
	b, err := tr.RenderBytes([]byte(in))
	return string(b), err
}

// RenderBytes returns the markdown rendered into a byte slice.
func (tr *TermRenderer) RenderBytes(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	err := tr.md.Convert(in, &buf)
	return buf.Bytes(), err
}

func getEnvironmentStyle() string {
	glamourStyle := os.Getenv("GLAMOUR_STYLE")
	if len(glamourStyle) == 0 {
		glamourStyle = styles.DarkStyle
	}

	return glamourStyle
}

func getDefaultStyle(style string) (*ansi.StyleConfig, error) {
	styles, ok := styles.DefaultStyles[style]
	if !ok {
		return nil, fmt.Errorf("%s: style not found", style)
	}
	return styles, nil
}
