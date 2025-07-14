package fang

import (
	"cmp"
	"fmt"
	"io"
	"iter"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	minSpace = 10
	shortPad = 2
	longPad  = 4
)

var width = sync.OnceValue(func() int {
	if s := os.Getenv("__FANG_TEST_WIDTH"); s != "" {
		w, _ := strconv.Atoi(s)
		return w
	}
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		return 120
	}
	return min(w, 120)
})

func helpFn(c *cobra.Command, w *colorprofile.Writer, styles Styles) {
	writeLongShort(w, styles, cmp.Or(c.Long, c.Short))
	usage := styleUsage(c, styles.Codeblock.Program, true)
	examples := styleExamples(c, styles)

	padding := styles.Codeblock.Base.GetHorizontalPadding()
	blockWidth := lipgloss.Width(usage)
	for _, ex := range examples {
		blockWidth = max(blockWidth, lipgloss.Width(ex))
	}
	blockWidth = min(width()-padding, blockWidth+padding)
	blockStyle := styles.Codeblock.Base.Width(blockWidth)

	// if the color profile is ascii or notty, or if the block has no
	// background color set, remove the vertical padding.
	if w.Profile <= colorprofile.Ascii || reflect.DeepEqual(blockStyle.GetBackground(), lipgloss.NoColor{}) {
		blockStyle = blockStyle.PaddingTop(0).PaddingBottom(0)
	}

	_, _ = fmt.Fprintln(w, styles.Title.Render("usage"))
	_, _ = fmt.Fprintln(w, blockStyle.Render(usage))
	if len(examples) > 0 {
		cw := blockStyle.GetWidth() - blockStyle.GetHorizontalPadding()
		_, _ = fmt.Fprintln(w, styles.Title.Render("examples"))
		for i, example := range examples {
			if lipgloss.Width(example) > cw {
				examples[i] = ansi.Truncate(example, cw, "…")
			}
		}
		_, _ = fmt.Fprintln(w, blockStyle.Render(strings.Join(examples, "\n")))
	}

	groups, groupKeys := evalGroups(c)
	cmds, cmdKeys := evalCmds(c, styles)
	flags, flagKeys := evalFlags(c, styles)
	space := calculateSpace(cmdKeys, flagKeys)

	for _, groupID := range groupKeys {
		group := cmds[groupID]
		if len(group) == 0 {
			continue
		}
		renderGroup(w, styles, space, groups[groupID], func(yield func(string, string) bool) {
			for _, k := range cmdKeys {
				cmds, ok := group[k]
				if !ok {
					continue
				}
				if !yield(k, cmds) {
					return
				}
			}
		})
	}

	if len(flags) > 0 {
		renderGroup(w, styles, space, "flags", func(yield func(string, string) bool) {
			for _, k := range flagKeys {
				if !yield(k, flags[k]) {
					return
				}
			}
		})
	}

	_, _ = fmt.Fprintln(w)
}

// DefaultErrorHandler is the default [ErrorHandler] implementation.
func DefaultErrorHandler(w io.Writer, styles Styles, err error) {
	_, _ = fmt.Fprintln(w, styles.ErrorHeader.String())
	_, _ = fmt.Fprintln(w, styles.ErrorText.Render(err.Error()+"."))
	_, _ = fmt.Fprintln(w)
	if isUsageError(err) {
		_, _ = fmt.Fprintln(w, lipgloss.JoinHorizontal(
			lipgloss.Left,
			styles.ErrorText.UnsetWidth().Render("Try"),
			styles.Program.Flag.Render(" --help "),
			styles.ErrorText.UnsetWidth().UnsetMargins().UnsetTransform().Render("for usage."),
		))
		_, _ = fmt.Fprintln(w)
	}
}

