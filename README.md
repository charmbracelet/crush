# Karigor

<p align="center">
<pre>
    __ __           _
   / //_/___ ______(_)___ _____  _____
  / ,< / __ `/ ___/ / __ `/ __ \/ ___/
 / /| / /_/ / /  / / /_/ / /_/ / /
/_/ |_\\__,_/_/  /_/\\__, /\____/_/
                  /____/
</pre>
<p align="center"><strong>Developer Preview by Faisal Kabir Galib</strong></p>
<p align="center">Your AI-powered coding companion, now available in your terminal.<br />Tools, code, and workflows, seamlessly integrated with your LLM of choice.</p>
<p align="center">您的 AI 编程助手，现在就在您的终端中。<br />您的工具、代码和工作流，与您选择的 LLM 模型完美集成。</p>
</p>

## Features

- **Multi-Model:** choose from a wide range of LLMs or add your own via OpenAI- or Anthropic-compatible APIs
- **Flexible:** switch LLMs mid-session while preserving context
- **Session-Based:** maintain multiple work sessions and contexts per project
- **LSP-Enhanced:** Karigor uses LSPs for additional context, just like you do
- **Extensible:** add capabilities via MCPs (`http`, `stdio`, and `sse`)
- **Works Everywhere:** first-class support in every terminal on macOS, Linux, Windows (PowerShell and WSL), FreeBSD, OpenBSD, and NetBSD

## Installation

**Developer Preview - Build from Source**

This is currently a developer preview. To install Karigor, build it from source:

```bash
# Clone the repository
git clone https://github.com/yourusername/karigor.git
cd karigor

# Build and install
go build .
./karigor --help
```

Or install directly with Go:

```bash
go install github.com/yourusername/karigor@latest
```

**Provide Feedback**

For this developer preview, please report bugs and feedback to: faisalkabirgalib@gmail.com

<details>
<summary><strong>Nix (NUR)</strong></summary>

