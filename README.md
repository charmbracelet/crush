> [!WARNING]
> 🚧 This is a pre-release under heavy, active development. Things are still in flux but we’re excited to share early progress.

# Crush

<p>
    <a href="https://github.com/charmbracelet/crush/releases"><img src="https://img.shields.io/github/release/charmbracelet/crush" alt="Latest Release"></a>
    <a href="https://github.com/charmbracelet/crush/actions"><img src="https://github.com/charmbracelet/crush/workflows/build/badge.svg" alt="Build Status"></a>
</p>

Crush is a tool for building software with AI.

## Installation

Nightly builds are available while Crush is in development.

- [Packages](https://github.com/charmbracelet/crush/releases/tag/nightly) are available in Debian and RPM formats
- [Binaries](https://github.com/charmbracelet/crush/releases/tag/nightly) are available for Linux and macOS

You can also just install it with go:

```
git clone git@github.com:charmbracelet/crush.git
cd crush
go install
```

Note that Crush doesn't support Windows yet, however Windows support is planned and in progress.

## Getting Started

For now, the quickest way to get started is to set an environment variable for
your preferred provider. Note that you can switch between providers mid-
sessions, so you're welcome to set environment variables for multiple
providers.

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
      "disabled": false,
      "command": "gopls"
    },
    "typescript": {
      "disabled": false,
      "command": "typescript-language-server",
      "args": ["--stdio"]
    },
    "nix": {
      "command": "alejandra"
    }
  }
}
```

### Amazon Bedrock

To use AWS Bedrock with Claude models, configure your AWS credentials and region:

```json
{
  "providers": [
    {
      "id": "bedrock",
      "provider_type": "bedrock",
      "extra_params": {
        "region": "us-east-1"
      }
    }
  ]
}
```

Bedrock uses your AWS credentials from environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`) or AWS credential profiles. The region can be specified in the configuration or via the `AWS_REGION` environment variable.

### Google Vertex AI

For Google Cloud Vertex AI with Gemini models, configure your project and location:

```json
{
  "providers": [
    {
      "id": "vertexai",
      "provider_type": "vertexai",
      "extra_headers": {
        "project": "your-gcp-project-id",
        "location": "us-central1"
      }
    }
  ]
}
```

Vertex AI uses Google Cloud authentication. Ensure you have the `GOOGLE_APPLICATION_CREDENTIALS` environment variable set or are authenticated via `gcloud auth application-default login`.

### OpenAI-Compatible APIs

Crush supports all OpenAI-compatible APIs, including local models via Ollama:

```json
{
  "providers": [
    {
      "id": "ollama",
      "provider_type": "openai",
      "base_url": "http://localhost:11434/v1",
      "models": [
        {
          "id": "llama3.2:3b",
          "name": "Llama 3.2 3B",
          "context_window": 8192,
          "default_max_tokens": 4096
        }
      ]
    }
  ]
}
```

For other OpenAI-compatible providers, adjust the `base_url` and provide an `api_key` if required:

```json
{
  "providers": [
    {
      "id": "custom-openai",
      "provider_type": "openai",
      "base_url": "https://api.example.com/v1",
      "api_key": "your-api-key"
    }
  ]
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
