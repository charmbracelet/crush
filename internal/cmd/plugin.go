package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/plugins"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

// newGitHubClient is indirected so a test can point these commands at a local
// API server instead of api.github.com.
var newGitHubClient = plugins.DefaultGitHub

var pluginCmd = &cobra.Command{
	Use:     "plugin",
	Aliases: []string{"plugins"},
	Short:   "Manage plugins (providers and tools)",
	Long: `Install plugins from GitHub.

A plugin is a Bash script that declares providers and agent tools with the
same builtins a crushrc uses, so either can ship without a Crush release.
Installing a repository writes every *.sh at its root into a plugins directory
and records the exact commit each file came from, which is what an update
compares against.`,
	Example: `
# Install every plugin in a repository, user-wide
crush plugin install example/crush-plugins

# Install from a tag, into this project's .crush/plugins
crush plugin install example/crush-plugins@v1.2.3 --project

# Update one repository, or everything installed
crush plugin update example/crush-plugins
crush plugin update

# See what is installed and which commit it pins
crush plugin list

# Delete an install
crush plugin uninstall example/crush-plugins`,
}

var (
	pluginInstallProject bool
	pluginGlobalScope    bool
	pluginProjectScope   bool
	pluginForce          bool
	pluginListJSON       bool
)

var pluginInstallCmd = &cobra.Command{
	Use:   "install <author>/<repo>[@ref]",
	Short: "Install plugins from a GitHub repository",
	Long: `Install every plugin in a GitHub repository.

The plugins are the non-hidden *.sh files at the repository root; the rest of
the repository is its own business. A ref may be a branch, tag, or commit, and
defaults to the repository's default branch. The resolved commit is recorded
beside the files, so an update can tell whether anything moved.

Installing a repository again refreshes it, and an install under .crush/plugins
can be committed so a team shares one pinned version. A private repository needs
GITHUB_TOKEN or GH_TOKEN exported.`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginInstall,
}

