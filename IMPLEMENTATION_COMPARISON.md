# Lash vs OpenCode Implementation Comparison

## Key Differences and Verification

### 1. Beta Headers
**Lash (anthropic.go:49):**
```
anthropic-beta: oauth-2025-04-20,claude-code-20250219,planning-mode-2025-06-10,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14
```

**OpenCode (provider.ts:56-57):**
```
anthropic-beta: oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14
```

**Status:** ✅ Fixed - Added `planning-mode-2025-06-10` to lash to support Claude Code Plan mode

### 2. OAuth Implementation
**Both implementations:**
- Use same client ID: `9d1c250a-e61b-44d9-88ed-5944d1962f5e`
- Support both "max" (claude.ai) and "console" (console.anthropic.com) modes
- Implement PKCE flow correctly
- Use same token exchange endpoint: `https://console.anthropic.com/v1/oauth/token`

**Status:** ✅ Matching

### 3. Header Handling
**Lash:**
- Uses custom `claudeCodeRoundTripper` to intercept requests
- Removes x-api-key headers
- Sets authorization header with Bearer token
- Applies beta headers

**OpenCode:**
- Uses custom fetch function
- Removes x-api-key headers  
- Sets authorization header with Bearer token
- Applies beta headers

**Status:** ✅ Equivalent approaches

### 4. Token Refresh
**Both implementations:**
- Check token expiry before use
- Refresh using refresh_token grant type
- Store updated tokens after refresh

**Status:** ✅ Matching

### 5. API Key Creation
**OpenCode (auth.ts:239):**
- Has option to create API key via `/api/oauth/claude_cli/create_api_key` endpoint

**Lash:**
- Does not have this option (OAuth only)

**Status:** ⚠️ Feature difference - OpenCode has additional API key creation option

## Summary
The main issue was the missing `planning-mode-2025-06-10` beta header in lash, which has been fixed. Both implementations now properly support Claude Code OAuth authentication with all required beta features.

The implementations are functionally equivalent with one minor difference: OpenCode offers an additional option to create API keys from OAuth tokens, while lash uses OAuth tokens directly.