# API Key Validation

This document describes how `ProviderConfig.TestConnection` proves that a user's
API key authenticates against a given provider, and the conventions for adding
or changing a provider's validation behavior.

If you are touching `buildValidationProbe`, any `classify*` function, the
`openaiCompatModelsAllowlist`, or `APIKeyInputState*` in the API key dialog,
**you must also update this document in the same commit.**

---

## The problem this layer solves

When a user enters an API key in the onboarding dialog, Crush tells them
whether the key is valid before saving it. The obvious implementation — make
an HTTP request to the provider and see if it succeeds — turns out to be
wrong in a specific and silent way.

Historically, Crush validated keys by calling `GET /models` and treating a
`200 OK` response as proof of authentication. This works for the native
OpenAI and Anthropic APIs, where `/models` is auth-gated. It does **not**
work for most OpenAI-compatible gateways, because `/models` on those
services is deliberately public: it powers SDK catalogs, docs sites, and
model-picker UIs that render without requiring a signup. A public `/models`
endpoint returns `200` to every caller — valid key, invalid key, no key at
all — so the response proves nothing about the caller's credentials.

The consequence was that for roughly ten providers (AiHubMix, Avian,
Cortecs, HuggingFace Router, io.net, OpenCode Go, OpenCode Zen, QiniuCloud,
Synthetic, Venice), Crush's "Key validated" message didn't actually reflect
whether the key would work. Any string the user typed — a typo, a
copy-paste from the wrong field, the literal word `test` — was reported as
a valid key. The failure surfaced later, when the user actually tried to
run the model and got an authentication error from the provider. A
separate bug made MiniMax and MiniMax China return "validated"
unconditionally, regardless of endpoint behavior.

The fix is to stop assuming `/models` proves authentication. For each
provider we either:

1. Find a different endpoint that actually gates on auth (typically
   account-scoped data like rate limits or credits).
2. Use the auth-gated `/chat/completions` endpoint with a deliberately
   malformed body, so we can tell auth-failure apart from schema-failure
   without running inference.
3. Admit we cannot reliably verify the key and say so in the UI
   ("saved, not verified") rather than faking success.

This is per-provider policy by necessity, because providers disagree about
which endpoints authenticate, which status codes they return for bad keys,
and whether their gateways authenticate before or after validating the
request body. Managing that per-provider-ness — without silently
regressing back into "assume `/models` proves auth" — is what the rest of
this document exists to do.

---

## Contract

`TestConnection` returns one of three things:

| Return value               | Meaning                                                 | UI state                                             |
| -------------------------- | ------------------------------------------------------- | ---------------------------------------------------- |
| `nil`                      | Authentication proven.                                  | `APIKeyInputStateVerified` ("validated")             |
| `ErrValidationUnsupported` | No deterministic probe exists. Key saved, not verified. | `APIKeyInputStateUnverified` ("saved, not verified") |
| Any other non-nil `error`  | Probe ran and the server rejected the key.              | `APIKeyInputStateError` ("invalid")                  |

"Saved, not verified" is a **first-class outcome**, not a failure. It is
strictly preferable to a false-positive "validated" — the original bug that
motivated this whole machinery was the system telling users bad keys were valid
because `/models` returned `200` to any caller.

---

## Why not just hit `/models`?

Many OpenAI-compatible gateways intentionally expose a public `/models` endpoint
so SDKs, docs sites, and model pickers can render a catalog without requiring
signup. On those providers, `GET /models` returns `200 OK` regardless of the key
— so `200` proves nothing about authentication.

The fix is to pick a probe that the server's response **depends on the key**.
That means either:

1. Hit an endpoint the provider actually gates on auth (typically account-scoped
   data like rate limits or credits).
2. Hit an auth-gated endpoint (`/chat/completions`) with an intentionally broken
   payload, so the server authenticates the caller before rejecting the body —
   without actually running inference.
3. If neither is available, return `ErrValidationUnsupported` and let the UI
   fall back to "saved, not verified."

---

## Classifiers

The four `classify*` functions in `config.go` cover every probe currently in
use. Keep this set small — prefer "use an existing classifier" over "add a new
one."

| Classifier                    | Valid → `nil`                             | Invalid → error     | Anything else              |
| ----------------------------- | ----------------------------------------- | ------------------- | -------------------------- |
| `classifyAuthGated`           | `200`                                     | `401`, `403`        | `ErrValidationUnsupported` |
| `classifyOpenAIChatMalformed` | `400`, `422` (auth passed, body rejected) | `401`, `403`        | `ErrValidationUnsupported` |
| `classifyGoogleModels`        | `200`                                     | `400`, `401`, `403` | `ErrValidationUnsupported` |
| `classifyZAIModels`           | anything except `401`                     | `401` only          | (no unsupported bucket)    |

