# VeniceCode

**The privacy-first AI coding agent optimized for Venice.ai**

VeniceCode is a specialized fork of [Crush](https://github.com/charmbracelet/crush) with enhanced support for Venice.ai's privacy-focused AI platform. Get the power of Llama 3.3 70B, Qwen, Deepseek Coder, and more—all with Venice.ai's commitment to privacy and competitive pricing.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org)
[![Venice.ai](https://img.shields.io/badge/Powered%20by-Venice.ai-blueviolet)](https://venice.ai)

## Why VeniceCode?

- **Privacy-First**: Built for Venice.ai's zero-retention, privacy-focused platform
- **Cost-Effective**: Access powerful models at competitive rates
- **GLM/Qwen Compatible**: Fixed write tool handles array content from advanced models
- **Venice-Optimized**: Pre-configured with Venice.ai's best models
- **Open Source**: Full transparency, MIT licensed

## Quick Start

### Installation

**macOS (Homebrew)**
```bash
brew install georgeglarson/tap/venicecode
```

**Linux / macOS (Direct Download)**
```bash
curl -L https://github.com/georgeglarson/venicecode/releases/latest/download/venicecode-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m) -o venicecode
chmod +x venicecode
sudo mv venicecode /usr/local/bin/
```

**Build from Source**
```bash
git clone https://github.com/georgeglarson/venicecode.git
cd venicecode
go build -o venicecode
```

### Setup

1. **Get your Venice.ai API key**  
   Visit [venice.ai/settings/api](https://venice.ai/settings/api) and generate a key.

2. **Set the API key**
   ```bash
   export VENICE_API_KEY="your-key-here"
   ```

3. **Run VeniceCode**
   ```bash
   venicecode
   ```

4. **Select Venice provider and start coding!**

## Features

### 🔐 Privacy-Focused
Venice.ai doesn't train on your code. Your conversations and code stay private.

### 🚀 Powerful Models
Access to cutting-edge open-source models:
- **Llama 3.3 70B** - Best overall performance
- **Qwen 32B/72B** - Excellent for coding tasks
- **Deepseek Coder V2** - Specialized for code generation
- **Mistral Nemo** - Fast and efficient
- **Hermes 3 405B** - Maximum capability

### 🛠️ Enhanced Write Tool
VeniceCode includes a critical fix for the write tool that handles both string and array content formats, ensuring compatibility with GLM 4.6 and Qwen 3 Coder 480B models.

**The Problem (Upstream Crush)**:
```
The write tool was called with invalid arguments:
"Invalid input: expected string, received array"
```

**The Solution (VeniceCode)**:
Seamlessly handles both content formats from any model.

### 💰 Cost-Effective
Venice.ai offers competitive pricing:
- Llama 3.3 70B: $0.30 per 1M tokens
- Qwen 32B: $0.20 per 1M tokens
- Deepseek Coder V2: $0.30 per 1M tokens

## Usage

### Basic Usage

```bash
# Start VeniceCode
venicecode

# In the TUI:
# 1. Select "Venice" provider
# 2. Choose your model (e.g., llama-3.3-70b)
# 3. Start coding!
```

### Example Tasks

**Create a web server**
```
Create a simple HTTP server in Go that serves static files
```

**Debug code**
```
Find and fix the bug in this Python function:
[paste your code]
```

**Refactor**
```
Refactor this JavaScript code to use async/await instead of callbacks
```

**Generate tests**
```
Write unit tests for this TypeScript class
```

## Configuration

VeniceCode comes pre-configured for Venice.ai, but you can customize it:

**~/.config/venicecode/config.json**
```json
{
  "default_provider": "venice",
  "default_model": "llama-3.3-70b",
  "venice": {
    "api_key": "$VENICE_API_KEY",
    "base_url": "https://api.venice.ai/api/v1"
  }
}
```

## Documentation

- [Getting Started Guide](docs/venice/getting-started.md)
- [Model Comparison](docs/venice/models.md)
- [Troubleshooting](docs/venice/troubleshooting.md)
- [Advanced Features](examples/venice/advanced-features.md)

## Differences from Crush

VeniceCode is based on Crush with these enhancements:

1. **Write Tool Fix**: Handles array content from GLM/Qwen models
2. **Venice Provider**: Pre-configured Venice.ai integration
3. **Privacy Focus**: Documentation emphasizes Venice.ai's privacy features
4. **Model Selection**: Curated list of best Venice.ai models

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md).

### Reporting Issues

- **VeniceCode-specific issues**: [GitHub Issues](https://github.com/georgeglarson/venicecode/issues)
- **Upstream Crush issues**: [Crush Issues](https://github.com/charmbracelet/crush/issues)
- **Venice.ai API issues**: [Venice Support](https://venice.ai/support)

## Roadmap

### v1.0 (Current)
- [x] Venice.ai provider integration
- [x] Write tool array content fix
- [x] Basic documentation

### v1.1 (Planned)
- [ ] venice-dev-tools SDK integration
- [ ] Real-time cost tracking in TUI
- [ ] Character-based coding personas
- [ ] Image generation for UI mockups

### v2.0 (Future)
- [ ] Web3 development mode
- [ ] Advanced privacy features
- [ ] Custom model fine-tuning support

## Credits

- **Crush**: Created by [Charmbracelet](https://github.com/charmbracelet)
- **Venice.ai**: Privacy-focused AI platform
- **VeniceCode**: Maintained by [George Larson](https://github.com/georgeglarson)

## License

MIT License - see [LICENSE](LICENSE) for details.

VeniceCode is a fork of Crush, which is also MIT licensed.

## Support

- **Documentation**: [docs/venice/](docs/venice/)
- **Issues**: [GitHub Issues](https://github.com/georgeglarson/venicecode/issues)
- **Venice.ai**: [venice.ai](https://venice.ai)
- **Discussions**: [GitHub Discussions](https://github.com/georgeglarson/venicecode/discussions)

---

**Made with ❤️ for the Venice.ai community**
