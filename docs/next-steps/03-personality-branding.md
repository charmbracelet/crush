# Personality & Branding

ᕕ( ᐛ )ᕗ  Cliffy's character: Quick, focused, always ready

## The Core Identity

**Cliffy is a ballboy at the US Open.**

Not the star player. Not calling the shots. Just incredibly good at:
- Being exactly where you need them
- Getting you what you need fast
- Staying out of the way
- Always ready for the next serve

**Key traits:**
- **Quick** - Fast startup, fast execution, no waiting around
- **Focused** - One task, done right, back to ready position
- **Efficient** - No unnecessary flourishes, straight to the point
- **Reliable** - Same consistent performance every time
- **Enthusiastic** - Happy to help, but not overly chatty

## Current Branding Elements

### ASCII Art Library

```go
// internal/llm/tools/ascii.go

AsciiCliffy        = "ᕕ( ᐛ )ᕗ"      // Main character
AsciiCliffyWaving  = "( ´◔ ω◔`) ノシ"  // Friendly greeting
AsciiCliffyProud   = "(ﾉ☉ヮ⚆)ﾉ ⌒*:･ﾟ✧" // Task completed!
AsciiCliffyContent = "(*・‿・)ノ⌒*:･ﾟ✧" // Satisfied

AsciiTennisBall = `
   ,odOO"bo,
 ,dOOOP'dOOOb,
,O3OP'dOO3OO33,
P",ad33O333O3Ob
?833O338333P",d
`88383838P,d38'
 `Y8888P,d88P'
   `"?8,8P"'
`
```

### Current Usage

**In git commits:**
```
ᕕ( ᐛ )ᕗ  Generated with Cliffy
Co-Authored-By: Cliffy <cliffy@ettio.com>
```

**In benchmarks:**
```
ᕕ( ᐛ )ᕗ  Cliffy vs Crush Benchmark
ᕕ( ᐛ )ᕗ  Benchmark validated
```

**In docs:**
```
# Cliffy ᕕ( ᐛ )ᕗ

Fast, focused AI coding assistant for one-off tasks.
```

## Personality Guidelines

### Voice & Tone

**Do:**
- Be direct and concise
- Use active voice
- Focus on what's done, not how you feel about it
- Show enthusiasm through speed, not words
- Use "ᕕ( ᐛ )ᕗ" as punctuation, not decoration

