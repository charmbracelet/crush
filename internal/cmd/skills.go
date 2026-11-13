package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:     "skills",
	Aliases: []string{"skill"},
	Short:   "Manage skills",
	Long:    "Manage Agent Skills (SKILL.md) for extending Crush capabilities.",
}

var (
	skillsListFlat bool
	skillsListJSON bool
)

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all discovered skills",
	Long:  `List all discovered skills, grouped by source (builtin, project, user).`,
	Example: `  # List all skills grouped by source
  crush skills list

  # List all skills in flat format
  crush skills list --flat

  # Output in JSON format
  crush skills list --json

  # Search skills by name
  crush skills list pdf`,
	Args: cobra.ArbitraryArgs,
	RunE: runSkillsList,
}

func init() {
	skillsListCmd.Flags().BoolVar(&skillsListFlat, "flat", false, "Show skills as a flat list instead of grouped by source")
	skillsListCmd.Flags().BoolVar(&skillsListJSON, "json", false, "Output in JSON format")
	skillsCmd.AddCommand(skillsListCmd)
	rootCmd.AddCommand(skillsCmd)
}

func runSkillsList(cmd *cobra.Command, args []string) error {
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return err
	}

	dataDir, _ := cmd.Flags().GetString("data-dir")
	debug, _ := cmd.Flags().GetBool("debug")

	cfg, err := config.Init(cwd, dataDir, debug)
	if err != nil {
		return err
	}

	allSkills, disabledSkills := discoverAllSkills(cfg)

	// Build disabled set for display.
	disabledSet := make(map[string]bool, len(disabledSkills))
	for _, name := range disabledSkills {
		disabledSet[name] = true
	}

	// Apply search filter.
	term := strings.ToLower(strings.Join(args, " "))
	if term != "" {
		allSkills = filterSkills(allSkills, term)
	}

	if len(allSkills) == 0 {
		if term != "" {
			return fmt.Errorf("no skills found matching %q", term)
		}
		return fmt.Errorf("no skills found")
	}

	projectDirs := config.ProjectSkillsDir(cwd)

	if skillsListJSON {
		return outputSkillsJSON(cmd, allSkills, disabledSet, projectDirs)
	}

	if skillsListFlat || !isatty.IsTerminal(os.Stdout.Fd()) {
		return outputSkillsFlat(cmd, allSkills, disabledSet, projectDirs)
	}

	return outputSkillsTree(cmd, allSkills, disabledSet, projectDirs)
}

// discoverAllSkills runs the skill discovery pipeline and returns the
// deduplicated list plus the disabled skill names.
func discoverAllSkills(cfg *config.ConfigStore) ([]*skills.Skill, []string) {
	discovered := skills.DiscoverBuiltin()

	opts := cfg.Config().Options
	if opts != nil && len(opts.SkillsPaths) > 0 {
		expandedPaths := make([]string, 0, len(opts.SkillsPaths))
		for _, pth := range opts.SkillsPaths {
			expandedPaths = append(expandedPaths, home.Long(pth))
		}
		discovered = append(discovered, skills.Discover(expandedPaths)...)
	}

	allSkills := skills.Deduplicate(discovered)

	var disabled []string
	if opts != nil {
		disabled = opts.DisabledSkills
	}
	return allSkills, disabled
}

func filterSkills(all []*skills.Skill, term string) []*skills.Skill {
	lower := strings.ToLower(term)
	var result []*skills.Skill
	for _, s := range all {
		if strings.Contains(strings.ToLower(s.Name), lower) ||
			strings.Contains(strings.ToLower(s.Description), lower) {
			result = append(result, s)
		}
	}
	return result
}

func outputSkillsTree(cmd *cobra.Command, all []*skills.Skill, disabledSet map[string]bool, projectDirs []string) error {
	groups := skills.GroupBySource(all, projectDirs)

	headerStyle := lipgloss.NewStyle().Foreground(charmtone.Malibu)
	disabledStyle := lipgloss.NewStyle().Foreground(charmtone.Damson)

	t := tree.New()
	for _, g := range groups {
		groupNode := tree.Root(headerStyle.Render(string(g.Source)))
		for _, s := range g.Skills {
			label := s.Name
			if s.Description != "" {
				label += " — " + s.Description
			}
			if disabledSet[s.Name] {
				label += " " + disabledStyle.Render("(disabled)")
			}
			groupNode.Child(label)
		}
		t.Child(groupNode)
	}

	cmd.Println(t)
	return nil
}

func outputSkillsFlat(cmd *cobra.Command, all []*skills.Skill, disabledSet map[string]bool, projectDirs []string) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	for _, s := range all {
		src := skills.ClassifySource(s, projectDirs)
		status := "enabled"
		if disabledSet[s.Name] {
			status = "disabled"
		}
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, strings.ToLower(string(src)), status); err != nil {
			return err
		}
	}
	return w.Flush()
}

type skillJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	Path        string `json:"path"`
}

func outputSkillsJSON(cmd *cobra.Command, all []*skills.Skill, disabledSet map[string]bool, projectDirs []string) error {
	output := make([]skillJSON, 0, len(all))
	for _, s := range all {
		src := skills.ClassifySource(s, projectDirs)
		status := "enabled"
		if disabledSet[s.Name] {
			status = "disabled"
		}
		output = append(output, skillJSON{
			Name:        s.Name,
			Description: s.Description,
			Source:      strings.ToLower(string(src)),
			Status:      status,
			Path:        s.SkillFilePath,
		})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetEscapeHTML(false)
	return enc.Encode(output)
}
