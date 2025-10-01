# Provider Rate Limits Research

**Date:** 2025-10-01
**Purpose:** Document actual rate limits for adaptive throttling in Volley feature

## Summary

Different API providers have vastly different rate limits. The Volley scheduler needs to be aware of these to avoid hitting 429 errors and to optimize concurrent task execution.

### Quick Reference

| Provider | Free Tier RPM | Paid Tier RPM | Concurrent (RPS) | Notes |
|----------|---------------|---------------|------------------|-------|
| OpenRouter (free models) | 20 | N/A | N/A | 50-1000 req/day limit |
| OpenRouter (paid models) | N/A | No fixed RPM | Up to 500 | Balance-dependent |
| Anthropic Tier 1 | 50 | 50 | N/A | All models, $5 deposit |
| Anthropic Tier 2 | 50 | 50 | N/A | All models, $40 deposit |
| Anthropic Tier 3 | 50 | 50 | N/A | All models, $200 deposit |
| Anthropic Tier 4 | 50 | 50 | N/A | All models, $400 deposit |
| OpenAI Tier 1 | 1,000 | 1,000 | N/A | GPT-4o, automatic tier |
| OpenAI Tier 2 | 5,000 | 5,000 | N/A | GPT-4o-mini |

## OpenRouter

OpenRouter aggregates multiple AI models and has unique rate limiting based on whether you're using free or paid models.

### Free Models (model ID ends with `:free`)

**Rate Limits:**
- **20 requests per minute (RPM)**
- **Daily limits depend on account balance:**
  - Less than $10 credits: **50 requests/day**
  - $10+ credits: **1,000 requests/day**

**Policy Change (2025):** The daily free model limit was reduced from 200 to 50 requests for users without significant account balances.

### Paid Models

**Rate Limits:**
- **No fixed RPM** - uses RPS (requests per second) instead
- **Maximum concurrency:** Up to **500 RPS** (requests per second)
- **Balance-dependent:** RPS decreases dynamically as account balance depletes
- **Note:** Even with $1000+ balance, max RPS is capped at 500 (contact support for higher limits)

### Recommended Volley Settings

```go
// For free models
MaxConcurrent: 3  // Well under 20 RPM
RetryDelay:    5s // Conservative for daily limits

// For paid models with good balance
MaxConcurrent: 10-50  // Well under 500 RPS
RetryDelay:    1s     // Standard exponential backoff
```

## Anthropic Claude API

Anthropic uses a tier-based system where rate limits increase automatically based on deposits and usage.

### Tier Advancement

| Tier | Credit Purchase Required | Max Spend/Month |
|------|-------------------------|-----------------|
| Tier 1 | $5 | $100 |
| Tier 2 | $40 | $500 |
| Tier 3 | $200 | $1,000 |
| Tier 4 | $400 | $5,000 |

### Rate Limits (All Tiers)

**Claude Opus 4.x:**
- Requests: **50 RPM**
- Input Tokens: **30,000 TPM**
- Output Tokens: **8,000 TPM**

**Claude Sonnet 4.x:**
- Requests: **50 RPM**
- Input Tokens: **30,000 TPM**
- Output Tokens: **8,000 TPM**

**Claude Sonnet 3.7:**
- Requests: **50 RPM**
- Input Tokens: **20,000 TPM**
- Output Tokens: **8,000 TPM**

**Claude Haiku 3.5:**
- Requests: **50 RPM**
- Input Tokens: **50,000 TPM**
- Output Tokens: **10,000 TPM**

### Key Points

- **Token bucket algorithm:** Capacity is continuously replenished up to max limit
- **Model-specific limits:** Each model has separate rate limits
- **Same RPM across tiers:** 50 RPM regardless of tier (tiers differ in max spend)
- **Long context (200K+):** Separate limits apply when using `context-1m-2025-08-07` beta header

### Recommended Volley Settings

```go
// Conservative approach (works for all tiers)
MaxConcurrent: 3   // Well under 50 RPM (36 RPM max)
RetryDelay:    2s  // Standard backoff

// Aggressive approach (if you know tier and token usage)
MaxConcurrent: 10  // Approach 50 RPM, but watch token limits
RetryDelay:    1s  // Faster retry
```