Crush is available via [NUR](https://github.com/nix-community/NUR) in `nur.repos.charmbracelet.crush`.

You can also try out Crush via `nix-shell`:

```bash
# Add the NUR channel.
nix-channel --add https://github.com/nix-community/NUR/archive/main.tar.gz nur
nix-channel --update

# Get Crush in a Nix shell.
nix-shell -p '(import <nur> { pkgs = import <nixpkgs> {}; }).repos.charmbracelet.crush'
```

### NixOS & Home Manager Module Usage via NUR

Crush provides NixOS and Home Manager modules via NUR.
You can use these modules directly in your flake by importing them from NUR. Since it auto detects whether its a home manager or nixos context you can use the import the exact same way :)

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    nur.url = "github:nix-community/NUR";
  };

  outputs = { self, nixpkgs, nur, ... }: {
    nixosConfigurations.your-hostname = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        nur.modules.nixos.default
        nur.repos.charmbracelet.modules.crush
        {
          programs.crush = {
            enable = true;
            settings = {
              providers = {
                openai = {
                  id = "openai";
                  name = "OpenAI";
                  base_url = "https://api.openai.com/v1";
                  type = "openai";
                  api_key = "sk-fake123456789abcdef...";
                  models = [
                    {
                      id = "gpt-4";
                      name = "GPT-4";
                    }
                  ];
                };
              };
              lsp = {
                go = { command = "gopls"; enabled = true; };
                nix = { command = "nil"; enabled = true; };
              };
              options = {
                context_paths = [ "/etc/nixos/configuration.nix" ];
                tui = { compact_mode = true; };
                debug = false;
              };
            };
          };
        }
      ];
    };
  };
}
```

</details>

<details>
<summary><strong>Debian/Ubuntu</strong></summary>

```bash
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list
sudo apt update && sudo apt install crush
```

</details>

<details>
<summary><strong>Fedora/RHEL</strong></summary>

```bash
echo '[charm]
name=Charm
baseurl=https://repo.charm.sh/yum/
enabled=1
gpgcheck=1
gpgkey=https://repo.charm.sh/yum/gpg.key' | sudo tee /etc/yum.repos.d/charm.repo
sudo yum install crush
```

</details>

Or, download it:

- [Packages][releases] are available in Debian and RPM formats
- [Binaries][releases] are available for Linux, macOS, Windows, FreeBSD, OpenBSD, and NetBSD

[releases]: https://github.com/charmbracelet/crush/releases

Or just install it with Go:

```
go install github.com/charmbracelet/crush@latest
```

> [!NOTE]
> This is a developer preview of Karigor by Faisal Kabir Galib. Please report
> bugs and feedback to faisalkabirgalib@gmail.com. Your input is valuable for
> improving this AI-powered coding assistant.

## Getting Started

The quickest way to get started is to grab an API key for your preferred
provider such as Anthropic, OpenAI, Groq, or OpenRouter and just start
Karigor. You'll be prompted to enter your API key.

That said, you can also set environment variables for preferred providers.

| Environment Variable        | Provider                                           |
| --------------------------- | -------------------------------------------------- |
| `ANTHROPIC_API_KEY`         | Anthropic                                          |
| `OPENAI_API_KEY`            | OpenAI                                             |
| `OPENROUTER_API_KEY`        | OpenRouter                                         |
| `GEMINI_API_KEY`            | Google Gemini                                      |
| `CEREBRAS_API_KEY`          | Cerebras                                           |
| `HF_TOKEN`                  | Huggingface Inference                              |
| `VERTEXAI_PROJECT`          | Google Cloud VertexAI (Gemini)                     |
| `VERTEXAI_LOCATION`         | Google Cloud VertexAI (Gemini)                     |
| `GROQ_API_KEY`              | Groq                                               |
| `AWS_ACCESS_KEY_ID`         | Amazon Bedrock (Claude)                               |
| `AWS_SECRET_ACCESS_KEY`     | Amazon Bedrock (Claude)                               |
| `AWS_REGION`                | Amazon Bedrock (Claude)                               |
| `AWS_PROFILE`               | Amazon Bedrock (Custom Profile)                       |
| `AWS_BEARER_TOKEN_BEDROCK`  | Amazon Bedrock                                        |
| `AZURE_OPENAI_API_ENDPOINT` | Azure OpenAI models                                |
| `AZURE_OPENAI_API_KEY`      | Azure OpenAI models (optional when using Entra ID) |
| `AZURE_OPENAI_API_VERSION`  | Azure OpenAI models                                |

### By the Way

Is there a provider you'd like to see in Karigor? Is there an existing model that needs an update?

Karigor's default model listing is managed in [Catwalk](https://github.com/charmbracelet/catwalk), a community-supported, open source repository of LLM-compatible models, and you're welcome to contribute.

<a href="https://github.com/charmbracelet/catwalk"><img width="174" height="174" alt="Catwalk Badge" src="https://github.com/user-attachments/assets/95b49515-fe82-4409-b10d-5beb0873787d" /></a>

## Configuration

Karigor runs great with no configuration. That said, if you do need or want to
customize Karigor, configuration can be added either local to the project itself,
or globally, with the following priority:

1. `.karigor.json`
2. `karigor.json`
3. `$HOME/.config/karigor/karigor.json`

Configuration itself is stored as a JSON object:

```json
{
  "this-setting": { "this": "that" },
  "that-setting": ["ceci", "cela"]
}
```

As an additional note, Karigor also stores ephemeral data, such as application state, in one additional location:

```bash
# Unix
$HOME/.local/share/karigor/karigor.json

