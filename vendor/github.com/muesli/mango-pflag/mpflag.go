package mpflag

import (
	"github.com/muesli/mango"
	"github.com/spf13/pflag"
)

// PFlagVisitor is used to visit all flags and track them in a mango.ManPage.
func PFlagVisitor(m *mango.ManPage) func(*pflag.Flag) {
	return func(f *pflag.Flag) {
		_ = m.Root.AddFlag(mango.Flag{
			Name:  f.Name,
			Short: f.Shorthand,
			Usage: f.Usage,
			PFlag: true,
		})
	}
}

// PFlagCommandVisitor is used to visit all flags and track them in a mango.Command.
func PFlagCommandVisitor(c *mango.Command) func(*pflag.Flag) {
	return func(f *pflag.Flag) {
		_ = c.AddFlag(mango.Flag{
			Name:  f.Name,
			Short: f.Shorthand,
			Usage: f.Usage,
			PFlag: true,
		})
	}
}
