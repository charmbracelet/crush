package shell

import (
	"context"

	"github.com/u-root/u-root/pkg/core"
	"github.com/u-root/u-root/pkg/core/cat"
	"github.com/u-root/u-root/pkg/core/chmod"
	"github.com/u-root/u-root/pkg/core/cp"
	"github.com/u-root/u-root/pkg/core/find"
	"github.com/u-root/u-root/pkg/core/ls"
	"github.com/u-root/u-root/pkg/core/mkdir"
	"github.com/u-root/u-root/pkg/core/mv"
	"github.com/u-root/u-root/pkg/core/rm"
	"github.com/u-root/u-root/pkg/core/touch"
	"github.com/u-root/u-root/pkg/core/xargs"
	"mvdan.cc/sh/v3/interp"
)

var coreUtils = map[string]func() core.Command{
	"cat":   func() core.Command { return cat.New() },
	"chmod": func() core.Command { return chmod.New() },
	"cp":    func() core.Command { return cp.New() },
	"find":  func() core.Command { return find.New() },
	"ls":    func() core.Command { return ls.New() },
	"mkdir": func() core.Command { return mkdir.New() },
	"mv":    func() core.Command { return mv.New() },
	"rm":    func() core.Command { return rm.New() },
	"touch": func() core.Command { return touch.New() },
	"xargs": func() core.Command { return xargs.New() },
}

func (s *Shell) coreUtilsHandler() func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return next(ctx, args)
			}

			program, programArgs := args[0], args[1:]

			newCoreUtil, ok := coreUtils[program]
			if !ok {
				return next(ctx, args)
			}

			c := interp.HandlerCtx(ctx)

			cmd := newCoreUtil()
			cmd.SetIO(c.Stdin, c.Stdout, c.Stderr)
			cmd.SetWorkingDir(c.Dir)
			cmd.SetLookupEnv(func(key string) (string, bool) {
				v := c.Env.Get(key)
				return v.Str, v.Set
			})
			return cmd.RunContext(ctx, programArgs...)
		}
	}
}
