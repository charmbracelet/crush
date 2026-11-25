# Tool plugins

A tool plugin gives the agent a new tool it can call, like `bash` or `grep`,
except what it does is up to you. Behind every tool is a shell command, so
anything you can script, the agent can use: your issue tracker, an internal
API, a deploy script, a CLI you use every day.

This page covers the common cases. Every flag is listed in the
[`tool` reference](../config/README.md#tool).

## A first tool

```bash
# ~/.config/crush/plugins/jira.sh
tool add jira_search \
  --description "Search Jira issues with a JQL query" \
  --param jql "The JQL query" --required jql \
  --command 'curl -fsS -G -H "Authorization: Bearer $JIRA_TOKEN" \
    --data-urlencode "jql=$jql" https://jira.example.com/rest/api/2/search'
```

Restart Crush and ask it to find your open bugs. It'll call `jira_search` with
a JQL query, and whatever the command prints is what it gets back.

The pieces:

- `--description` is what the model reads to decide when to use the tool.
  Write it for the model: say what the tool does and when it's useful.
- `--param NAME DESC` declares a parameter. Add as many as you need, and mark
  the ones the model can't skip with `--required`.
- `--command` is the script that runs on every call.

## How the command runs

When the agent calls your tool:

- The command runs in the project folder.
- Each parameter becomes a shell variable with the same name, so `jql` is
  `$jql`. If the model leaves a parameter out, the variable is empty. Values
  are passed as data, never run as code, so you don't have to worry about
  the model sneaking a command in through a parameter.
- The full input is also on stdin as JSON, if you'd rather parse it yourself
  with `jq`.
- Whatever goes to stdout is the result. Very long output gets cut down the
  same way the built-in `bash` tool does it.
- If the command exits with a non-zero code, the agent sees it as an error,
  along with the exit code, stderr, and stdout. That's usually enough for it
  to figure out what went wrong and try again.
- Calls time out after 60 seconds. Change that with `--timeout`.

A few environment variables are set too, which helps when the command is a
longer script: `CRUSH_TOOL_NAME`, `CRUSH_TOOL_INPUT` (the JSON input),
`CRUSH_SESSION_ID`, `CRUSH_PROJECT_DIR`, and `CRUSH_VERSION`. When the tool
comes from a plugin, `CRUSH_PLUGIN_FILE` and `CRUSH_PLUGIN_DIR` point at it,
so you can ship a helper script next to the plugin and call it:

```bash
tool add deploy \
  --description "Deploy the current branch to staging" \
  --command '"$CRUSH_PLUGIN_DIR/scripts/deploy.sh"'
```

Use `--env KEY VALUE` to give the command extra environment variables.

Only the main coding agent gets custom tools. The read-only helper agents
don't, since a tool could do anything.

## Permissions

Custom tools ask before they run, like MCP tools do. The prompt shows the
parameters, values included, and the exact command, so you know what you're
approving.

If a tool only reads things and you don't want to be asked every time, add
`--skip-permission true`. You can also decide from your own `crushrc`:

```bash
permissions allow jira_search   # never ask for this tool
permissions deny deploy         # hide it from the agent entirely
```

Hooks run for custom tools the same as for built-in ones.

## One tool, many actions

Wrapping a whole CLI as ten separate tools gets messy. Actions let one tool
cover a set of related commands. The model picks one with an `action`
parameter, and Crush lists the actions and their parameters in the tool's
description for you.

```bash
tool add GitHub --description "Work with GitHub through the gh CLI" \
  --param repo "OWNER/REPO; defaults to the project's repository"

tool action GitHub pr_view --description "Show a pull request" \
  --param number "Pull request number" --required number \
  --skip-permission true \
  --command 'gh pr view "$number" ${repo:+-R "$repo"}'

tool action GitHub pr_comment --description "Comment on a pull request" \
  --param number "Pull request number" --required number \
  --param body "The comment" --required body \
  --command 'printf "%s" "$body" | gh pr comment "$number" ${repo:+-R "$repo"} -F -'
```

Parameters on the tool (`repo` here) are shared by every action. Parameters
on an action only belong to that action.

Each action has its own permission, which is the nice part: reading a pull
request can skip the prompt while commenting still asks. In the permission
prompt, "Allow for Session" only approves that one action. To pre-approve a
single action from your `crushrc`, use `permissions allow GitHub:pr_view`.

## Making calls readable in the chat

By default a call shows up in the chat as the tool's name plus its raw JSON
input. That's fine for debugging but not great to read. `--title` and
`--summary` let you describe a call in plain words, using Go
[templates](https://pkg.go.dev/text/template):

```bash
tool action GitHub pr_list --description "List pull requests" \
  --param limit "How many to show" \
  --title 'List PRs' \
  --summary 'Latest {{plural .limit 20 "pull request"}}{{with .repo}} in {{.}}{{end}}' \
  --command 'gh pr list --limit "${limit:-20}" ${repo:+-R "$repo"}'
```

Now a call shows as **List PRs** · Latest 5 pull requests in
charmbracelet/crush.

Every parameter is available as `.name`. There are two helpers:

- `plural COUNT DEFAULT WORD` gives "1 pull request" or "5 pull requests",
  and uses `DEFAULT` when the count is empty.
- `truncate N TEXT` cuts text down to one line of at most `N` characters,
  handy for previewing something like a comment body.

You can set these on the tool, or on an action to override the tool's.

## Rules and gotchas

- Tool names can use letters, numbers, `_` and `-`, up to 64 characters.
  They can't reuse a built-in tool's name or start with `mcp_`.
- Parameter names have to be valid lowercase shell variable names, like
  `issue_id`.
- `--param` covers string parameters. If you need numbers, arrays, or nested
  objects, pass a full JSON Schema with `--schema` instead. Its top-level
  properties still become shell variables; anything that isn't a string
  arrives as JSON, ready for `jq`.
- A mistake in a tool declaration stops Crush from starting and names the
  tool, so you find out right away rather than in the middle of a task.
- You can tweak a plugin's tool from your own `crushrc` without copying it:
  `tool add jira_search --timeout 120`. To switch one off, use
  `tool add jira_search --disabled true`.
- If a plugin skips the permission prompt for an action and you'd rather it
  didn't, declare that action again in your `crushrc` with
  `--skip-permission false`.