## OpenAI API

OpenAI uses usage tiers that automatically increase based on payment history and usage patterns.

### Tier Advancement

**Automatic progression based on:**
- Successful payment history
- Account age
- Usage patterns

### Rate Limits by Model & Tier

**GPT-4o (Tier 1):**
- Requests: **1,000 RPM** (increased Sept 2025)
- Tokens: **500,000 TPM**

**GPT-4o-mini (Tier 2):**
- Requests: **5,000 RPM**
- Tokens: Higher TPM (varies)

**GPT-4 (typical):**
- Requests: **3,500 RPM**
- Tokens: **90,000 TPM**

### Key Points

- **Dual limiting:** Both RPM and TPM enforced simultaneously
- **Auto-scaling:** Tiers increase automatically with usage/payment
- **Model-specific:** Different models have different limits even in same tier

### Recommended Volley Settings

```go
// For GPT-4o (Tier 1+)
MaxConcurrent: 50  // Well under 1000 RPM
RetryDelay:    1s  // Fast retry

// For GPT-4 or lower tiers
MaxConcurrent: 20  // Conservative for 3500 RPM
RetryDelay:    2s  // Standard backoff
```

## Design Implications for Volley

### 1. Provider Detection Needed

The scheduler should detect which provider is being used and apply appropriate defaults:

```go
type ProviderRateLimits struct {
    Provider          string
    MaxRPM            int
    MaxRPS            int  // OpenRouter
    MaxTPM            int  // Token-based limits
    RecommendedConcurrent int
}
```

### 2. Default Conservative Settings

Current Volley defaults (`MaxConcurrent: 3`) are **safe for all providers**:
- OpenRouter free: 3 concurrent = ~18 RPM (< 20 limit) ✅
- Anthropic: 3 concurrent = ~36 RPM (< 50 limit) ✅
- OpenAI: 3 concurrent = minimal load on any tier ✅

### 3. Adaptive Throttling Strategy

**On 429 errors:**
1. Reduce `MaxConcurrent` by 50% (min: 1)
2. Increase retry delay by 2x
3. Track success rate over 10 requests
4. Gradually increase concurrency on sustained success

**On success:**
1. After 20 consecutive successes, increase `MaxConcurrent` by 1
2. Cap at provider-specific max (e.g., 10 for Anthropic, 50 for OpenAI)

### 4. Token-Based Rate Limiting (Future)

Some providers (Anthropic, OpenAI) have **token per minute** limits that may be hit before RPM limits. Future enhancement:

```go
type TokenRateLimiter struct {
    InputTPM  int
    OutputTPM int
    CurrentUsage TokenUsage
    WindowStart  time.Time
}
```

## Testing Recommendations

### Mock Rate Limit Tests

```go
func TestSchedulerRateLimitHandling(t *testing.T) {
    // Simulate provider returning 429 on every 5th request
    // Verify scheduler backs off and reduces concurrency
    // Verify scheduler recovers and increases concurrency
}
```

### Real API Stress Tests

```bash
# Intentionally hit rate limits to verify behavior
./bin/cliffy volley --max-concurrent 25 $(printf "task%d " {1..100})

# Expected: 429 errors → backoff → recovery → completion
```

## Recommendations for Phase 2

1. **Keep conservative defaults** - `MaxConcurrent: 3` is safe everywhere
2. **Add `--aggressive` flag** - Allows users to opt into higher concurrency
3. **Implement adaptive throttling** - Auto-adjust on 429 errors
4. **Document provider limits** - Help users choose appropriate `--max-concurrent`
5. **Monitor token usage** - Track TPM for Anthropic/OpenAI (future)

## References

- [OpenRouter Rate Limits](https://openrouter.ai/docs/api-reference/limits)
- [Anthropic Rate Limits](https://docs.claude.com/en/api/rate-limits)
- [OpenAI Rate Limits](https://platform.openai.com/docs/guides/rate-limits)

---

**Last Updated:** 2025-10-01
**Next Review:** After implementing adaptive throttling