// XXX: this is a hack to detect usage errors.
// See: https://github.com/spf13/cobra/pull/2266
func isUsageError(err error) bool {
	s := err.Error()
	for _, prefix := range []string{
		"flag needs an argument:",
		"unknown flag:",
		"unknown shorthand flag:",
		"unknown command",
		"invalid argument",
	} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func writeLongShort(w *colorprofile.Writer, styles Styles, longShort string) {
	if longShort == "" {
		return
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, styles.Text.Width(width()).PaddingLeft(shortPad).Render(longShort))
}

var otherArgsRe = regexp.MustCompile(`(\[.*\])`)

// styleUsage stylized styleUsage line for a given command.
func styleUsage(c *cobra.Command, styles Program, complete bool) string {
	u := c.Use
	if complete {
		u = c.UseLine()
	}
	hasArgs := strings.Contains(u, "[args]")
	hasFlags := strings.Contains(u, "[flags]") || strings.Contains(u, "[--flags]") || c.HasFlags() || c.HasPersistentFlags() || c.HasAvailableFlags()
	hasCommands := strings.Contains(u, "[command]") || c.HasAvailableSubCommands()
	for _, k := range []string{
		"[args]",
		"[flags]", "[--flags]",
		"[command]",
	} {
		u = strings.ReplaceAll(u, k, "")
	}

	var otherArgs []string //nolint:prealloc
	for _, arg := range otherArgsRe.FindAllString(u, -1) {
		u = strings.ReplaceAll(u, arg, "")
		otherArgs = append(otherArgs, arg)
	}

	u = strings.TrimSpace(u)

	useLine := []string{}
	if complete {
		parts := strings.Fields(u)
		useLine = append(useLine, styles.Name.Render(parts[0]))
		if len(parts) > 1 {
			useLine = append(useLine, styles.Command.Render(" "+strings.Join(parts[1:], " ")))
		}
	} else {
		useLine = append(useLine, styles.Command.Render(u))
	}
	if hasCommands {
		useLine = append(
			useLine,
			styles.DimmedArgument.Render(" [command]"),
		)
	}
	if hasArgs {
		useLine = append(
			useLine,
			styles.DimmedArgument.Render(" [args]"),
		)
	}
	for _, arg := range otherArgs {
		useLine = append(
			useLine,
			styles.DimmedArgument.Render(" "+arg),
		)
	}
	if hasFlags {
		useLine = append(
			useLine,
			styles.DimmedArgument.Render(" [--flags]"),
		)
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, useLine...)
}

// styleExamples for a given command.
// will print both the cmd.Use and cmd.Example bits.
func styleExamples(c *cobra.Command, styles Styles) []string {
	if c.Example == "" {
		return nil
	}
	usage := []string{}
	examples := strings.Split(c.Example, "\n")
	var indent bool
	for i, line := range examples {
		line = strings.TrimSpace(line)
		if (i == 0 || i == len(examples)-1) && line == "" {
			continue
		}
		s := styleExample(c, line, indent, styles.Codeblock)
		usage = append(usage, s)
		indent = len(line) > 1 && (line[len(line)-1] == '\\' || line[len(line)-1] == '|')
	}

	return usage
}

func styleExample(c *cobra.Command, line string, indent bool, styles Codeblock) string {
	if strings.HasPrefix(line, "# ") {
		return lipgloss.JoinHorizontal(
			lipgloss.Left,
			styles.Comment.Render(line),
		)
	}

	var isQuotedString bool
	var foundProgramName bool
	var isRedirecting bool
	programName := c.Root().Name()
	args := strings.Fields(line)
	var cleanArgs []string
	for i, arg := range args {
		isQuoteStart := arg[0] == '"' || arg[0] == '\''
		isQuoteEnd := arg[len(arg)-1] == '"' || arg[len(arg)-1] == '\''
		isFlag := arg[0] == '-'

		switch i {
		case 0:
			args[i] = ""
			if indent {
				args[i] = styles.Program.DimmedArgument.Render("  ")
				indent = false
			}
		default:
			args[i] = styles.Program.DimmedArgument.Render(" ")
		}

		if isRedirecting {
			args[i] += styles.Program.DimmedArgument.Render(arg)
			isRedirecting = false
			continue
		}

		switch arg {
		case "\\":
			if i == len(args)-1 {
				args[i] += styles.Program.DimmedArgument.Render(arg)
				continue
			}
		case "|", "||", "-", "&", "&&":
			args[i] += styles.Program.DimmedArgument.Render(arg)
			continue
		}

		if isRedirect(arg) {
			args[i] += styles.Program.DimmedArgument.Render(arg)
			isRedirecting = true
			continue
		}

		if !foundProgramName { //nolint:nestif
			if isQuotedString {
				args[i] += styles.Program.QuotedString.Render(arg)
				isQuotedString = !isQuoteEnd
				continue
			}
			if left, right, ok := strings.Cut(arg, "="); ok {
				args[i] += styles.Program.Flag.Render(left + "=")
				if right[0] == '"' {
					isQuotedString = true
					args[i] += styles.Program.QuotedString.Render(right)
					continue
				}
				args[i] += styles.Program.Argument.Render(right)
				continue
			}

			if arg == programName {
				args[i] += styles.Program.Name.Render(arg)
				foundProgramName = true
				continue
			}
		}

		if !isQuoteStart && !isQuotedString && !isFlag {
			cleanArgs = append(cleanArgs, arg)
		}

		if !isQuoteStart && !isFlag && isSubCommand(c, cleanArgs, arg) {
			args[i] += styles.Program.Command.Render(arg)
			continue
		}
		isQuotedString = isQuotedString || isQuoteStart
		if isQuotedString {
			args[i] += styles.Program.QuotedString.Render(arg)
			isQuotedString = !isQuoteEnd
			continue
		}
		// handle a flag
		if isFlag {
			name, value, ok := strings.Cut(arg, "=")
			// it is --flag=value
			if ok {
				args[i] += lipgloss.JoinHorizontal(
					lipgloss.Left,
					styles.Program.Flag.Render(name+"="),
					styles.Program.Argument.Render(value),
				)
				continue
			}
			// it is either --bool-flag or --flag value
			args[i] += lipgloss.JoinHorizontal(
				lipgloss.Left,
				styles.Program.Flag.Render(name),
			)
			continue
		}

		args[i] += styles.Program.Argument.Render(arg)
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		args...,
	)
}

func evalFlags(c *cobra.Command, styles Styles) (map[string]string, []string) {
	flags := map[string]string{}
	keys := []string{}
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		var parts []string
		if f.Shorthand == "" {
			parts = append(
				parts,
				styles.Program.Flag.Render("--"+f.Name),
			)
		} else {
			parts = append(
				parts,
				styles.Program.Flag.Render("-"+f.Shorthand+" --"+f.Name),
			)
		}
		key := lipgloss.JoinHorizontal(lipgloss.Left, parts...)
		help := styles.FlagDescription.Render(f.Usage)
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" && f.DefValue != "[]" {
			help = lipgloss.JoinHorizontal(
				lipgloss.Left,
				help,
				styles.FlagDefault.Render("("+f.DefValue+")"),
			)
		}
		flags[key] = help
		keys = append(keys, key)
	})
	return flags, keys
}