# Windows
%LOCALAPPDATA%\karigor\karigor.json
```

### LSPs

Karigor can use LSPs for additional context to help inform its decisions, just
like you would. LSPs can be added manually like so:

```json
{
  "$schema": "https://charm.land/karigor.json",
  "lsp": {
    "go": {
      "command": "gopls",
      "env": {
        "GOTOOLCHAIN": "go1.24.5"
      }
    },
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"]
    },
    "nix": {
      "command": "nil"
    }
  }
}
```

### MCPs

Karigor also supports Model Context Protocol (MCP) servers through three
transport types: `stdio` for command-line servers, `http` for HTTP endpoints,
and `sse` for Server-Sent Events. Environment variable expansion is supported
using `$(echo $VAR)` syntax.

```json
{
  "$schema": "https://charm.land/karigor.json",
  "mcp": {
    "filesystem": {
      "type": "stdio",
      "command": "node",
      "args": ["/path/to/mcp-server.js"],
      "timeout": 120,
      "disabled": false,
      "disabled_tools": ["some-tool-name"],
      "env": {
        "NODE_ENV": "production"
      }
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "timeout": 120,
      "disabled": false,
      "disabled_tools": ["create_issue", "create_pull_request"],
      "headers": {
        "Authorization": "Bearer $GH_PAT"
      }
    },
    "streaming-service": {
      "type": "sse",
      "url": "https://example.com/mcp/sse",
      "timeout": 120,
      "disabled": false,
      "headers": {
        "API-Key": "$(echo $API_KEY)"
      }
    }
  }
}
```

### Ignoring Files

Karigor respects `.gitignore` files by default, but you can also create a
`.karigorigore` file to specify additional files and directories that Karigor
should ignore. This is useful for excluding files that you want in version
control but don't want Karigor to consider when providing context.

The `.karigorigore` file uses the same syntax as `.gitignore` and can be placed
in the root of your project or in subdirectories.

### Allowing Tools

By default, Karigor will ask you for permission before running tool calls. If
you'd like, you can allow tools to be executed without prompting you for
permissions. Use this with care.

```json
{
  "$schema": "https://charm.land/karigor.json",
  "permissions": {
    "allowed_tools": [
      "view",
      "ls",
      "grep",
      "edit",
      "mcp_context7_get-library-doc"
    ]
  }
}
```

You can also skip all permission prompts entirely by running Karigor with the
`--yolo` flag. Be very, very careful with this feature.

### Disabling Built-In Tools

If you'd like to prevent Karigor from using certain built-in tools entirely, you
can disable them via the `options.disabled_tools` list. Disabled tools are
completely hidden from the agent.

```json
{
  "$schema": "https://charm.land/karigor.json",
  "options": {
    "disabled_tools": [
      "bash",
      "sourcegraph"
    ]
  }
}
```

To disable tools from MCP servers, see the [MCP config section](#mcps).

### Initialization

When you initialize a project, Karigor analyzes your codebase and creates
a context file that helps it work more effectively in future sessions.
By default, this file is named `AGENTS.md`, but you can customize the
name and location with the `initialize_as` option:

```json
{
  "$schema": "https://charm.land/karigor.json",
  "options": {
    "initialize_as": "AGENTS.md"
  }
}
```

This is useful if you prefer a different naming convention or want to
place the file in a specific directory (e.g., `KARIGOR.md` or
`docs/LLMs.md`). Karigor will fill the file with project-specific context
like build commands, code patterns, and conventions it discovered during
initialization.

### Attribution Settings

By default, Karigor adds attribution information to Git commits and pull requests
it creates. You can customize this behavior with the `attribution` option:

```json
{
  "$schema": "https://charm.land/karigor.json",
  "options": {
    "attribution": {
      "trailer_style": "co-authored-by",
      "generated_with": true
    }
  }
}
```

- `trailer_style`: Controls the attribution trailer added to commit messages
  (default: `assisted-by`)
	- `assisted-by`: Adds `Assisted-by: [Model Name] via Karigor <karigor@faisalkabirgalib.com>`
	  (includes the model name)
	- `co-authored-by`: Adds `Co-Authored-By: Karigor <karigor@faisalkabirgalib.com>`
	- `none`: No attribution trailer
- `generated_with`: When true (default), adds `💘 Generated with Karigor` line to
  commit messages and PR descriptions

### Custom Providers

Karigor supports custom provider configurations for both OpenAI-compatible and
Anthropic-compatible APIs.

> [!NOTE]
> Note that we support two "types" for OpenAI. Make sure to choose the right one
> to ensure the best experience!
> * `openai` should be used when proxying or routing requests through OpenAI.
> * `openai-compat` should be used when using non-OpenAI providers that have OpenAI-compatible APIs.

#### OpenAI-Compatible APIs

Here’s an example configuration for Deepseek, which uses an OpenAI-compatible
API. Don't forget to set `DEEPSEEK_API_KEY` in your environment.

```json
{
  "$schema": "https://charm.land/karigor.json",
  "providers": {
    "deepseek": {
      "type": "openai-compat",
      "base_url": "https://api.deepseek.com/v1",
      "api_key": "$DEEPSEEK_API_KEY",
      "models": [
        {
          "id": "deepseek-chat",
          "name": "Deepseek V3",
          "cost_per_1m_in": 0.27,
          "cost_per_1m_out": 1.1,
          "cost_per_1m_in_cached": 0.07,
          "cost_per_1m_out_cached": 1.1,
          "context_window": 64000,
          "default_max_tokens": 5000
        }
      ]
    }
  }
}
```

#### Anthropic-Compatible APIs

Custom Anthropic-compatible providers follow this format:

```json
{
  "$schema": "https://charm.land/karigor.json",
  "providers": {
    "custom-anthropic": {
      "type": "anthropic",
      "base_url": "https://api.anthropic.com/v1",
      "api_key": "$ANTHROPIC_API_KEY",
      "extra_headers": {
        "anthropic-version": "2023-06-01"
      },
      "models": [
        {
          "id": "claude-sonnet-4-20250514",
          "name": "Claude Sonnet 4",
          "cost_per_1m_in": 3,
          "cost_per_1m_out": 15,
          "cost_per_1m_in_cached": 3.75,
          "cost_per_1m_out_cached": 0.3,
          "context_window": 200000,
          "default_max_tokens": 50000,
          "can_reason": true,
          "supports_attachments": true
        }
      ]
    }
  }
}
```

### Amazon Bedrock

Karigor currently supports running Anthropic models through Bedrock, with caching disabled.

- A Bedrock provider will appear once you have AWS configured, i.e. `aws configure`
- Karigor also expects the `AWS_REGION` or `AWS_DEFAULT_REGION` to be set
- To use a specific AWS profile set `AWS_PROFILE` in your environment, i.e. `AWS_PROFILE=myprofile karigor`
- Alternatively to `aws configure`, you can also just set `AWS_BEARER_TOKEN_BEDROCK`

### Vertex AI Platform

Vertex AI will appear in the list of available providers when `VERTEXAI_PROJECT` and `VERTEXAI_LOCATION` are set. You will also need to be authenticated:

```bash
gcloud auth application-default login
```

To add specific models to the configuration, configure as such:

```json
{
  "$schema": "https://charm.land/karigor.json",
  "providers": {
    "vertexai": {
      "models": [
        {
          "id": "claude-sonnet-4@20250514",
          "name": "VertexAI Sonnet 4",
          "cost_per_1m_in": 3,
          "cost_per_1m_out": 15,
          "cost_per_1m_in_cached": 3.75,
          "cost_per_1m_out_cached": 0.3,
          "context_window": 200000,
          "default_max_tokens": 50000,
          "can_reason": true,
          "supports_attachments": true
        }
      ]
    }
  }
}
```

### Local Models

Local models can also be configured via OpenAI-compatible API. Here are two common examples:

#### Ollama

```json
{
  "providers": {
    "ollama": {
      "name": "Ollama",
      "base_url": "http://localhost:11434/v1/",
      "type": "openai-compat",
      "models": [
        {
          "name": "Qwen 3 30B",
          "id": "qwen3:30b",
          "context_window": 256000,
          "default_max_tokens": 20000
        }
      ]
    }
  }
}
```

#### LM Studio

```json
{
  "providers": {
    "lmstudio": {
      "name": "LM Studio",
      "base_url": "http://localhost:1234/v1/",
      "type": "openai-compat",
      "models": [
        {
          "name": "Qwen 3 30B",
          "id": "qwen/qwen3-30b-a3b-2507",
          "context_window": 256000,
          "default_max_tokens": 20000
        }
      ]
    }
  }
}
```

## Logging

Sometimes you need to look at logs. Luckily, Karigor logs all sorts of
stuff. Logs are stored in `./.karigor/logs/karigor.log` relative to the project.

The CLI also contains some helper commands to make perusing recent logs easier:

```bash
# Print the last 1000 lines
karigor logs

