package fang

import (
	"cmp"
	"fmt"
	"os"
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

	styles.Codeblock.Base = styles.Codeblock.Base.Width(blockWidth)

	_, _ = fmt.Fprintln(w, styles.Title.Render("usage"))
	_, _ = fmt.Fprintln(w, styles.Codeblock.Base.Render(usage))
	if len(examples) > 0 {
		cw := styles.Codeblock.Base.GetWidth() - styles.Codeblock.Base.GetHorizontalPadding()
		_, _ = fmt.Fprintln(w, styles.Title.Render("examples"))
		for i, example := range examples {
			if lipgloss.Width(example) > cw {
				examples[i] = ansi.Truncate(example, cw, "…")
			}
		}
		_, _ = fmt.Fprintln(w, styles.Codeblock.Base.Render(strings.Join(examples, "\n")))
	}

	cmds, cmdKeys := evalCmds(c, styles)
	flags, flagKeys := evalFlags(c, styles)
	space := calculateSpace(cmdKeys, flagKeys)

	leftPadding := 4
	if len(cmds) > 0 {
		_, _ = fmt.Fprintln(w, styles.Title.Render("commands"))
		for _, k := range cmdKeys {
			_, _ = fmt.Fprintln(w, lipgloss.JoinHorizontal(
				lipgloss.Left,
				lipgloss.NewStyle().PaddingLeft(leftPadding).Render(k),
				strings.Repeat(" ", space-lipgloss.Width(k)),
				cmds[k],
			))
		}
	}

	if len(flags) > 0 {
		_, _ = fmt.Fprintln(w, styles.Title.Render("flags"))
		for _, k := range flagKeys {
			_, _ = fmt.Fprintln(w, lipgloss.JoinHorizontal(
				lipgloss.Left,
				lipgloss.NewStyle().PaddingLeft(leftPadding).Render(k),
				strings.Repeat(" ", space-lipgloss.Width(k)),
				flags[k],
			))
		}
	}

	_, _ = fmt.Fprintln(w)
}

func writeError(w *colorprofile.Writer, styles Styles, err error) {
	_, _ = fmt.Fprintln(w, styles.ErrorHeader.String())
	_, _ = fmt.Fprintln(w, styles.ErrorText.Render(err.Error()+"."))
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, lipgloss.JoinHorizontal(
		lipgloss.Left,
		styles.ErrorText.UnsetWidth().Render("Try"),
		styles.Program.Flag.Render("--help"),
		styles.ErrorText.UnsetWidth().UnsetMargins().UnsetTransform().PaddingLeft(1).Render("for usage."),
	))
	_, _ = fmt.Fprintln(w)
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
	// XXX: maybe use c.UseLine() here?
	u := c.Use
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

	useLine := []string{
		styles.Name.Render(u),
	}
	if !complete {
		useLine[0] = styles.Command.Render(u)
	}
	if hasCommands {
		useLine = append(
			useLine,
			styles.DimmedArgument.Render("[command]"),
		)
	}
	if hasArgs {
		useLine = append(
			useLine,
			styles.DimmedArgument.Render("[args]"),
		)
	}
	for _, arg := range otherArgs {
		useLine = append(
			useLine,
			styles.DimmedArgument.Render(arg),
		)
	}
	if hasFlags {
		useLine = append(
			useLine,
			styles.DimmedArgument.Render("[--flags]"),
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
	for i, line := range examples {
		line = strings.TrimSpace(line)
		if (i == 0 || i == len(examples)-1) && line == "" {
			continue
		}
		s := styleExample(c, line, styles.Codeblock)
		usage = append(usage, s)
	}

	return usage
}

func styleExample(c *cobra.Command, line string, styles Codeblock) string {
	if strings.HasPrefix(line, "# ") {
		return lipgloss.JoinHorizontal(
			lipgloss.Left,
			styles.Comment.Render(line),
		)
	}

	args := strings.Fields(line)
	var nextIsFlag bool
	var isQuotedString bool
	for i, arg := range args {
		if i == 0 {
			args[i] = styles.Program.Name.Render(arg)
			continue
		}

		quoteStart := arg[0] == '"'
		quoteEnd := arg[len(arg)-1] == '"'
		flagStart := arg[0] == '-'
		if i == 1 && !quoteStart && !flagStart {
			args[i] = styles.Program.Command.Render(arg)
			continue
		}
		if quoteStart {
			isQuotedString = true
		}
		if isQuotedString {
			args[i] = styles.Program.QuotedString.Render(arg)
			if quoteEnd {
				isQuotedString = false
			}
			continue
		}
		if nextIsFlag {
			args[i] = styles.Program.Flag.Render(arg)
			continue
		}
		var dashes string
		if strings.HasPrefix(arg, "-") {
			dashes = "-"
		}
		if strings.HasPrefix(arg, "--") {
			dashes = "--"
		}
		// handle a flag
		if dashes != "" {
			name, value, ok := strings.Cut(arg, "=")
			name = strings.TrimPrefix(name, dashes)
			// it is --flag=value
			if ok {
				args[i] = lipgloss.JoinHorizontal(
					lipgloss.Left,
					styles.Program.Flag.Render(dashes+name+"="),
					styles.Program.Argument.UnsetPadding().Render(value),
				)
				continue
			}
			// it is either --bool-flag or --flag value
			args[i] = lipgloss.JoinHorizontal(
				lipgloss.Left,
				styles.Program.Flag.Render(dashes+name),
			)
			// if the flag is not a bool flag, next arg continues current flag
			nextIsFlag = !isFlagBool(c, name)
			continue
		}
		args[i] = styles.Program.Argument.Render(arg)
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
				styles.Program.Flag.Render("-"+f.Shorthand),
				styles.Program.Flag.Render("--"+f.Name),
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

func evalCmds(c *cobra.Command, styles Styles) (map[string]string, []string) {
	padStyle := lipgloss.NewStyle().PaddingLeft(0) //nolint:mnd
	keys := []string{}
	cmds := map[string]string{}
	for _, sc := range c.Commands() {
		if sc.Hidden {
			continue
		}
		key := padStyle.Render(styleUsage(sc, styles.Program, false))
		help := styles.FlagDescription.Render(sc.Short)
		cmds[key] = help
		keys = append(keys, key)
	}
	return cmds, keys
}

func calculateSpace(k1, k2 []string) int {
	const spaceBetween = 2
	space := minSpace
	for _, k := range append(k1, k2...) {
		space = max(space, lipgloss.Width(k)+spaceBetween)
	}
	return space
}

func isFlagBool(c *cobra.Command, name string) bool {
	flag := c.Flags().Lookup(name)
	if flag == nil && len(name) == 1 {
		flag = c.Flags().ShorthandLookup(name)
	}
	if flag == nil {
		return false
	}
	return flag.Value.Type() == "bool"
}