**Don't:**
- Use emoji (ASCII art only for terminal compatibility)
- Over-explain or add unnecessary context
- Apologize or hedge ("I think...", "Maybe...", "Sorry...")
- Use filler words or preamble
- Make jokes or puns (you're helpful, not a comedian)

### Examples

**Good:**
```
ᕕ( ᐛ )ᕗ  Task complete
ᕕ( ᐛ )ᕗ  5 files updated
ᕕ( ᐛ )ᕗ  Tests passing
```

**Bad:**
```
✨ Yay! I successfully completed your task! ✨
🎉 All done! Hope you're happy with the results! 🎉
😊 Sorry if this took a while, but I finished! 😊
```

### When to Use ASCII Art

**Always:**
- CLI startup banner
- Benchmark headers
- Completion messages
- Git attribution

**Sometimes (when relevant):**
- Error recovery ("ᕕ( ᐛ )ᕗ  Retrying...")
- Progress updates ("ᕕ( ᐛ )ᕗ  Step 2/5")
- Tool outputs (sparingly)

**Never:**
- Every line of output (too noisy)
- In error messages (stay professional)
- In JSON/structured output (breaks parsing)
- In quiet mode (obviously)

## Feature Ideas That Enhance Personality

### ✓ Good Ideas (Fast, Focused, No Extra LLM Calls)

#### 1. Startup Banner with Context
```
ᕕ( ᐛ )ᕗ  Cliffy ready
Model: grok-4-fast | Context: 2M tokens | Mode: fast
```

**Why:** Shows readiness, gives useful info, stays minimal
**Cost:** Zero - just printed text
**Latency:** ~1ms

#### 2. Progress Dots for Long Operations
```
ᕕ( ᐛ )ᕗ  Running tests...
.......
ᕕ( ᐛ )ᕗ  Tests passing
```

**Why:** Shows Cliffy is working, not stuck
**Cost:** Zero
**Latency:** None (visual only)

#### 3. Completion Summary
```
ᕕ( ᐛ )ᕗ  Task complete
  • 3 files read
  • 2 files updated
  • 0 errors
  • 4.2s total
```

**Why:** Efficient summary, ballboy reporting back
**Cost:** Zero
**Latency:** None (data already collected)

#### 4. ASCII Tennis Ball on Major Milestones
```
   ,odOO"bo,
 ,dOOOP'dOOOb,
,O3OP'dOO3OO33,
P",ad33O333O3Ob
?833O338333P",d
`88383838P,d38'
 `Y8888P,d88P'
   `"?8,8P"'

ᕕ( ᐛ )ᕗ  Version 1.0.0 released
```

**Why:** Celebrates wins without being obnoxious
**Cost:** Zero
**Latency:** None
**When:** Releases, major achievements, first-time setup

#### 5. Smart Quiet Mode
```bash
cliffy --quiet "task"         # No tool logs, just results
cliffy --silent "task"        # Absolutely nothing except final output
cliffy "task"                 # Default: tool logs visible
```

**Why:** Lets users tune verbosity for scripts vs interactive use
**Cost:** Zero
**Latency:** Actually faster (less to print)

#### 6. Exit Codes with Meaning
```bash
cliffy "task"
echo $?  # 0 = success, 1 = error, 2 = user cancelled, 3 = rate limited
```

**Why:** Scriptable, clear status without parsing output
**Cost:** Zero
**Latency:** None

#### 7. Timing Breakdown (Optional Flag)
```bash
cliffy --timings "task"

ᕕ( ᐛ )ᕗ  Task complete
  Startup:     120ms
  First token: 890ms
  Execution:   2.1s
  Total:       3.1s
```

**Why:** Performance transparency, helps users optimize
**Cost:** Zero (metrics already tracked)
**Latency:** None

### ✗ Bad Ideas (Slow, Chatty, Require Extra LLM Calls)

#### ❌ Task Summaries
```bash
# After task completes, call LLM again to summarize
"I refactored 3 functions to improve performance by..."
```

**Why bad:** Extra LLM call = 1-2s latency, costs money, not needed

#### ❌ Motivational Messages
```bash
ᕕ( ᐛ )ᕗ  Great job asking that question!
ᕕ( ᐛ )ᕗ  You're doing amazing!
```

**Why bad:** Patronizing, adds noise, not helpful

#### ❌ Auto-Generated Tips
```bash
💡 Tip: You can use --fast for quicker responses!
```

**Why bad:** Interruptive, assumes user ignorance, emoji

#### ❌ Interactive Confirmations (Default)
```bash
This will modify 5 files. Continue? [y/n]
```

**Why bad:** Breaks automation, slows one-off tasks. (Optional flag ok!)

#### ❌ Chatty Error Messages
```bash
😢 Oh no! It looks like there was a problem with the API.
Let me tell you all about what went wrong...
```

**Why bad:** Emoji, unnecessary emotion, verbosity

## Branding Opportunities

### 1. CLI Help Text

**Current:** Standard help text

**Opportunity:**
```
ᕕ( ᐛ )ᕗ  Cliffy - Fast AI coding assistant

USAGE
  cliffy [flags] "your task"

SPEED FLAGS
  --fast, --small       Use small model for quick tasks
  --smart, --large      Use large model for complex tasks
  --model <name>        Specify model directly

OUTPUT FLAGS
  --quiet               Hide tool execution logs
  --silent              Show only final output
  --show-thinking       Display LLM reasoning
  --timings             Show performance breakdown

Run 'cliffy --help' for full documentation
Ready to help ᕕ( ᐛ )ᕗ
```

**Why:** Sets personality from first interaction, stays concise

### 2. First-Time Setup

**Current:** Config error if no API key

**Opportunity:**
```
ᕕ( ᐛ )ᕗ  First time running Cliffy!

Quick setup:
  1. Get API key: https://openrouter.ai/settings/keys
  2. Set variable: export CLIFFY_OPENROUTER_API_KEY="sk-..."
  3. Run again: cliffy "list Go files"

Or see: https://cliffy.ettio.com/setup

Ready when you are ᕕ( ᐛ )ᕗ
```

**Why:** Friendly without being chatty, actionable steps

### 3. Version Info

**Current:** Not implemented

**Opportunity:**
```bash
$ cliffy --version

   ,odOO"bo,
 ,dOOOP'dOOOb,
,O3OP'dOO3OO33,
P",ad33O333O3Ob
?833O338333P",d
`88383838P,d38'
 `Y8888P,d88P'
   `"?8,8P"'

Cliffy v1.0.0
Fast AI coding assistant

https://cliffy.ettio.com
Built on Crush • Powered by OpenRouter

ᕕ( ᐛ )ᕗ  Ready to help
```

**Why:** Showcases tennis ball art, credits Crush, establishes identity

### 4. Update Notifications

**Current:** Not implemented

**Opportunity:**
```
ᕕ( ᐛ )ᕗ  Update available: v1.0.1
Run: brew upgrade cliffy
```

**Why:** Helpful without being naggy, one line only

### 5. Error Messages

**Keep professional, add Cliffy touch at end:**

```
Error: Model 'gpt-99' not found in configuration

Check ~/.config/cliffy/cliffy.json or use:
  --model grok-4-fast
  --model sonnet

ᕕ( ᐛ )ᕗ  Ready to retry
```

**Why:** Error is serious, but recovery hint shows Cliffy's ready attitude

## Implementation Guidelines

### Where Personality Lives

**High priority (user-facing):**
- CLI help text and flags
- Startup banner
- Completion messages
- Version info
- First-time setup
- Error recovery messages

**Medium priority (nice-to-have):**
- Progress indicators
- Timing breakdowns
- Update notifications

**Low priority (internal):**
- Log messages
- Debug output
- Internal comments

### Code Patterns

**Good:**
```go
// Short, direct, use constant
fmt.Fprintf(stderr, "%s  Task complete\n", tools.AsciiCliffy)
```

**Better:**
```go
// Helper for consistent formatting
func PrintReady() {
    fmt.Fprintf(stderr, "%s  Ready to help\n", tools.AsciiCliffy)
}
```

**Best:**
```go
// Centralized output formatting
type Output struct {
    quiet bool
    silent bool
}

func (o *Output) Success(msg string) {
    if !o.silent {
        fmt.Fprintf(stderr, "%s  %s\n", tools.AsciiCliffy, msg)
    }
}
```

## Testing Personality

### Visual Consistency Tests

```go
func TestAsciiArtFormatting(t *testing.T) {
    // Ensure no trailing spaces
    // Ensure consistent spacing after Cliffy
    // Ensure no emoji mixed with ASCII
}
```

### Tone Tests (Manual)

Run through common scenarios and verify:
- Is the output concise?
- Does it feel fast/efficient?
- Would it annoy you in a script?
- Does it work well piped to another tool?

### Benchmarks

Track that personality features don't add latency:
- Startup banner: <5ms
- Progress dots: <1ms per dot
- Completion summary: <10ms

## Guiding Principles

1. **Speed over words** - Show, don't tell, that Cliffy is fast
2. **ASCII over emoji** - Terminal compatibility, better appearance
3. **Concise over cute** - Ballboy efficiency, not entertainer
4. **Helpful over chatty** - Information when needed, quiet otherwise
5. **Professional with personality** - Serious about tasks, lighthearted in spirit

## Next Steps

### Phase 1: Polish Existing (Week 1)
- [ ] Improve startup banner with context
- [ ] Add completion summary
- [ ] Enhance error messages with recovery hints
- [ ] Implement --timings flag

### Phase 2: First-Time Experience (Week 2)
- [ ] Better first-run setup guide
- [ ] Version info with tennis ball
- [ ] Help text with personality
- [ ] Update checking (non-intrusive)

### Phase 3: Scriptability (Week 3)
- [ ] Meaningful exit codes
- [ ] --silent mode for piping
- [ ] JSON output format
- [ ] Environment variable docs

### Phase 4: Refinement (Ongoing)
- [ ] User feedback on tone
- [ ] A/B test verbosity levels
- [ ] Ensure personality doesn't hinder automation
- [ ] Keep benchmarking latency impact

## Success Metrics

**Personality is working when:**
- Users describe Cliffy as "snappy" or "quick"
- No complaints about chattiness in issues
- ASCII art renders correctly in 99% of terminals
- Startup time stays under 200ms
- Scripts can use Cliffy without parsing challenges

**Personality needs work when:**
- Users find messages annoying or verbose
- ASCII art breaks in common terminals
- Personality features add measurable latency
- Output interferes with piping/scripting
- Messages feel generic, could be any tool

## The Bottom Line

Cliffy's personality should feel like having a really good ballboy on a tennis court:
- You barely notice them when the game is flowing
- They're exactly where you need them, exactly when you need them
- They move fast and efficiently
- They don't talk during your serve
- But you definitely appreciate having them there

The tennis ball ASCII art is perfect because it's:
- Visually striking but terminal-safe
- On-theme with the ballboy persona
- Reserved for special moments (not overused)
- Professional yet playful

ᕕ( ᐛ )ᕗ  Quick, focused, always ready
