package mango

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ManPage represents a man page generator.
type ManPage struct {
	Root            Command
	sections        []Section
	section         uint
	description     string
	longDescription string
}

// Command represents a command.
type Command struct {
	Name     string
	Short    string
	Usage    string
	Flags    map[string]Flag
	Commands map[string]*Command
}

// Flag represents a flag.
type Flag struct {
	Name  string
	Short string
	Usage string
	PFlag bool
}

// Section represents a section.
type Section struct {
	Name string
	Text string
}

// NewManPage returns a new ManPage generator instance.
func NewManPage(section uint, title string, description string) *ManPage {
	root := NewCommand(title, "", "")
	return &ManPage{
		Root:        *root,
		section:     section,
		description: description,
	}
}

// NewCommand returns a new Command.
func NewCommand(name string, short string, usage string) *Command {
	return &Command{
		Name:     name,
		Short:    short,
		Usage:    usage,
		Flags:    make(map[string]Flag),
		Commands: make(map[string]*Command),
	}
}

// WithLongDescription sets the long description.
func (m *ManPage) WithLongDescription(desc string) *ManPage {
	m.longDescription = desc
	return m
}

// WithSection adds a section to the man page.
func (m *ManPage) WithSection(section string, text string) *ManPage {
	m.sections = append(m.sections, Section{
		Name: section,
		Text: text,
	})
	return m
}

// AddFlag adds a flag to the command.
func (m *Command) AddFlag(f Flag) error {
	if _, found := m.Flags[f.Name]; found {
		return errors.New("duplicate flag: " + f.Name)
	}

	m.Flags[f.Name] = f
	return nil
}

// AddCommand adds a sub-command.
func (m *Command) AddCommand(c *Command) error {
	if _, found := m.Commands[c.Name]; found {
		return errors.New("duplicate command: " + c.Name)
	}

	m.Commands[c.Name] = c
	return nil
}

func (m ManPage) buildCommand(w Builder, c Command) {
	if len(c.Flags) > 0 {
		if c.Name == m.Root.Name {
			w.Section("Options")
			w.TaggedParagraph(-1)
		} else {
			w.TaggedParagraph(-1)
			w.TextBold("OPTIONS")
			w.Indent(4)
		}
		keys := make([]string, 0, len(c.Flags))
		for k := range c.Flags {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, k := range keys {
			opt := c.Flags[k]
			if i > 0 {
				w.TaggedParagraph(-1)
			}

			prefix := "-"
			if opt.PFlag {
				prefix = "--"
			}

			if opt.Short != "" {
				w.TextBold(fmt.Sprintf("%[1]s%[2]s %[1]s%[3]s", prefix, opt.Short, opt.Name))
			} else {
				w.TextBold(prefix + opt.Name)
			}
			w.EndSection()

			w.Text(strings.ReplaceAll(opt.Usage, "\n", " "))
		}

		if c.Name == m.Root.Name {
		} else {
			w.IndentEnd()
		}
	}

	if len(c.Commands) > 0 {
		if c.Name == m.Root.Name {
			w.Section("Commands")
			w.TaggedParagraph(-1)
		} else {
			w.TaggedParagraph(-1)
			w.TextBold("COMMANDS")
			w.Indent(4)
		}
		keys := make([]string, 0, len(c.Commands))
		for k := range c.Commands {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, k := range keys {
			opt := c.Commands[k]
			if i > 0 {
				w.TaggedParagraph(-1)
			}

			w.TextBold(opt.Name)
			if opt.Usage != "" {
				w.Text(strings.TrimPrefix(opt.Usage, opt.Name))
			}
			w.Indent(4)
			w.Text(strings.ReplaceAll(opt.Short, "\n", " "))
			w.IndentEnd()

			m.buildCommand(w, *opt)
		}

		if c.Name == m.Root.Name {
		} else {
			w.IndentEnd()
		}
	}
}

// Build generates the man page.
func (m ManPage) Build(w Builder) string {
	/*
	   Common order:
	   - name
	   - synopsis
	   - description
	   - options
	   - exit status
	   - usage
	   - notes
	   - authors
	   - copyright
	   - see also
	*/

	w.Heading(m.section, m.Root.Name, m.description, time.Now())

	w.Section("Name")
	w.Text(m.Root.Name + " - " + m.description)

	w.Section("Synopsis")
	w.TextBold(m.Root.Name)
	w.Text(" [")
	w.TextItalic("options...")
	w.Text("] [")
	w.TextItalic("argument...")
	w.Text("]")

	w.Section("Description")
	w.Text(m.longDescription)

	m.buildCommand(w, m.Root)

	for _, v := range m.sections {
		w.Section(v.Name)
		w.Text(v.Text)
	}

	return w.String()
}
