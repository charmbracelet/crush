# AI SDK Bridge for Claude Code OAuth Support

## Problem
Claude Code OAuth tokens are restricted and cannot be used for direct Anthropic API calls. They return the error:
```
"This credential is only authorized for use with Claude Code and cannot be used for other API requests."
```

OpenCode works around this by using the Vercel AI SDK, which has special handling for Claude Code OAuth tokens.

## Solution
Since lash is written in Go and the AI SDK is JavaScript/TypeScript, we've implemented a Node.js bridge service that:
1. Runs locally on port 8765
2. Receives API requests from lash
3. Uses the Vercel AI SDK to make proper Claude Code OAuth requests
4. Returns responses in Anthropic API format

## Architecture

```
lash (Go) 
  ↓ [detects Bearer token]
  ↓ [redirects to bridge]
AI SDK Bridge (Node.js)
  ↓ [uses @ai-sdk/anthropic]
  ↓ [custom fetch with OAuth]
Anthropic API
```

## Setup

### 1. Install Node.js dependencies
```bash
cd internal/llm/provider/ai-sdk-bridge
npm install
```

### 2. Start the bridge service
```bash
./start-bridge.sh
# Or manually:
npm start
```

### 3. Use lash with Claude Code OAuth
The bridge will automatically be used when lash detects Bearer tokens (OAuth) instead of API keys.

## How It Works

1. **Detection**: When lash's Anthropic provider detects a Bearer token (starts with "Bearer "), it knows this is a Claude Code OAuth token.

2. **Redirection**: Instead of using the Anthropic API directly, lash redirects requests to `http://localhost:8765` (configurable via `AI_SDK_BRIDGE_URL` env var).

3. **Bridge Processing**: The Node.js bridge:
   - Receives the request with Bearer token
   - Creates an Anthropic client using the AI SDK
   - Sets up custom fetch with proper OAuth headers
   - Makes the actual API call using the AI SDK
   - Converts responses back to Anthropic API format

4. **Response**: Lash receives the response as if it came from the Anthropic API directly.

## Configuration

- **Bridge URL**: Set `AI_SDK_BRIDGE_URL` environment variable (default: `http://localhost:8765`)
- **Bridge Port**: Set `AI_SDK_BRIDGE_PORT` environment variable (default: `8765`)

## Testing

1. Check bridge health:
```bash
curl http://localhost:8765/health
```

2. Test with lash:
```bash
./lash run "What is 2+2?"
```

## Key Differences from Direct API

The AI SDK bridge handles:
- Proper OAuth token flow
- Claude Code specific headers and protocols
- Token refresh (handled by AI SDK)
- Special request formatting for Claude Code

## Files Modified

- `/internal/llm/provider/anthropic.go` - Modified to detect OAuth and redirect to bridge
- `/internal/llm/provider/ai-sdk-bridge/` - New directory with Node.js bridge implementation
  - `package.json` - Node.js dependencies
  - `server.js` - Express server implementing the bridge
  - `start-bridge.sh` - Startup script

## Dependencies

- Node.js 18+ 
- @ai-sdk/anthropic
- ai (Vercel AI SDK)
- express
- cors