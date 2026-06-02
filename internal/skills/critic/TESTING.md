# Testing Guide — Self-Critic Skill

## Philosophy

Every component in the critic skill must be testable without network calls or
LLM dependencies. We achieve this through:

- **Interface boundaries**: `CheckpointEmitter` is a function type; tests inject
  deterministic implementations.
- **Mock agents**: `mockAgent` implements `agent.SessionAgent` for middleware
  tests.
- **Parser fallbacks**: All JSON repair strategies are tested with static strings.

## Running Tests

```bash
# All critic skill tests
go test ./internal/skills/critic/... -v

# With race detector
go test ./internal/skills/critic/... -race

# Specific test
go test ./internal/skills/critic/... -run TestParseFeedback -v
```

## Test Matrix

### Parser Tests (`parser_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestParseFeedback_DirectJSON` | Happy path: raw JSON unmarshals cleanly. |
| `TestParseFeedback_MarkdownFence` | Extracts JSON from ` ```json ... ``` ` fences. |
| `TestParseFeedback_TrailingCommaRepair` | Repairs invalid trailing commas before `}` or `]`. |
| `TestParseFeedback_Empty` | Returns error on empty input. |
| `TestParseFeedback_Invalid` | Returns error when no strategy succeeds. |
| `TestParseFeedback_LargeInputTruncatedInError` | Error messages truncate raw output to 500 bytes. |
| `TestParseFeedback_NestedFence` | Ignores surrounding text, extracts fenced JSON. |
| `TestTruncate` | Truncation helper respects limit and appends suffix. |

**When to add a parser test**: Any time a new fallback strategy is added or a
new malformed JSON pattern is discovered in production.

### Config Tests (`config_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestNewCriticSkillConfig_NilConfig` | All defaults returned when cfg is nil. |
| `TestNewCriticSkillConfig_NilOptions` | Defaults when Options is nil. |
| `TestNewCriticSkillConfig_NilCritic` | Defaults when Critic sub-config is nil. |
| `TestNewCriticSkillConfig_Overrides` | Respects every field from `config.CriticConfig`. |
| `TestNewCriticSkillConfig_DefaultsForZeroValues` | Replaces zero-values with sensible defaults. |
| `TestNewCriticSkillConfig_GlobalDisable` | `CRUSH_CRITIC_GLOBAL_DISABLE=1` forces disabled. |

**When to add a config test**: New fields added to `CriticConfig` or `CriticSkillConfig`.

### Checkpoint Tests (`checkpoint_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestGate_NilFeedback` | Nil feedback defaults to `GateRevise` (fail-closed). |
| `TestGate_Approve` | `"approve"` → `GateApprove`. |
| `TestGate_Revise` | `"revise"` → `GateRevise`. |
| `TestGate_Halt` | `"halt"` → `GateHalt`. |
| `TestGate_UnknownVerdict` | Unknown verdicts default to `GateApprove`. |
| `TestCheckpointType_Constants` | String values of checkpoint types are stable. |

**When to add a checkpoint test**: New verdict types, new gate logic (e.g.
threshold-based decisions), or new checkpoint fields.

### Middleware Tests (`middleware_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestNewMiddleware_NilPrimary` | Returns nil instead of panicking. |
| `TestMiddleware_Run_DelegatesWhenDisabled` | Bypasses critic when config is disabled. |
| `TestMiddleware_Run_DelegatesWhenNoCriticService` | Bypasses when critic service is nil. |
| `TestMiddleware_Run_DelegatesWhenCriticDisabledPerSession` | Bypasses when `CriticEnabled=false`. |
| `TestMiddleware_Run_NoChangesNoReview` | Skips review when no files changed. |
| `TestMiddleware_Run_HaltRollsBack` | Halt triggers snapshot rollback. |
| `TestMiddleware_Run_ApproveClearsSnapshot` | Approve releases snapshot memory. |
| `TestMiddleware_Run_RevisionInjectsMessage` | Revise injects feedback into conversation. |
| `TestMiddleware_Run_MaxIterationsExceeded` | Returns error when max iterations reached. |
| `TestMiddleware_Run_SkipRevisionWhenNotAutoApproved` | Low-confidence revise is skipped. |
| `TestMiddleware_InterfaceCompliance` | All delegate methods are callable without panic. |

**When to add a middleware test**: New interception logic in `Run()`, lifecycle
methods, or concurrency behavior.

