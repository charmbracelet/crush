# Double Base64 Encoding

## The Problem

Passing base64-encoded data to an API that expects raw bytes, causing double encoding.

## Root Cause

`fantasy.NewImageResponse(data []byte, mediaType string)` expects **raw bytes**. Internally, `executeSingleTool` base64-encodes `ToolResponse.Data` before storing in `ToolResultOutputContentMedia.Data`:

```go
// fantasy/agent.go
result.Result = ToolResultOutputContentMedia{
    Data:      base64.StdEncoding.EncodeToString(toolResult.Data),  // encodes here
    MediaType: toolResult.MediaType,
}
```

If the caller pre-encodes the data, it becomes base64-of-base64:

```
Raw bytes → caller encodes → base64 string → fantasy encodes → base64(base64 string)
```

## Symptoms

- Images sent to LLM appear corrupted or blank
- "Could not process image" errors from Anthropic/Gemini
- Tool results with media content fail unexpectedly
- Clipboard images work fine, but tool-generated images fail

## Affected Code Paths

| Path | Original (buggy) | Fixed |
|------|------------------|-------|
| `view.go` image file | `NewImageResponse([]byte(base64.EncodeToString(raw)), ...)` | `NewImageResponse(raw, ...)` |
| MCP tool screenshot | `normalizeMCPMediaPayload` returns base64 bytes | Returns raw bytes |
| Additional MCP media | Metadata stored base64 bytes | Encode only when serializing to JSON |

## The Fix

Always pass **raw bytes** to `NewImageResponse` / `NewMediaResponse`:

```go
// WRONG — double encoding
encoded := base64.StdEncoding.EncodeToString(imageData)
return fantasy.NewImageResponse([]byte(encoded), mimeType)

// CORRECT — single encoding (fantasy handles it)
return fantasy.NewImageResponse(imageData, mimeType)
```

## Prevention Checklist

When using `fantasy.NewImageResponse` or `fantasy.NewMediaResponse`:

1. ✅ Verify the data being passed is **raw bytes** (not a base64 string)
2. ✅ Check if any upstream source already encoded the data (MCP, file read)
3. ✅ If upstream is base64, decode it first before passing to fantasy
4. ✅ For metadata JSON serialization, encode at that point only

## Related APIs

This pattern applies to any API with similar contracts:

- `fantasy.NewImageResponse(data []byte, ...)` — expects raw bytes
- `fantasy.NewMediaResponse(data []byte, ...)` — expects raw bytes
- `fantasy.ToolResponse{Data: []byte}` — expects raw bytes

Always check: **Does this API encode internally?** If yes, pass raw data.

## Discovery

Found during MCP multi-image support implementation. Symptoms:
- Clipboard Ctrl+V images worked correctly
- `view` tool and MCP screenshot tool produced "Could not process image"

Difference: clipboard images use `BinaryContent → FilePart → Anthropic provider encodes once`, while tool images used `ToolResponse → fantasy encodes → but caller already encoded`.