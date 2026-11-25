# Plugins

A plugin is a Bash script that adds providers or tools to Crush. You write it
with the same commands you'd use in a `crushrc` (`provider add`, `tool add`,
and so on), drop it in a plugins folder, and Crush picks it up the next time
it starts. You don't need any Go code or a new Crush release.

- [Provider plugins](providers.md): add a model provider, including ones you
  sign in to with OAuth
- [Tool plugins](tools.md): give the agent new tools, backed by any shell
  command

## Where plugins live

Crush looks in two places:

- `~/.config/crush/plugins/` for plugins you want everywhere (or
  `$XDG_CONFIG_HOME/crush/plugins/` if you've set that)
- `.crush/plugins/` inside a project, for plugins that only belong there

Every `*.sh` file that isn't hidden gets run, in alphabetical order. Crush
also looks one folder down, which is where installed repositories end up.

When two places declare the same thing, the more specific one wins: a project
plugin beats a global one, and your own `crushrc` beats both. That means you
can always change or switch off something a plugin added without editing the
plugin.

If a plugin fails, Crush won't start, same as a broken `crushrc`. That's on
purpose: you'd rather find out now than halfway through a session.

## Installing plugins from GitHub

You don't have to copy files around by hand. If someone publishes plugins in
a GitHub repository, install them with:

```bash
crush plugin install example/crush-plugins
```

The rest of the commands do what you'd expect:

```bash
crush plugin install example/crush-plugins@v1.2.3  # a branch, tag, or commit
crush plugin update example/crush-plugins          # update one repository
crush plugin update                                # update everything
crush plugin list                                  # what's installed, and at which commit
crush plugin uninstall example/crush-plugins
```

A few things worth knowing:

- Only the `.sh` files at the root of the repository count as plugins.
  Everything else in the repo is ignored.
- Add `--project` to install into `.crush/plugins` instead. You can commit
  that folder so everyone on the team runs the same version.
- Crush records the exact commit it installed. `crush plugin update` tells you
  which commit it moved from and to, and deletes plugins the repository has
  removed.
- If you edit an installed plugin, `update` leaves it alone so your changes
  don't get lost. Run `crush plugin trust example/crush-plugins` to keep your
  edits, or `crush plugin update --force` to throw them away.
- For a private repository, export `GITHUB_TOKEN` or `GH_TOKEN` first.

## Only install plugins you trust

A plugin is ordinary Bash that runs with your permissions every time Crush
starts. Crush doesn't sign, sandbox, or review plugins. Installing one is a
lot like piping a script from the internet into your shell, so read it first.
`crush plugin install` prints the commit it fetched, so you know exactly what
to look at.
