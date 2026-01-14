# AI Provider APIs (Consumed)

Crush acts as a client for multiple AI provider APIs. It uses the `fantasy` abstraction layer to normalize interactions across different providers.

## Supported Providers

| Provider | Type | API Endpoint (Default) |
|----------|------|------------------------|
| **Anthropic** | `anthropic` | `https://api.anthropic.com/v1` |
| **OpenAI** | `openai` | `https://api.openai.com/v1` |
| **Google Gemini** | `gemini` | `https://generativelanguage.googleapis.com` |
| **Azure OpenAI** | `azure` | Configurable |
| **Bedrock (AWS)** | `bedrock` | Region-specific |
| **OpenRouter** | `openai` | `https://openrouter.ai/api/v1` |
| **Vertex AI** | `vertexai` | Configurable |
| **OpenAI-Compatible** | `openai-compat` | Configurable |

## Authentication Mechanisms

### API Keys
Most providers use standard API keys passed via headers:
- `Authorization: Bearer <key>` (OpenAI, OpenRouter, etc.)
- `x-api-key: <key>` (Anthropic)
- URL parameters (Google AI)

### OAuth2
Some providers like GitHub Copilot and Hyper use OAuth2:
- Supports device flow for authentication.
- Automatically handles token refreshing when expired.

### AWS Credentials
Bedrock uses standard AWS credential chains (IAM roles, environment variables, or `~/.aws/credentials`).

## Message Formats
Crush normalizes messages into a common format before sending them to providers:
- **System Messages:** Provides core instructions and context.
- **User Messages:** User input and file attachments.
- **Assistant Messages:** Model responses.
- **Tool Calls:** Requests from the model to execute a tool.
- **Tool Results:** Output from executed tools returned to the model.

## Media Support
- **Images:** Supported natively for Anthropic and Bedrock.
- **Workaround:** For providers that don't support images in tool results (OpenAI, Gemini), Crush injects the image into a subsequent user message.
