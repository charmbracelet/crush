# Fang

<p>
    <img width="485" alt="Charm Fang" src="https://github.com/user-attachments/assets/3f34ea01-3750-4760-beb2-a1b700e110f5">
</p>
<p>
    <a href="https://github.com/charmbracelet/fang/releases"><img src="https://img.shields.io/github/release/charmbracelet/fang.svg" alt="Latest Release"></a>
    <a href="https://pkg.go.dev/github.com/charmbracelet/fang?tab=doc"><img src="https://godoc.org/github.com/charmbracelet/fang?status.svg" alt="GoDoc"></a>
    <a href="https://github.com/charmbracelet/fang/actions"><img src="https://github.com/charmbracelet/fang/workflows/build/badge.svg" alt="Build Status"></a>
</p>

The CLI starter kit. A small, experimental library for batteries-included [Cobra][cobra] applications.

<p>
    <img width="859" alt="The Charm Fang mascot and title treatment" src="https://github.com/user-attachments/assets/5c35e1fa-9577-4f81-a879-3ddb4d4a43f0" />
</p>

## Features

- **Fancy output**: fully styled help and usage pages
- **Fancy errors**: fully styled errors
- **Automatic `--version`**: set it to the [build info][info], or a version of your choice
- **Manpages**: Adds a hidden `man` command to generate _manpages_ using
  [mango][][^1]
- **Completions**: Adds a `completion` command to generate shell completions
- **Themeable**: use the built-in theme, or make your own
- **UX**: Silent `usage` output (help is not shown after a user error)

[info]: https://pkg.go.dev/runtime/debug#BuildInfo
[cobra]: https://github.com/spf13/cobra
[mango]: https://github.com/muesli/mango

[^1]:
    Default cobra man pages generates one man page for each command. This is
    generally fine for programs with a lot of sub commands, like git, but its an
    overkill for smaller programs.
    Mango also uses _roff_ directly instead of converting from markdown, so it
    should render better looking man pages.

## Usage

To use it, invoke `fang.Execute` passing your root `*cobra.Command`:

```go
package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:   "example",
		Short: "A simple example program!",
	}
	if err := fang.Execute(context.Background(), cmd); err != nil {
		os.Exit(1)
	}
}
```

That's all there is to it!

## Contributing

See [contributing][contribute].

[contribute]: https://github.com/charmbracelet/fang/contribute

## Feedback

We’d love to hear your thoughts on this project. Feel free to drop us a note!

- [Twitter](https://twitter.com/charmcli)
- [Discord](https://charm.sh/chat)
- [The Fediverse](https://mastodon.social/@charmcli)

## License

[MIT](https://github.com/charmbracelet/gum/raw/main/LICENSE)

---

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" src="https://stuff.charm.sh/charm-badge.jpg" width="400" /></a>

Charm热爱开源 • Charm loves open source