var pluginUpdateCmd = &cobra.Command{
	Use:   "update [<author>/<repo>[@ref]]",
	Short: "Update one or all installed plugins",
	Long: `Update an installed repository to the current commit of the ref it follows, or
update every installed repository when none is named.

Naming a ref with @ref points the install at a different branch or tag from now
on. An install with local edits is left alone: accept them with
crush plugin trust, or overwrite them with --force.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPluginUpdate,
}

var pluginListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List installed plugin repositories",
	Long: `List every plugin repository installed from GitHub, with the ref and commit it
pins, where it lives, and whether its files still match what was installed.
Use --json for machine-readable output.`,
	RunE: runPluginList,
}

var pluginUninstallCmd = &cobra.Command{
	Use:     "uninstall <author>/<repo>",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete an installed plugin repository",
	Long: `Delete an installed repository and the record of its commit. A repository
installed in both the global and the project directory is removed from both,
unless a scope flag narrows it.`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginUninstall,
}

var pluginTrustCmd = &cobra.Command{
	Use:   "trust <author>/<repo>",
	Short: "Accept an install's files as they now stand",
	Long: `Record an installed repository's files as they now stand on disk.

This is for a plugin you edited after installing it: without it,
crush plugin list keeps reporting the install as modified and
crush plugin update refuses to overwrite your changes. A plugin is Bash either
way, and what it does is what it does.`,
	Args: cobra.ExactArgs(1),
	RunE: runPluginTrust,
}

func init() {
	pluginInstallCmd.Flags().BoolVar(&pluginInstallProject, "project", false, "install into .crush/plugins instead of the user-wide directory")
	for _, c := range []*cobra.Command{pluginUpdateCmd, pluginListCmd, pluginUninstallCmd, pluginTrustCmd} {
		c.Flags().BoolVar(&pluginGlobalScope, "global", false, "act on the user-wide plugins directory only")
		c.Flags().BoolVar(&pluginProjectScope, "project", false, "act on .crush/plugins only")
	}
	pluginUpdateCmd.Flags().BoolVar(&pluginForce, "force", false, "overwrite local changes instead of leaving an install alone")
	pluginListCmd.Flags().BoolVar(&pluginListJSON, "json", false, "output in JSON format")

	pluginCmd.AddCommand(
		pluginInstallCmd,
		pluginUpdateCmd,
		pluginListCmd,
		pluginUninstallCmd,
		pluginTrustCmd,
	)
	rootCmd.AddCommand(pluginCmd)
}

func runPluginInstall(cmd *cobra.Command, args []string) error {
	src, ref, err := plugins.ParseSource(args[0])
	if err != nil {
		return err
	}
	root, err := pluginInstallRoot(cmd)
	if err != nil {
		return err
	}

	res, err := plugins.Install(cmd.Context(), plugins.Options{
		Root:   root,
		Source: src,
		Ref:    ref,
		GitHub: newGitHubClient(),
	})
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	switch {
	case res.UpToDate:
		fmt.Fprintf(out, "%s is already up to date at %s\n", src, res.Manifest.ShortCommit())
	case res.Previous != "":
		fmt.Fprintf(out, "Updated %s: %s → %s\n", src, shortSHA(res.Previous), res.Manifest.ShortCommit())
	default:
		fmt.Fprintf(out, "Installed %s at %s\n", src, res.Manifest.ShortCommit())
	}
	if !res.UpToDate {
		printFileChanges(out, res)
		fmt.Fprintf(out, "Where:  %s\n", dimStyle().Render(home.Short(res.Dir)))
		printTrustNote(out, []string{res.Manifest.CommitURL})
	}
	return nil
}

func runPluginUpdate(cmd *cobra.Command, args []string) error {
	roots, err := pluginScope(cmd)
	if err != nil {
		return err
	}

	var (
		targets  []plugins.Installed
		tracked  string
		failures []error
	)
	if len(args) == 1 {
		// A spec may name a new ref to follow, which is the only way an
		// install changes what it tracks.
		src, ref, err := plugins.ParseSource(args[0])
		if err != nil {
			return err
		}
		tracked = ref
		if targets, err = plugins.Find(roots, src); err != nil {
			return err
		}
		if len(targets) == 0 {
			return fmt.Errorf("%s is not installed; run crush plugin install %s first", src, src)
		}
	} else if targets, err = plugins.Discover(roots); err != nil {
		return err
	} else if len(targets) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No plugin repositories are installed.")
		return nil
	}

	gh := newGitHubClient()
	out := cmd.OutOrStdout()
	reviewed := make([]string, 0, len(targets))
	for _, target := range targets {
		if target.Err != nil {
			failures = append(failures, target.Err)
			fmt.Fprintf(out, "%s: %v\n", dimStyle().Render(home.Short(target.Dir)), target.Err)
			continue
		}
		src, _, err := plugins.ParseSource(target.Manifest.Source)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		res, err := plugins.Update(cmd.Context(), plugins.UpdateOptions{
			Root:   target.Root,
			Source: src,
			Ref:    tracked,
			Force:  pluginForce,
			GitHub: gh,
		})
		if err != nil {
			failures = append(failures, err)
			fmt.Fprintf(out, "%s (%s): %v\n", src, target.Scope, err)
			continue
		}
		if res.UpToDate {
			fmt.Fprintf(out, "%s (%s) is up to date at %s\n", src, target.Scope, res.Manifest.ShortCommit())
			continue
		}
		fmt.Fprintf(out, "%s (%s): %s → %s\n", src, target.Scope, shortSHA(res.Previous), res.Manifest.ShortCommit())
		printFileChanges(out, res)
		reviewed = append(reviewed, res.Manifest.CommitURL)
	}

	if len(reviewed) > 0 {
		printTrustNote(out, reviewed)
	}
	if len(failures) > 0 {
		return fmt.Errorf("%d of %d plugin repositories could not be updated", len(failures), len(targets))
	}
	return nil
}

func runPluginList(cmd *cobra.Command, args []string) error {
	roots, err := pluginScope(cmd)
	if err != nil {
		return err
	}
	installs, err := plugins.Discover(roots)
	if err != nil {
		return err
	}

	if pluginListJSON {
		rows := make([]pluginJSON, 0, len(installs))
		for _, install := range installs {
			rows = append(rows, pluginJSON{
				Scope:    string(install.Scope),
				Dir:      install.Dir,
				Manifest: install.Manifest,
				Modified: install.Modified,
				Error:    errorText(install.Err),
			})
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(rows)
	}

	out := cmd.OutOrStdout()
	if len(installs) == 0 {
		fmt.Fprintln(out, "No plugin repositories are installed.")
		fmt.Fprintln(out, "Try: crush plugin install <author>/<repo>")
		return nil
	}

	rows := make([]pluginRow, 0, len(installs))
	styled := term.IsTerminal(os.Stdout.Fd())
	var (
		drifted    int
		unreadable []string
	)
	for _, install := range installs {
		row := pluginRow{cells: [6]string{
			install.Manifest.Source,
			install.Manifest.Ref,
			install.Manifest.ShortCommit(),
			humanTime(install.Manifest.UpdatedAt),
			string(install.Scope),
			"",
		}}
		switch {
		case install.Err != nil:
			row.bad = true
			// A damaged record names no source or commit, so the directory is
			// the only thing a person can act on.
			row.cells = [6]string{filepath.Base(install.Dir), "", "", "", string(install.Scope), "unreadable"}
			unreadable = append(unreadable, fmt.Sprintf("%s: %v", home.Short(install.Dir), install.Err))
		case install.Modified:
			drifted++
			row.cells[5] = fmt.Sprintf("%d (modified)", len(install.Manifest.Files))
		default:
			row.cells[5] = fmt.Sprintf("%d", len(install.Manifest.Files))
		}
		rows = append(rows, row)
	}
	printPluginTable(out, rows, styled)

	for _, problem := range unreadable {
		if styled {
			problem = errorStyle().Render(problem)
		}
		fmt.Fprintln(out, problem)
	}
	if drifted > 0 {
		fmt.Fprintf(out, "\n%d of %d installs do not match the files recorded for them: review them, or accept them with crush plugin trust.\n",
			drifted, len(installs))
	}
	if styled {
		fmt.Fprintln(out, dimStyle().Render("Every plugin above runs as Bash at config load. crush plugin update follows the ref it lists."))
	}
	return nil
}

func runPluginUninstall(cmd *cobra.Command, args []string) error {
	src, _, err := plugins.ParseSource(args[0])
	if err != nil {
		return err
	}
	targets, err := pluginTargets(cmd, src)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	for _, target := range targets {
		m, err := plugins.Remove(target.Root, src)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Removed %s (%s) at %s from %s\n",
			src, target.Scope, m.ShortCommit(), home.Short(target.Root))
	}
	return nil
}

func runPluginTrust(cmd *cobra.Command, args []string) error {
	src, _, err := plugins.ParseSource(args[0])
	if err != nil {
		return err
	}
	targets, err := pluginTargets(cmd, src)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	reviewed := make([]string, 0, len(targets))
	for _, target := range targets {
		m, diff, err := plugins.Trust(target.Root, src)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Trusted %s (%s) as it stands: %d %s at %s\n",
			src, target.Scope, len(m.Files), plural("file", len(m.Files)), home.Short(target.Dir))
		for _, name := range diff.Names() {
			fmt.Fprintf(out, "  %s\n", name)
		}
		reviewed = append(reviewed, m.CommitURL)
	}
	printTrustNote(out, reviewed)
	return nil
}

// pluginInstallRoot is where an install writes. Global is the default because a
// provider is usually wanted everywhere, and --project puts it in the working
// directory's config instead.
func pluginInstallRoot(cmd *cobra.Command) (string, error) {
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return "", err
	}
	if pluginInstallProject {
		return plugins.ProjectRoot(cwd), nil
	}
	return plugins.GlobalRoot(config.GlobalConfig()), nil
}

// pluginScope resolves which plugins directories a command acts on. With
// neither flag it is both, which is what makes an unqualified
// "crush plugin update" cover the machine.
func pluginScope(cmd *cobra.Command) ([]plugins.ScopedRoot, error) {
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return nil, err
	}
	if pluginGlobalScope && pluginProjectScope {
		return nil, errors.New("choose only one of --global and --project")
	}
	return plugins.Roots(plugins.GlobalRoot(config.GlobalConfig()), cwd, pluginGlobalScope, pluginProjectScope), nil
}

// pluginTargets finds every install of one source, which is how uninstall and
// trust reach a repository installed in both scopes.
func pluginTargets(cmd *cobra.Command, src plugins.Source) ([]plugins.Installed, error) {
	roots, err := pluginScope(cmd)
	if err != nil {
		return nil, err
	}
	targets, err := plugins.Find(roots, src)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("%s is not installed", src)
	}
	return targets, nil
}

// printFileChanges says what an install did to the set of plugins, in the order
// a reader scans: what is new, what changed, what is gone.
func printFileChanges(out io.Writer, res plugins.Result) {
	for _, name := range res.Added {
		fmt.Fprintf(out, "  + %s\n", name)
	}
	for _, name := range res.Changed {
		fmt.Fprintf(out, "  * %s\n", name)
	}
	for _, name := range res.Removed {
		fmt.Fprintf(out, "  - %s\n", name)
	}
}

// printTrustNote is the whole trust model for an installed plugin: it is Bash
// that runs with the user's shell privileges at config load, so the note points
// at the commits to read and says nothing else.
func printTrustNote(out io.Writer, commits []string) {
	fmt.Fprintf(out, "\n%s\n", warnStyle().Render(plugins.TrustNote))
	for _, url := range commits {
		if url == "" {
			continue
		}
		fmt.Fprintf(out, "  %s\n", commitStyle().Render(url))
	}
}

func shortSHA(sha string) string {
	if len(sha) <= 8 {
		return sha
	}
	return sha[:8]
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func plural(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

// humanTime keeps a list row readable: a month-old install does not need
// seconds.
func humanTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	since := time.Since(t)
	switch {
	case since < time.Minute:
		return "just now"
	case since < time.Hour:
		return fmt.Sprintf("%dm ago", int(since.Minutes()))
	case since < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(since.Hours()))
	case since < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(since.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// pluginJSON is one row of crush plugin list --json.
type pluginJSON struct {
	Scope    string           `json:"scope"`
	Dir      string           `json:"dir"`
	Manifest plugins.Manifest `json:"manifest"`
	Modified bool             `json:"modified"`
	Error    string           `json:"error,omitempty"`
}

func boldStyle() lipgloss.Style { return lipgloss.NewStyle().Bold(true) }
func dimStyle() lipgloss.Style  { return lipgloss.NewStyle().Foreground(charmtone.Squid) }
func warnStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(charmtone.Butter) }
func commitStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(charmtone.Malibu)
}

func errorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(charmtone.Coral)
}

// pluginColumns are the list table's headers, in order.
var pluginColumns = [6]string{"SOURCE", "REF", "COMMIT", "UPDATED", "SCOPE", "PLUGINS"}

// pluginRow is one list entry held as plain text. Widths are measured before
// any style is applied, because an escape code is invisible to a person but
// not to a naive width count.
type pluginRow struct {
	cells [6]string
	bad   bool
}

// printPluginTable writes the list with the columns padded to their widest
// entry. Without styled, for a pipe or a non-terminal, every cell is plain.
func printPluginTable(out io.Writer, rows []pluginRow, styled bool) {
	widths := make([]int, len(pluginColumns))
	for i, header := range pluginColumns {
		widths[i] = lipgloss.Width(header)
	}
	for _, row := range rows {
		for i, cell := range row.cells {
			widths[i] = max(widths[i], lipgloss.Width(cell))
		}
	}
	write := func(cells [6]string, style func(int) lipgloss.Style) {
		var line strings.Builder
		for i, cell := range cells {
			line.WriteString(style(i).Render(cell))
			if i < len(cells)-1 {
				line.WriteString(strings.Repeat(" ", widths[i]-lipgloss.Width(cell)+2))
			}
		}
		fmt.Fprintln(out, line.String())
	}
	plain := func(int) lipgloss.Style { return lipgloss.NewStyle() }
	header := plain
	if styled {
		header = func(int) lipgloss.Style { return boldStyle() }
	}
	write(pluginColumns, header)
	for _, row := range rows {
		write(row.cells, func(i int) lipgloss.Style {
			if !styled {
				return lipgloss.NewStyle()
			}
			switch {
			case i == 0:
				return boldStyle()
			case i == 2:
				return commitStyle()
			case i == 3:
				return dimStyle()
			case row.bad && i == 5:
				return errorStyle()
			default:
				return lipgloss.NewStyle()
			}
		})
	}
}