Transient statuses (`5xx`, `429`, `402`, unexpected `200` on the chat probe)
collapse into `ErrValidationUnsupported` on purpose — a flaky gateway should not
surface as "your key is bad."

---

## Probes by provider

### Auth-gated endpoint (GET)

| Provider ID                | Endpoint                             | Auth                              | Classifier             |
| -------------------------- | ------------------------------------ | --------------------------------- | ---------------------- |
| `openai`                   | `GET {base}/models`                  | `Authorization: Bearer`           | `classifyAuthGated`    |
| `openrouter`               | `GET {base}/credits`                 | `Authorization: Bearer`           | `classifyAuthGated`    |
| `anthropic`                | `GET {base}/models`                  | `x-api-key` + `anthropic-version` | `classifyAuthGated`    |
| `kimi-coding`              | `GET {base}/v1/models`               | `x-api-key` + `anthropic-version` | `classifyAuthGated`    |
| `gemini`                   | `GET {base}/v1beta/models?key=<key>` | key in query                      | `classifyGoogleModels` |
| `venice`                   | `GET {base}/api_keys/rate_limits`    | `Authorization: Bearer`           | `classifyAuthGated`    |
| `minimax`, `minimax-china` | `GET {base}/v1/models`               | `x-api-key` + `anthropic-version` | `classifyAuthGated`    |

### OpenAI-compat `/models` allowlist

For `openai-compat` providers only. Entry in this allowlist means the provider's
`/models` endpoint has been **empirically confirmed** to return `401` on a bad
key:

| Provider ID                                                                         | Probe                                                                                                           |
| ----------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `deepseek`, `groq`, `xai`, `zhipu`, `zhipu-coding`, `cerebras`, `nebius`, `copilot` | `GET {base}/models` + `Authorization: Bearer`, `classifyAuthGated`                                              |
| `zai`                                                                               | Same probe, but uses `classifyZAIModels` (only `401` is invalid; valid keys return assorted non-`200` statuses) |

### Malformed-body chat probe

Used when the provider's `/models` is public but `/chat/completions` is
auth-gated. Sends `{"__crush_probe__": true}` (missing required fields) so the
gateway authenticates the caller before rejecting the schema. No tokens
consumed.

| Provider IDs                                                                                                     |
| ---------------------------------------------------------------------------------------------------------------- |
| `aihubmix`, `avian`, `cortecs`, `huggingface`, `ionet`, `opencode-go`, `opencode-zen`, `qiniucloud`, `synthetic` |

All use `POST {base}/chat/completions` + `Authorization: Bearer` +
`classifyOpenAIChatMalformed`.

### Prefix check (no network)

| Provider ID | Rule                                                                                                                                        |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `bedrock`   | Key must start with `ABSK`. Weak signal — Bedrock's `/foundation-models` endpoint is region-specific, so we fall back to format validation. |
| `vercel`    | Key must start with `vck_`. Vercel's `/models` does not gate on auth.                                                                       |

### Explicitly unverified

| Provider ID                          | Why                                                                                                                                  |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------ |
| `chutes`                             | Observed ambiguous response (`429`) on unauthenticated probe path; classifier cannot reliably distinguish bad-key from rate-limited. |
| `neuralwatt`                         | Observed classifier ambiguity on malformed-body probe.                                                                               |
| Any unknown `openai-compat` provider | Default fallback. We don't assume `/models` is auth-gated for providers we haven't tested.                                           |

---

## Adding a new provider

Follow this checklist in order:

### 1. Identify the provider's `Type`

Check the `type` field in the catwalk provider definition:

- `openai`, `anthropic`, `google`, `openrouter`, `bedrock`, `vercel`, `azure`,
  `vertexai` — type-based default in `buildValidationProbe` already covers you.
  Stop here unless the provider has quirks.
- `openai-compat` — keep going.

### 2. Test the provider's `/models` endpoint with a bad key

```sh
curl -i -H "Authorization: Bearer definitely-not-a-real-key" \
  https://<provider>/v1/models
```

| Response                    | Action                                                                                              |
| --------------------------- | --------------------------------------------------------------------------------------------------- |
| `401` or `403`              | `/models` is auth-gated. Add the provider ID to `openaiCompatModelsAllowlist` in `config.go`. Done. |
| `200`                       | `/models` is public. Go to step 3.                                                                  |
| `429`, `5xx`, anything else | Ambiguous. Mark unsupported (step 5).                                                               |

### 3. Test `/chat/completions` with a bad key and malformed body

```sh
curl -i -X POST \
  -H "Authorization: Bearer definitely-not-a-real-key" \
  -H "Content-Type: application/json" \
  -d '{"__crush_probe__":true}' \
  https://<provider>/v1/chat/completions
```

