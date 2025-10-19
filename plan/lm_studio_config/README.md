# LM Studio Configuration for Qwen3-8B

## Overview

This directory contains configuration and setup instructions for running Qwen3-8B with LM Studio on an RTX 4080 (16 GB VRAM).

## Model Recommendation

**Model:** Qwen3-8B (4-bit or 5-bit quantization)

**Why this model?**
- Fits comfortably on RTX 4080 with 16 GB VRAM
- Excellent code generation capabilities
- Fast inference with quantization
- Good balance of quality and performance

## Installation Steps

### 1. Download LM Studio

1. Visit https://lmstudio.ai/
2. Download the Windows version
3. Install and launch LM Studio

### 2. Download Qwen3-8B Model

In LM Studio:
1. Click the "Search" tab
2. Search for "Qwen3-8B" or "Qwen2.5-Coder-8B"
3. Look for quantized versions:
   - `qwen3-8b-Q4_K_M.gguf` (4-bit, ~5 GB)
   - `qwen3-8b-Q5_K_M.gguf` (5-bit, ~6 GB)
4. Download your preferred quantization

**Recommended:** Q5_K_M for better quality, Q4_K_M if you need more VRAM headroom

### 3. Start the Local Server

1. In LM Studio, go to the "Local Server" tab
2. Select your downloaded Qwen3-8B model
3. Configure settings:
   - **Port:** 1234 (default)
   - **Context Length:** 8192 or higher
   - **GPU Offload:** Maximum (all layers to GPU)
4. Click "Start Server"

The server will be available at: `http://localhost:1234/v1`

## Server Configuration

### Default Settings

```json
{
  "model": "qwen3-8b",
  "temperature": 0.7,
  "max_tokens": 2048,
  "top_p": 0.9,
  "frequency_penalty": 0.0,
  "presence_penalty": 0.0
}
```

### Optimal Settings for Code Generation

```json
{
  "model": "qwen3-8b",
  "temperature": 0.3,
  "max_tokens": 4096,
  "top_p": 0.95,
  "frequency_penalty": 0.1,
  "presence_penalty": 0.1
}
```

## Testing the Server

### Using PowerShell

```powershell
$body = @{
    model = "qwen3-8b"
    messages = @(@{
        role = "user"
        content = "Write a hello world in Python"
    })
    temperature = 0.7
} | ConvertTo-Json

Invoke-RestMethod -Uri http://localhost:1234/v1/chat/completions `
    -Method Post -Body $body -ContentType "application/json"
```

### Using curl

```bash
curl http://localhost:1234/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3-8b",
    "messages": [{"role": "user", "content": "Hello!"}],
    "temperature": 0.7
  }'
```

## Performance Notes

### RTX 4080 (16 GB)

- **Q4 Quantization:** ~5 GB VRAM, ~40-60 tokens/sec
- **Q5 Quantization:** ~6 GB VRAM, ~35-50 tokens/sec
- **Recommended:** Q5 for best quality while maintaining good speed

### Memory Management

- Model VRAM: ~5-6 GB
- Context VRAM: ~2-4 GB (depends on context length)
- System overhead: ~1-2 GB
- **Total:** ~8-12 GB (plenty of headroom on 16 GB)

## Troubleshooting

### Server won't start
- Check if port 1234 is already in use
- Ensure CUDA is properly installed
- Update GPU drivers

### Slow inference
- Increase GPU offload layers
- Reduce context length
- Use Q4 instead of Q5 quantization

### Out of memory errors
- Reduce context length to 4096
- Switch to Q4 quantization
- Close other GPU-intensive applications

## Integration with Crush

Once the server is running, Crush can connect via the OpenAI-compatible API:

```json
{
  "$schema": "https://charm.land/crush.json",
  "providers": {
    "lmstudio": {
      "name": "LM Studio",
      "base_url": "http://localhost:1234/v1/",
      "type": "openai",
      "models": [
        {
          "name": "Qwen3 8B",
          "id": "qwen3-8b",
          "context_window": 8192,
          "default_max_tokens": 4096
        }
      ]
    }
  }
}
```

## References

- LM Studio: https://lmstudio.ai/
- Qwen Models: https://huggingface.co/Qwen
- OpenAI API Compatibility: https://platform.openai.com/docs/api-reference
