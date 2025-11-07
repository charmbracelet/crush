# Custom Commands in Crush

Crush supports custom commands (sometimes called saved prompts) that can be
created by users to quickly send predefined prompts to the AI assistant.

## Creating Custom Commands

Custom commands are predefined prompts stored as Markdown files in one of three
locations:

1. User Commands:

   ```bash
   # Unix-like systems (Linux, macOS, FreeBSD, etc.)
   $XDG_CONFIG_HOME/crush/commands/

   # Windows
   %USERPROFILE%\AppData\Local\crush\commands\
   ```

2. Project Commands:

   ```bash
   <PROJECT DIR>/.crush/commands/
   ```

Each Markdown file in these directories becomes a custom command. The file name
(without the extension) becomes the name of the command.

For example, creating a file at `~/.config/crush/commands/carrot-cake.md` with
the following content:

```markdown
RUN git ls-files
READ README.md
```

...creates a command called `user:carrot-cake`.

### Command Arguments

Crush supports named arguments in custom commands using placeholders in the
format `$NAME` (where NAME consists of uppercase letters, numbers, and
underscores, and must start with a letter).

For example:

```markdown
# Fetch Context for Issue $ISSUE_NUMBER

RUN gh issue view $ISSUE_NUMBER --json title,body,comments
RUN git grep --author="$AUTHOR_NAME" -n .
RUN grep -R "$SEARCH_PATTERN" $DIRECTORY
```

When you run a command with arguments, crush will prompt you to enter values
for each unique placeholder.

### Organizing Commands

Commands can be organized into sub-directories:

```
~/.config/crush/commands/git/commit.md
```

This creates a command with ID `user:git:commit`.

### Using Custom Commands

1. Press <kbd>Ctrl+p</kbd> to open the command dialog
2. Press <kbd>Tab</kbd> to switch to User Commands
3. Select your custom command

---

Part of [Charm](https://charm.land).

<a href="https://charm.land/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-banner-next.jpg" /></a>

<!--prettier-ignore-->
Charm热爱开源 • Charm loves open source
