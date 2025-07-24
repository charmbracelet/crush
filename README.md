> [!WARNING]
> 🚧 This is a pre-release under heavy, active development. Things are still in flux but we’re excited to share early progress.

# Crush

<p>
    <a href="https://github.com/charmbracelet/crush/releases"><img src="https://img.shields.io/github/release/charmbracelet/crush" alt="Latest Release"></a>
    <a href="https://github.com/charmbracelet/crush/actions"><img src="https://github.com/charmbracelet/crush/workflows/build/badge.svg" alt="Build Status"></a>
</p>

Crush is a tool for building software with AI.

## Installation

Crush has first class support for macOS, Linux, and Windows.

Nightly builds are available while Crush is in development.

- [Packages](https://github.com/charmbracelet/crush/releases/tag/nightly) are available in Debian, RPM, APK, and PKG formats
- [Binaries](https://github.com/charmbracelet/crush/releases/tag/nightly) are available for Linux, macOS and Windows

You can also just install it with go:

```
git clone git@github.com:charmbracelet/crush.git
cd crush
go install
```

<details>
<summary>Not a developer? Here’s a quick how-to.</summary>

Download the latest [nightly release](https://github.com/charmbracelet/crush/releases) for your system. The [macOS ARM64 one](https://github.com/charmbracelet/crush/releases/download/nightly/crush_0.1.0-nightly_Darwin_arm64.tar.gz) is most likely what you want.

Next, open a terminal and run the following commands:

```bash
cd ~/Downloads
tar -xvzf crush_0.1.0-nightly_Darwin_arm64.tar.gz -C crush
sudo mv ./crush/crush /usr/local/bin/crush
rm -rf ./crush
```

Then, run Crush by typing `crush`.

---

</details>

## Getting Started

The quickest way to get started to grab an API key for your preferred
provider such as Anthropic, OpenAI, or Groq, and just start Crush. You'll be
prompted to enter your API key.

That said, you can also set environment variables for preferred providers:

| Environment Variable       | Provider                                           |
| -------------------------- | -------------------------------------------------- |
| `ANTHROPIC_API_KEY`        | Anthropic                                          |
| `OPENAI_API_KEY`           | OpenAI                                             |
| `GEMINI_API_KEY`           | Google Gemini                                      |
| `VERTEXAI_PROJECT`         | Google Cloud VertexAI (Gemini)                     |
| `VERTEXAI_LOCATION`        | Google Cloud VertexAI (Gemini)                     |
| `GROQ_API_KEY`             | Groq                                               |
| `AWS_ACCESS_KEY_ID`        | AWS Bedrock (Claude)                               |
| `AWS_SECRET_ACCESS_KEY`    | AWS Bedrock (Claude)                               |
| `AWS_REGION`               | AWS Bedrock (Claude)                               |
| `AZURE_OPENAI_ENDPOINT`    | Azure OpenAI models                                |
| `AZURE_OPENAI_API_KEY`     | Azure OpenAI models (optional when using Entra ID) |
| `AZURE_OPENAI_API_VERSION` | Azure OpenAI models                                |

## Configuration

For many use cases, Crush can be run with no config. That said, if you do need config, it can be added either local to the project itself, or globally. Configuration has the following priority:

1. `.crush.json`
2. `crush.json`
3. `$HOME/.config/crush/crush.json`

### LSPs

Crush can use LSPs for additional context to help inform its decisions, just like you would. LSPs can be added manually like so:

```json
{
  "lsp": {
    "go": {
      "command": "gopls"
    },
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"]
    },
    "nix": {
      "command": "alejandra"
    }
  }
}
```

### MCPs

Crush supports Model Context Protocol (MCP) servers through three transport types: `stdio` for command-line servers, `http` for HTTP endpoints, and `sse` for Server-Sent Events. Environment variable expansion is supported using `$(echo $VAR)` syntax.

```json
{
  "mcp": {
    "filesystem": {
      "type": "stdio",
      "command": "node",
      "args": ["/path/to/mcp-server.js"],
      "env": {
        "NODE_ENV": "production"
      }
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "$(echo Bearer $GH_MCP_TOKEN)"
      }
    },
    "streaming-service": {
      "type": "sse",
      "url": "https://example.com/mcp/sse",
      "headers": {
        "API-Key": "$(echo $API_KEY)"
      }
    }
  }
}
```

### Logging

Enable debug logging with the `-d` flag or in config. View logs with `crush logs`. Logs are stored in `.crush/logs/crush.log`.

```bash
# Run with debug logging
crush -d

# View last 1000 lines
crush logs

# Follow logs in real-time
crush logs -f

# Show last 500 lines
crush logs -t 500
```

Add to your `crush.json` config file:

```json
{
  "options": {
    "debug": true,
    "debug_lsp": true
  }
}
```

### Configurable Default Permissions

Crush includes a permission system to control which tools can be executed without prompting. You can configure allowed tools in your `crush.json` config file:

```json
{
  "permissions": {
    "allowed_tools": [
      "view",
      "ls",
      "grep",
      "edit:write",
      "mcp_context7_get-library-doc"
    ]
  }
}
```

The `allowed_tools` array accepts:

- Tool names (e.g., `"view"`) - allows all actions for that tool
- Tool:action combinations (e.g., `"edit:write"`) - allows only specific actions

You can also skip all permission prompts entirely by running Crush with the `--yolo` flag.

### OpenAI-Compatible APIs

Crush supports all OpenAI-compatible APIs. Here's an example configuration for Deepseek, which uses an OpenAI-compatible API. Don't forget to set `DEEPSEEK_API_KEY` in your environment.

```json
{
  "providers": {
    "deepseek": {
      "provider_type": "openai",
      "base_url": "https://api.deepseek.com/v1",
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

## Whatcha think?

We’d love to hear your thoughts on this project. Feel free to drop us a note!

- [Twitter](https://twitter.com/charmcli)
- [The Fediverse](https://mastodon.social/@charmcli)
- [Discord](https://charm.sh/chat)

## License

[MIT](https://github.com/charmbracelet/crush/raw/main/LICENSE)

---

Part of [Charm](https://charm.land).

<a href="https://charm.sh/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-banner-next.jpg" /></a>

<!--prettier-ignore-->
Charm热爱开源 • Charm loves open source