### Service Tests (`service_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestNewCriticService` | Service creation and `Enabled()` reporting. |
| `TestCriticService_Enabled_False` | Disabled config returns false. |
| `TestCriticService_Review_NoEmitter` | Returns error when emitter is unset. |
| `TestCriticService_Review_WithEmitter` | Delegates to emitter and returns feedback. |
| `TestCriticService_Review_CacheHit` | Cache hit bypasses emitter. |
| `TestCriticService_Publish` | Pub/sub events are emitted. |
| `TestCriticService_ShouldAutoApprove` | Auto-approve logic respects threshold. |
| `TestCriticService_PublishLoopCompleted` | Loop-completed event is emitted. |

**When to add a service test**: Timeout logic, caching, retry loops, pub/sub
emission, or LSP diagnostic enrichment.

## Mocking Patterns

### CheckpointEmitter

```go
emitter := func(ctx context.Context, cp Checkpoint) (*CriticFeedback, error) {
    return &CriticFeedback{Verdict: "approve", Confidence: 0.99}, nil
}
```

### SessionAgent

Use `mockAgent` from `middleware_test.go` or define your own minimal struct
implementing `agent.SessionAgent`.

### Diff Tests (`diff_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestComputeDiff_EmptyPaths` | Returns empty for nil paths. |
| `TestLibraryDiff` | Unified diff contains `-old` and `+new` lines. |
| `TestLibraryDiff_BinaryFile` | Binary files produce "Binary file ... differs". |
| `TestLibraryDiff_Truncation` | Respects `maxSize` and appends truncation marker. |
| `TestGitDiff_SkipsInNonGitRepo` | Returns error outside a git repo. |
| `TestGitDiff_InsideGitRepo` | Produces diff inside a git repo. |

### Breaker Tests (`breaker_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestBreakerRegistry_SuccessCloses` | Success resets breaker to closed. |
| `TestBreakerRegistry_OpensAfterFailures` | 5 retryable failures open the circuit. |
| `TestBreakerRegistry_NonRetryableDoesNotOpen` | Non-retryable errors don't open circuit. |
| `TestBreakerRegistry_HalfOpenAfterCooldown` | Cooldown transitions to half-open. |
| `TestBreakerRegistry_SuccessResets` | Success in half-open resets to closed. |
| `TestBreakerRegistry_Cleanup` | Inactive breakers are garbage-collected. |

### Prompt Tests (`prompt_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestBuildCriticPrompt_DefaultTemplate` | Default template renders all sections. |
| `TestBuildCriticPrompt_NoDiffNoDiags` | Omits empty sections. |
| `TestBuildCriticPrompt_WithProjectContext` | Loads `AGENTS.md` from working directory. |
| `TestLoadProjectContext_Truncation` | Caps output at 4 KB with marker. |
| `TestLoadTemplate_Fallback` | Falls back to embedded template. |

### Cache Tests (`cache_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestFeedbackCache_HitMiss` | Put/Get round-trip works. |
| `TestFeedbackCache_DifferentKeys` | Different checkpoints get different cache entries. |
| `TestFeedbackCache_Eviction` | LRU evicts oldest entry. |
| `TestFeedbackCache_Stats` | Hit/miss counters are accurate. |

### Store Tests (`store_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestStore_CreateAndGet` | Create and retrieve by message ID. |
| `TestStore_ListBySession` | List all reviews for a session. |
| `TestStore_GetByMessageID_NotFound` | Returns error for missing review. |
| `TestStore_Prune` | Deletes old reviews by cutoff. |
| `TestStore_Prune_NoDB` | Returns error when DB is unset. |

### Skill Config Tests (`skillconfig_test.go`)

| Test | What it verifies |
|------|-----------------|
| `TestLoadSkillConfig_NoFile` | Returns base config unchanged. |
| `TestLoadSkillConfig_MergesValues` | Overrides fields from `config.json`. |
| `TestLoadSkillConfig_InvalidJSON` | Returns error for invalid JSON. |

## Integration Test Plan (Future)

Once the critic is wired into the coordinator, add VCR-based tests in
`internal/agent/critic_test.go` (or similar) that:

1. Record real LLM critic interactions.
2. Assert that known-bad diffs trigger `revise` or `halt`.
3. Assert that known-good diffs trigger `approve`.

Run `task test:record` to refresh cassettes after prompt changes.