# Print the last 500 lines
karigor logs --tail 500

# Follow logs in real time
karigor logs --follow
```

Want more logging? Run `karigor` with the `--debug` flag, or enable it in the
config:

```json
{
  "$schema": "https://charm.land/karigor.json",
  "options": {
    "debug": true,
    "debug_lsp": true
  }
}
```

## Provider Auto-Updates

By default, Karigor automatically checks for the latest and greatest list of
providers and models from [Catwalk](https://github.com/charmbracelet/catwalk),
the open source LLM provider database. This means that when new providers and
models are available, or when model metadata changes, Karigor automatically
updates your local configuration.

### Disabling automatic provider updates

For those with restricted internet access, or those who prefer to work in
air-gapped environments, this might not be want you want, and this feature can
be disabled.

To disable automatic provider updates, set `disable_provider_auto_update` into
your `karigor.json` config:

```json
{
  "$schema": "https://charm.land/karigor.json",
  "options": {
    "disable_provider_auto_update": true
  }
}
```

Or set the `KARIGOR_DISABLE_PROVIDER_AUTO_UPDATE` environment variable:

```bash
export KARIGOR_DISABLE_PROVIDER_AUTO_UPDATE=1
```

### Manually updating providers

Manually updating providers is possible with the `karigor update-providers`
command:

```bash
# Update providers remotely from Catwalk.
karigor update-providers

