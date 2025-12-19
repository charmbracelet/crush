<div align="center">

# 🏗️ Karigor

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-FSL--1.1--MIT-blue?style=for-the-badge)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey?style=for-the-badge)](https://github.com/yourusername/karigor/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/yourusername/karigor/release.yml?style=for-the-badge)](https://github.com/yourusername/karigor/actions)

<pre>
    __ __           _
   / //_/___ ______(_)___  _____
  / ,< / __ `/ ___/ / __ `/ ___/
 / /| / /_/ / /  / / /_/ / /
/_/ |_|\__,_/_/  /_/\__,_/_/

    _   _           _   _
   / \ | | ___  ___| |_(_) ___
  / _ \| |/ _ \/ __| __| |/ __|
 / ___ \ |  __/ (__| |_| | (__
/_/   \_\_|\___|\___|\__|_|\___|
</pre>

### 🚀 Your AI-Powered Terminal Development Assistant

**Intelligent coding companion that lives in your terminal.** Streamline your development workflow with AI assistance, multi-model support, and seamless tool integration.

</div>

## ✨ Why Karigor?

Karigor (কারিগর - Bengali for "Artisan") is designed to be your personal development assistant, bringing the power of multiple AI models directly to your terminal experience. Unlike other AI tools, Karigor integrates with your development environment, understands your codebase context, and provides intelligent assistance throughout your coding journey.

### 🎯 Core Features

- **🤖 Multi-Model Support**: Switch between GPT-4, Claude, Gemini, and custom providers mid-session
- **🔧 Integrated Development**: Built-in file operations, git integration, and terminal commands
- **🧠 Context-Aware**: Leverages Language Server Protocol (LSP) for deep code understanding
- **📚 Session Management**: Maintain multiple conversation contexts per project
- **🔌 Extensible via MCP**: Add powerful capabilities through Model Context Protocol servers
- **⚡ Zero Configuration**: Pre-configured with intelligent MCP servers (karigor-mcp-server, karigor-web-search, karigor-web-reader)
- **🌍 Cross-Platform**: Native support for Linux, macOS, Windows, BSD systems

---

## 🚀 Quick Start

### Installation

#### 📦 Package Managers (Recommended)

<details>
<summary><strong>🍺 macOS / Linux (Homebrew)</strong></summary>

```bash
brew install yourusername/tap/karigor
```

</details>

<details>
<summary><strong>🪟 Windows (Scoop)</strong></summary>

```bash
scoop bucket add yourusername https://github.com/yourusername/scoop-bucket
scoop install karigor
```

</details>

<details>
<summary><strong>🐧 Linux (Debian/Ubuntu)</strong></summary>

```bash
wget https://github.com/yourusername/karigor/releases/latest/download/karigor_Linux_x86_64.deb
sudo dpkg -i karigor_Linux_x86_64.deb
```

</details>

<details>
<summary><strong>🐧 Linux (RedHat/Fedora)</strong></summary>

```bash
wget https://github.com/yourusername/karigor/releases/latest/download/karigor-*.rpm
sudo rpm -i karigor-*.rpm
```

</details>

<details>
<summary><strong>🏛️ Arch Linux (AUR)</strong></summary>

```bash
yay -S karigor
```

</details>

#### 🔧 Direct Download

Download the latest binary from [GitHub Releases](https://github.com/yourusername/karigor/releases/latest) for your platform:

```bash
# Linux (x64)
wget https://github.com/yourusername/karigor/releases/latest/download/karigor_Linux_x86_64.tar.gz
tar -xzf karigor_Linux_x86_64.tar.gz
sudo mv karigor /usr/local/bin/

# macOS (Apple Silicon)
wget https://github.com/yourusername/karigor/releases/latest/download/karigor_Darwin_arm64.tar.gz
tar -xzf karigor_Darwin_arm64.tar.gz
sudo mv karigor /usr/local/bin/
```

#### 🐳 Docker

```bash
docker run -it --rm -v $(pwd):/workspace ghcr.io/yourusername/karigor:latest
```

#### 🛠️ Build from Source

```bash
git clone https://github.com/yourusername/karigor.git
cd karigor
go build .
./karigor --help
```

---

## 🎮 Usage

### First Launch

```bash
# Start Karigor with interactive TUI
karigor

# Or run a single command non-interactively
karigor run "Explain this Go function" --file main.go
```

### Getting Started

When you first run Karigor, it will:
1. **Auto-configure** default MCP servers (karigor-mcp-server, karigor-web-search, karigor-web-reader, sequential-thinking)
2. **Detect** your development environment and available tools
3. **Guide** you through provider setup with your preferred AI model

### Basic Workflow

```bash
# Launch Karigor
karigor

# In the TUI:
# 1. Choose your AI provider (OpenAI, Anthropic, Google, etc.)
# 2. Start coding with AI assistance
# 3. Use integrated tools for file operations, git commands, web search
# 4. Switch between different AI models as needed
```

### Configuration

Karigor automatically creates configuration files:

- **Global config**: `~/.config/karigor/karigor.json`
- **Project config**: `.karigor/karigor.json`

#### Basic Configuration Example

```json
{
  "$schema": "https://karigor.dev/schema.json",
  "models": {
    "large": {
      "model": "gpt-4o",
      "provider": "openai"
    },
    "small": {
      "model": "claude-3-haiku",
      "provider": "anthropic"
    }
  },
  "mcp": {
    "karigor-mcp-server": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@z_ai/mcp-server"]
    },
    "karigor-web-search": {
      "type": "http",
      "url": "https://api.z.ai/api/mcp/web_search_prime/mcp"
    }
  },
  "permissions": {
    "allowed_tools": ["view", "edit", "bash", "grep"]
  }
}
```

---

## 🛠️ Features Deep Dive

### 🔌 Built-in MCP Servers

Karigor comes pre-configured with intelligent MCP servers:

#### 🧠 karigor-mcp-server
- **Package**: `@z_ai/mcp-server`
- **Purpose**: Advanced code analysis and generation
- **Features**: Context-aware code suggestions, refactoring, documentation

#### 🔍 karigor-web-search
- **Purpose**: Real-time web search integration
- **Features**: Stack Overflow search, documentation lookup, trend analysis

#### 📖 karigor-web-reader
- **Purpose**: Web content extraction and analysis
- **Features**: Article summarization, code snippet extraction, API documentation parsing

#### 🤔 sequential-thinking
- **Package**: `@modelcontextprotocol/server-sequential-thinking`
- **Purpose**: Complex problem decomposition
- **Features**: Step-by-step reasoning, planning, logical breakdown

### 🔧 Integrated Tools

- **File Operations**: `view`, `edit`, `write`, `ls`, `glob`
- **Search**: `grep`, `rg` (ripgrep integration)
- **Git**: Full git workflow support
- **Terminal**: Execute shell commands safely
- **Web**: Fetch, read, and analyze web content
- **LSP**: Language-aware code intelligence

### 🧠 AI Provider Support

#### Supported Providers
- **OpenAI**: GPT-4, GPT-3.5, custom models
- **Anthropic**: Claude 3 family (Opus, Sonnet, Haiku)
- **Google**: Gemini family
- **Custom**: Any OpenAI-compatible API endpoint

#### Multi-Provider Example
```json
{
  "providers": {
    "openai": {
      "api_key": "$OPENAI_API_KEY",
      "base_url": "https://api.openai.com/v1"
    },
    "anthropic": {
      "api_key": "$ANTHROPIC_API_KEY"
    },
    "custom": {
      "api_key": "$CUSTOM_API_KEY",
      "base_url": "https://your-custom-endpoint.com/v1",
      "type": "openai-compat"
    }
  }
}
```

---

## 🎯 Use Cases

### 👨‍💻 For Developers

- **Code Review**: AI-powered code analysis and suggestions
- **Documentation**: Auto-generate and maintain documentation
- **Debugging**: Intelligent error analysis and solutions
- **Refactoring**: Safe, context-aware code transformations
- **Learning**: Understand complex codebases with AI explanations

### 🏢 For Teams

- **Onboarding**: Help new developers understand project structure
- **Code Standards**: Ensure consistent coding practices
- **Knowledge Sharing**: Preserve project knowledge in AI conversations
- **Prototyping**: Quickly prototype new features with AI assistance

### 🔬 For Researchers

- **Experimentation**: Rapidly test and iterate on ideas
- **Analysis**: Analyze datasets and research papers
- **Documentation**: Generate research documentation and reports
- **Code Generation**: Create analysis scripts and tools

---

## 🎨 Customization

### Shell Completions

Karigor provides shell completions for bash, zsh, and fish:

```bash
# Enable bash completions
source <(karigor completion bash)

# Enable zsh completions
source <(karigor completion zsh)

# Enable fish completions
karigor completion fish | source
```

### Custom MCP Servers

Add your own MCP servers to extend functionality:

```json
{
  "mcp": {
    "my-custom-server": {
      "type": "stdio",
      "command": "python",
      "args": ["/path/to/my/mcp-server.py"],
      "env": {
        "CUSTOM_API_KEY": "$MY_API_KEY"
      }
    }
  }
}
```

### Themes and Appearance

Customize the TUI appearance:

```json
{
  "options": {
    "tui": {
      "compact_mode": false,
      "diff_mode": "split",
      "completions": {
        "max_depth": 10,
        "max_items": 1000
      }
    }
  }
}
```

---

## 🔐 Security

- **Sandboxed Execution**: All operations run in controlled environments
- **Permission System**: Fine-grained control over tool access
- **API Key Security**: Secure storage and management of API keys
- **Audit Logging**: Complete audit trail of all AI interactions
- **Local Processing**: Sensitive code stays on your machine

---

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/yourusername/karigor.git
cd karigor

# Install dependencies
go mod download

# Run tests
go test ./...

# Run with debug mode
go run . --debug

# Build for development
task build
```

---

## 📊 Architecture

Karigor is built with:
- **Go**: For performance and cross-platform support
- **Bubble Tea**: Modern terminal user interface framework
- **MCP SDK**: Extensible protocol integration
- **SQLite**: Local data storage for conversations
- **Language Server Protocol**: Deep code understanding

---

## 🆘 Support & Community

### Getting Help
- **Documentation**: [docs.karigor.dev](https://docs.karigor.dev)
- **Issues**: [GitHub Issues](https://github.com/yourusername/karigor/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourusername/karigor/discussions)

### Feedback
For this developer preview, please send feedback to: **faisalkabirgalib@gmail.com**

### Community
- **Discord**: [Join our Discord](https://discord.gg/karigor)
- **Twitter**: [@karigor_dev](https://twitter.com/karigor_dev)

---

## 📈 Roadmap

### Upcoming Features
- [ ] **Plugin System**: Custom plugin development
- [ ] **Team Collaboration**: Shared sessions and contexts
- [ ] **Advanced Analytics**: Usage insights and optimization
- [ ] **IDE Integrations**: VS Code, IntelliJ, and more
- [ ] **Mobile Apps**: iOS and Android companions

### Current Development Focus
- [ ] Enhanced multi-modal support (images, audio)
- [ ] Advanced debugging capabilities
- [ ] Performance optimizations
- [ ] Extended LSP support

---

## 📄 License

Karigor is available under the [FSL-1.1-MIT License](LICENSE). This means it's free for personal and commercial use, with additional freedoms for the community.

---

## 🙏 Acknowledgments

- **Charmbracelet** - For the amazing Bubble Tea framework and inspiration
- **Model Context Protocol** - For the extensible AI integration standard
- **OpenAI, Anthropic, Google** - For providing incredible AI capabilities
- **Our Contributors** - For making Karigor better every day

---

<div align="center">

**Made with ❤️ by [Faisal Kabir Galib](https://github.com/faisalkabirgalib)**

[![Star History Chart](https://api.star-history.com/svg?repos=yourusername/karigor&type=Date)](https://star-history.com/#yourusername/karigor&Date)

*If you find Karigor helpful, please ⭐ star us on GitHub!*

</div>