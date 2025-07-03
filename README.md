> [!WARNING]
> 🚧 This is a pre-release under heavy, active development. Things are still in flux but we’re excited to share early progress.

# Crush

<p>
    <a href="https://github.com/charmbracelet/crush/releases"><img src="https://img.shields.io/github/release/charmbracelet/crush" alt="Latest Release"></a>
    <a href="https://github.com/charmbracelet/crush/actions"><img src="https://github.com/charmbracelet/crush/workflows/build/badge.svg" alt="Build Status"></a>
</p>

Crush is a tool for building software with AI.

## Installation

* [Packages](https://github.com/charmbracelet/crush/releases/tag/nightly) are available in Debian and RPM formats
* [Binaries](https://github.com/charmbracelet/crush/releases/tag/nightly) are available for Linux and macOS

Or just install it with go:

```
git clone git@github.com:charmbracelet/crush.git
cd crush
go install
```

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

## License

[MIT](https://github.com/charmbracelet/crush/raw/main/LICENSE)

---

Part of [Charm](https://charm.sh).

<a href="https://charm.sh/"><img alt="The Charm logo" width="400" src="https://stuff.charm.sh/charm-badge.jpg" /></a>

<!--prettier-ignore-->
Charm热爱开源 • Charm loves open source
