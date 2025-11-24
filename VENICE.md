# Venice.ai Integration Guide

This document explains how VeniceCode integrates with Venice.ai and how to get the most out of the platform.

## What is Venice.ai?

Venice.ai is a privacy-focused AI platform that provides access to powerful open-source language models without compromising your data. Unlike other AI platforms:

- **No Training on Your Data**: Your code and conversations are never used for model training
- **Zero Retention**: Conversations are not stored long-term
- **Competitive Pricing**: Access to top models at affordable rates
- **Open Source Models**: Llama, Qwen, Mistral, Deepseek, and more

## Getting Your API Key

1. Visit [venice.ai](https://venice.ai)
2. Sign up or log in
3. Go to [Settings → API](https://venice.ai/settings/api)
4. Click "Generate New Key"
5. Copy your key (starts with a random string)

**Security Note**: Never commit your API key to version control. Use environment variables:

```bash
export VENICE_API_KEY="your-key-here"
```

## Available Models

VeniceCode supports all Venice.ai models. Here are the recommended ones for coding:

### Best for Code Generation

**Llama 3.3 70B** (Recommended)
- **ID**: `llama-3.3-70b`
- **Cost**: $0.30 per 1M tokens
- **Context**: 128K tokens
- **Best for**: General coding, complex logic, refactoring

**Deepseek Coder V2**
- **ID**: `deepseek-coder-v2`
- **Cost**: $0.30 per 1M tokens
- **Context**: 128K tokens
- **Best for**: Code-specific tasks, debugging, optimization

**Qwen 72B**
- **ID**: `qwen-72b`
- **Cost**: $0.40 per 1M tokens
- **Context**: 32K tokens
- **Best for**: High-quality code generation, complex algorithms

### Best for Speed

**Qwen 32B**
- **ID**: `qwen-32b`
- **Cost**: $0.20 per 1M tokens
- **Context**: 32K tokens
- **Best for**: Quick tasks, simple scripts, rapid iteration

**Llama 3.2 3B**
- **ID**: `llama-3.2-3b`
- **Cost**: $0.05 per 1M tokens
- **Context**: 128K tokens
- **Best for**: Very simple tasks, testing, low-cost usage

### Best for Maximum Capability

**Hermes 3 405B**
- **ID**: `hermes-3-405b`
- **Cost**: $0.50 per 1M tokens
- **Context**: 128K tokens
- **Best for**: Most complex tasks, architectural decisions, code review

## Configuration

### Default Configuration

VeniceCode comes with Venice.ai pre-configured. The default config is:

```json
{
  "providers": {
    "venice": {
      "id": "venice",
      "name": "Venice",
      "type": "openai",
      "base_url": "https://api.venice.ai/api/v1",
      "api_key": "$VENICE_API_KEY"
    }
  },
  "default_provider": "venice",
  "default_model": "llama-3.3-70b"
}
```

### Custom Configuration

Create `~/.config/venicecode/config.json` to customize:

```json
{
  "default_provider": "venice",
  "default_model": "qwen-32b",
  "venice": {
    "api_key": "$VENICE_API_KEY",
    "base_url": "https://api.venice.ai/api/v1",
    "timeout": 60,
    "max_retries": 3
  },
  "preferences": {
    "auto_save": true,
    "show_diffs": true,
    "confirm_writes": false
  }
}
```

## API Compatibility

Venice.ai uses an OpenAI-compatible API, which means:

- **Standard Endpoints**: `/v1/chat/completions`, `/v1/models`, etc.
- **Same Request Format**: Compatible with OpenAI client libraries
- **Streaming Support**: Real-time token streaming
- **Function Calling**: Tool use (experimental)

### Differences from OpenAI

1. **No Fine-Tuning**: Venice.ai doesn't support custom fine-tuned models
2. **Different Models**: Open-source models instead of GPT-4
3. **Privacy Features**: Additional privacy-focused parameters
4. **Pricing**: Per-token pricing, not subscription-based

## Privacy Features

### What Venice.ai Does NOT Do

- ❌ Store your conversations long-term
- ❌ Train models on your data
- ❌ Share your data with third parties
- ❌ Require personal information beyond email

### What You Should Still Do

- ✅ Use environment variables for API keys
- ✅ Review code before committing
- ✅ Don't include secrets in prompts
- ✅ Be aware of what you're sending to the API

## Cost Management

### Estimating Costs

**Rule of Thumb**:
- 1 token ≈ 4 characters
- 1 token ≈ 0.75 words
- Average coding session: 10K-50K tokens
- Cost per session: $0.003-$0.015 (with llama-3.3-70b)

**Example Costs**:

| Task | Tokens | Cost (Llama 3.3 70B) |
|------|--------|---------------------|
| Simple function | 500 | $0.00015 |
| Full class | 2,000 | $0.0006 |
| Code review | 5,000 | $0.0015 |
| Refactor project | 20,000 | $0.006 |

### Reducing Costs

1. **Use Smaller Models**: Qwen 32B or Llama 3.2 3B for simple tasks
2. **Limit Context**: Don't include unnecessary files
3. **Be Specific**: Clear prompts reduce back-and-forth
4. **Use Streaming**: Stop generation when you have enough

## Troubleshooting

### "Authentication failed"

**Cause**: Invalid or missing API key

**Solution**:
```bash
# Check if key is set
echo $VENICE_API_KEY

# Test key directly
curl -H "Authorization: Bearer $VENICE_API_KEY" https://api.venice.ai/api/v1/models

# If empty, set it:
export VENICE_API_KEY="your-key-here"
```

### "Rate limit exceeded"

**Cause**: Too many requests in a short time

**Solution**:
- Wait a few seconds and retry
- Venice.ai has generous rate limits for normal use
- Contact Venice support if you need higher limits

### "Model not found"

**Cause**: Model ID is incorrect or model is unavailable

**Solution**:
```bash
# List available models
curl -H "Authorization: Bearer $VENICE_API_KEY" https://api.venice.ai/api/v1/models

# Use exact model ID from the list
```

### "Write tool array error" (Fixed in VeniceCode)

**Cause**: Some models (GLM 4.6, Qwen 3 Coder) send content as arrays

**Solution**: This is already fixed in VeniceCode! If you see this error:
1. Make sure you're using VeniceCode, not vanilla Crush
2. Verify the fix is applied: `grep UnmarshalJSON internal/agent/tools/write.go`

## Advanced Usage

### Using Multiple Models

Switch models mid-session:

```bash
# In VeniceCode TUI:
# Press 'm' to change model
# Select different model from list
```

### Custom System Prompts

Create `~/.config/venicecode/prompts/`:

```bash
mkdir -p ~/.config/venicecode/prompts
echo "You are an expert in security. Focus on secure coding practices." > ~/.config/venicecode/prompts/security.txt
```

Use with:
```bash
venicecode --prompt security
```

### Batch Processing

Process multiple files:

```bash
for file in src/*.py; do
  echo "Add type hints to $file" | venicecode --file "$file" --non-interactive
done
```

## Venice.ai Resources

- **Website**: [venice.ai](https://venice.ai)
- **Documentation**: [docs.venice.ai](https://docs.venice.ai)
- **API Reference**: [docs.venice.ai/api](https://docs.venice.ai/api)
- **Discord**: [venice.ai/discord](https://venice.ai/discord)
- **Status**: [status.venice.ai](https://status.venice.ai)

## Support

For VeniceCode issues:
- [GitHub Issues](https://github.com/georgeglarson/venicecode/issues)

For Venice.ai API issues:
- [Venice Support](https://venice.ai/support)
- [Venice Discord](https://venice.ai/discord)

---

**Happy coding with privacy! 🔐**
