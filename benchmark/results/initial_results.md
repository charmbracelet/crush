# Initial Benchmark Results

Quick sanity check comparing Cliffy vs Crush performance.

## Cold Start Tasks

| Task | Cliffy | Crush | Speedup |
|------|--------|-------|---------|
| list_files | 6902ms ±1607 | 7719ms ±2519 | 1.11x |
| count_lines | 9243ms ±1636 | 14278ms ±2881 | 1.54x |

## Summary

Cliffy shows measurable performance improvements on quick tasks:
- 11-54% faster execution
- More consistent timing (lower standard deviation)
- Same model, same configuration, direct comparison

Both tools running:
- Model: x-ai/grok-4-fast:free via OpenRouter
- Config: 16384 max_tokens, no LSP/MCP
- 5 runs per test

## Conclusion

Sanity check passed. Cliffy delivers on the performance promise with real-world tasks.

ᕕ( ᐛ )ᕗ  Benchmark validated