// result is map[groupID]map[styled cmd name]styled cmd help, and the keys in
// the order they are defined.
func evalCmds(c *cobra.Command, styles Styles) (map[string](map[string]string), []string) {
	padStyle := lipgloss.NewStyle().PaddingLeft(0) //nolint:mnd
	keys := []string{}
	cmds := map[string]map[string]string{}
	for _, sc := range c.Commands() {
		if sc.Hidden {
			continue
		}
		if _, ok := cmds[sc.GroupID]; !ok {
			cmds[sc.GroupID] = map[string]string{}
		}
		key := padStyle.Render(styleUsage(sc, styles.Program, false))
		help := styles.FlagDescription.Render(sc.Short)
		cmds[sc.GroupID][key] = help
		keys = append(keys, key)
	}
	return cmds, keys
}

func evalGroups(c *cobra.Command) (map[string]string, []string) {
	// make sure the default group is the first
	ids := []string{""}
	groups := map[string]string{"": "commands"}
	for _, g := range c.Groups() {
		groups[g.ID] = g.Title
		ids = append(ids, g.ID)
	}
	return groups, ids
}

func renderGroup(w io.Writer, styles Styles, space int, name string, items iter.Seq2[string, string]) {
	_, _ = fmt.Fprintln(w, styles.Title.Render(name))
	for key, help := range items {
		_, _ = fmt.Fprintln(w, lipgloss.JoinHorizontal(
			lipgloss.Left,
			lipgloss.NewStyle().PaddingLeft(longPad).Render(key),
			strings.Repeat(" ", space-lipgloss.Width(key)),
			help,
		))
	}
}

func calculateSpace(k1, k2 []string) int {
	const spaceBetween = 2
	space := minSpace
	for _, k := range append(k1, k2...) {
		space = max(space, lipgloss.Width(k)+spaceBetween)
	}
	return space
}

func isSubCommand(c *cobra.Command, args []string, word string) bool {
	cmd, _, _ := c.Root().Traverse(args)
	return cmd != nil && cmd.Name() == word
}

var redirectPrefixes = []string{">", "<", "&>", "2>", "1>", ">>", "2>>"}

func isRedirect(s string) bool {
	for _, p := range redirectPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