| Response       | Action                                                                                                 |
| -------------- | ------------------------------------------------------------------------------------------------------ |
| `401` or `403` | Auth gate works. Add the provider ID to the chat-probe `case` in `buildValidationProbe`. Go to step 4. |
| `400` or `422` | Gateway validates schema before auth. `/chat/completions` is not usable as a probe. Go to step 5.      |
| Anything else  | Mark unsupported (step 5).                                                                             |

### 4. Confirm the good-key behavior

Using a real key, repeat the `/chat/completions` probe with the malformed body.
Expect `400` or `422`. If you get `200`, the gateway is not validating the body
and the probe cannot distinguish valid from invalid keys — go to step 5 instead.

### 5. Mark the provider unsupported

Add the provider ID to the
`InferenceProviderChutes, InferenceProviderNeuralwatt` case in
`buildValidationProbe` (or extend it). The UI will show "saved (not verified)"
and the user can still use the provider.

### 6. Update this document and the tests

- Add the provider to the appropriate table in the "Probes by provider" section
  above.
- Add a test case to `TestTestConnectionOpenAICompatProviderAudit` in
  `config_validate_test.go` (for probe-based providers).
- Add the provider to the appropriate list in
  `TestTestConnectionPublicModelsAuthGatedChatRegression` (for chat-probe
  providers) or `TestTestConnectionOpenAICompatAllowlistUsesModelsProbe` (for
  allowlist providers).

---

## Decision log

### Why allowlist, not default-allow, for `openai-compat` `/models`?

The regression that motivated this document was exactly "default-allow." Commit
`7d14abb9` (2025-10-20) expanded the `/models` check from `TypeOpenAI` to all
`openai-compat` providers, silently turning every gateway with a public model
catalog into a false-positive validator. A default-deny allowlist makes new
providers opt in explicitly — the cost is one line per provider; the benefit is
no more silent regressions of this shape.

### Why is ZAI special?

ZAI's `/models` endpoint is authoritative about bad keys (always `401`) but
noisy about valid keys (returns assorted non-`200` statuses, seemingly depending
on backend state). Folding it into `classifyAuthGated` would regress valid-key
detection, since everything except `200` would be classified as
`ErrValidationUnsupported`. The ZAI classifier treats `401` as invalid and
everything else as valid.

This is fragile — if ZAI ever changes their endpoint so `401` is no longer
specific to auth, the classifier becomes wrong. It is documented here so the
next person to touch it knows why it exists.

### Why is MiniMax in the provider-ID switch and not the `TypeAnthropic` branch?

MiniMax's base URL is `https://api.minimax.io/anthropic`, which means
`{base}/models` resolves to `/anthropic/models` — a 404. The correct endpoint is
`{base}/v1/models`. The `TypeAnthropic` default assumes the base URL already
ends in `/v1`, which holds for native Anthropic but not for MiniMax. Rather than
special-case the type branch, MiniMax gets its own explicit probe entry.

A previous bug (commit `cce8edf9`, 2026-04-23) removed MiniMax's validation
entirely by returning `nil` unconditionally. Don't reintroduce that.

### Why does Google need its own classifier?

Google returns `400 INVALID_ARGUMENT` for unknown API keys, not `401`. If
`classifyAuthGated` were used, bad Google keys would produce `400` →
`ErrValidationUnsupported` → "saved, not verified" — downgrading a real auth
failure into a soft warning. `classifyGoogleModels` adds `400` to the invalid
bucket specifically for this case.

### Why not push all this into catwalk?

Probe metadata (method, path, classifier kind) is provider-identity data and
arguably belongs alongside the rest of the catwalk provider definition. The
classifier functions and UI mapping are crush-specific behavior and should stay
here.

This refactor hasn't been done yet because the set of providers and classifiers
is still small enough that the indirection cost (catwalk schema change, version
bump, fallback for older catwalks) outweighs the benefit. If the allowlist or
classifier set grows significantly, revisit this decision.

---

## Regression history

| Date       | Commit     | Effect                                                                                                                                                              |
| ---------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 2025-10-20 | `7d14abb9` | Expanded `TestConnection` from `openai` to `openai-compat` with generic `/models` probe. Silently false-validated bad keys for ~10 providers with public `/models`. |
| 2026-04-23 | `cce8edf9` | Removed MiniMax's format-prefix guard, causing `TestConnection` to return `nil` unconditionally for MiniMax / MiniMax China.                                        |
| (current)  | (this PR)  | Replaced generic `/models` with per-provider probes, added `ErrValidationUnsupported` sentinel, wired UI "saved (not verified)" state.                              |

Any future regression in this layer should be recorded here.