# Update providers from a custom Catwalk base URL.
karigor update-providers https://example.com/

# Update providers from a local file.
karigor update-providers /path/to/local-providers.json

# Reset providers to the embedded version, embedded at karigor at build time.
karigor update-providers embedded

# For more info:
karigor update-providers --help
```

## Metrics

Karigor records pseudonymous usage metrics (tied to a device-specific hash),
which help inform development and support priorities. The
metrics include solely usage metadata; prompts and responses are NEVER
collected.

Details on exactly what's collected are in the source code.

You can opt out of metrics collection at any time by setting the environment
variable by setting the following in your environment:

```bash
export KARIGOR_DISABLE_METRICS=1
```

Or by setting the following in your config:

```json
{
  "options": {
    "disable_metrics": true
  }
}
```

Karigor also respects the [`DO_NOT_TRACK`](https://consoledonottrack.com)
convention which can be enabled via `export DO_NOT_TRACK=1`.

## Contributing

This is a developer preview by Faisal Kabir Galib. For feedback, bug reports, and suggestions, please contact:

**Email**: faisalkabirgalib@gmail.com

## Get Help

For this developer preview, direct feedback is invaluable:

- **Email**: faisalkabirgalib@gmail.com (for bugs, feedback, and suggestions)
- **Issues**: Report bugs and feature requests via the project repository

## License

[FSL-1.1-MIT](https://github.com/charmbracelet/crush/raw/main/LICENSE.md)

---

---

**Developer Preview by Faisal Kabir Galib**
