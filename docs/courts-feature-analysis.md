Comprehensive Analysis: Courts Feature for Cliffy

  1. How Context Management Works (OpenRouter & Providers)

  Key Finding: Context is 100% client-side managed. There is NO server-side session state.

  Looking at internal/llm/provider/openai.go:68-150, every API call to OpenRouter/OpenAI requires:
  func (o *openaiClient) StreamResponse(ctx context.Context, messages []message.Message, tools
  []tools.BaseTool)

  The entire conversation history (messages []message.Message) must be sent with every single
  request. The API is stateless - it doesn't remember anything between calls.

  This means:
  - Each Cliffy invocation sends: System prompt + All previous messages + New user message
  - OpenRouter doesn't track sessions - we have to
  - Token costs scale linearly with conversation length (full history each time)
  - We control 100% of the "knobs" because state is local

  ---
  2. What Crush Had (That We Removed)

  From analyzing /tmp/crush/internal/:

  Session Infrastructure (internal/session/session.go)

  type Session struct {
      ID               string
      ParentSessionID  string        // For nested task sessions
      Title            string          // Auto-generated
      MessageCount     int64
      PromptTokens     int64
      CompletionTokens int64
      SummaryMessageID string         // For long conversations
      Cost             float64
      CreatedAt        int64
      UpdatedAt        int64
  }

  SQLite Storage (internal/db/migrations/20250424200609_initial.sql)

  - sessions table - metadata
  - messages table - full conversation history (JSON parts)
  - files table - file version tracking per session
  - Triggers for auto-updating counts, timestamps
  - Foreign keys with CASCADE delete

  Message Service (we replaced with in-memory message.Store)

  - Full CRUD operations backed by SQLite
  - Pub/sub events for TUI updates
  - Parent/child session relationships
  - Conversation summarization support

  What Cliffy Currently Has

  - internal/message/store.go:14-24 - In-memory map only
  - internal/runner/runner.go:86 - Generates ephemeral UUID each run
  - Everything discarded when process exits

  ---
  3. How Competitors Handle Sessions

  Simon Willison's LLM (from source code)

  # llm/cli.py
  def load_conversation(conversation_id=None):
      # Always uses SQLite (~/.config/io.datasette.llm/logs.db)
      if not conversation_id:
          return most_recent_conversation()
      return get_conversation_by_id(conversation_id)

  # Usage
  $ llm "first message"           # Creates new conversation
  $ llm --continue "follow up"    # Loads last conversation
  $ llm --cid abc123 "message"    # Specific conversation

  Key insights:
  - Persistence by default - logging is core philosophy
  - SQLite stores ALL interactions forever
  - Conversations auto-continued (most recent)
  - Explicit IDs for specific threads

  GitHub Copilot (from research)

  - IDE sessions only - no CLI persistence
  - Context maintained within IDE session lifetime
  - Multi-turn in chat panel, but resets on IDE restart
  - Recent issue: Context loss after summarization (users complained)

  Google Gemini Code Assist (from research)

  - Persistent across IDE restarts (added 2025)
  - Multiple concurrent chat sessions
  - Context drawer UI shows active files
  - Agent mode state persists between sessions

  ---
  4. All the Knobs: Court Design Variables

  Here's every decision point for implementing courts:

  A. Storage Strategy

  | Knob             | Options                                            | Cliffy Recommendation
                           |
  |------------------|----------------------------------------------------|-------------------------
  -------------------------|
  | Storage backend  | SQLite / JSON files / Filesystem                   | JSON files - simpler, no
   migrations              |
  | Storage location | ~/.config/cliffy/courts/ vs ~/.local/share/cliffy/ | ~/.config/cliffy/courts/
   - keeps config adjacent |
  | File structure   | One file per court vs directory per court          | One JSON file - easier
  to manage                 |
  | What to persist  | Full messages vs summary vs compressed             | Full messages initially
                           |

  B. Lifecycle Management

  | Knob              | Options                                      | Cliffy Recommendation
               |
  |-------------------|----------------------------------------------|------------------------------
  -------------|
  | TTL default       | 15min / 30min / 1hr / 24hr / never           | 30 minutes - balance
  usability vs clutter |
  | TTL configurable? | Yes / No                                     | Yes - --ttl 1h flag
               |
  | Cleanup trigger   | On startup / Background daemon / Manual only | On startup - no daemon
  complexity         |
  | Max courts        | Unlimited / Hard limit (e.g., 50)            | Unlimited with warning if >50
               |
  | Max age           | 7 days / 30 days / never                     | 7 days absolute max
  regardless of TTL     |

  C. Naming & Discovery

  | Knob            | Options                                    | Cliffy Recommendation
            |
  |-----------------|--------------------------------------------|----------------------------------
  ----------|
  | Court ID format | User-defined / Auto UUID / Auto named      | User-defined required - explicit
  is better |
  | ID validation   | Alphanumeric / Any string / File-safe only | File-safe - [a-zA-Z0-9_-]+
            |
  | Auto-resume     | Last court / Never / Configurable          | Never - always explicit
            |
  | List display    | Table / JSON / Simple list                 | Table with last accessed time
            |

  D. Context Management

  | Knob           | Options                                       | Cliffy Recommendation
            |
  |----------------|-----------------------------------------------|--------------------------------
  ----------|
  | Message limit  | Unlimited / Hard cap (e.g., 50) / Token-based | Token-based - warn at 100k
  tokens        |
  | Summarization  | Auto / On-demand / Never                      | On-demand - cliffy --court X
  --summarize |
  | Token tracking | Per-message / Session total / None            | Session total - show on list
            |
  | Prompt caching | Anthropic's prompt cache / None               | Use if available - huge cost
  savings     |

  E. Tool & File State

  | Knob              | Options                                      | Cliffy Recommendation
      |
  |-------------------|----------------------------------------------|------------------------------
  ----|
  | Working directory | Save CWD / Always use current / Configurable | Save CWD, warn if different
      |
  | Shell state       | Persist persistent shell / Fresh each time   | Fresh - too complex otherwise
      |
  | File versions     | Track like Crush / Ignore                    | Ignore initially - can add
  later |

  F. Safety & Recovery

  | Knob                | Options                                               | Cliffy
  Recommendation                      |
  |---------------------|-------------------------------------------------------|-------------------
  -------------------------|
  | Corruption handling | Fail / Backup & recreate / Ignore                     | Backup & recreate
  - don't block user       |
  | Concurrent access   | Lock file / Allow (last write wins) / Detect conflict | Lock file -
  prevent corruption             |
  | Export format       | JSON / Markdown / Both                                | Both - JSON for
  automation, MD for reading |

  ---
  5. Architectural Patterns We Could Use

  Pattern 1: Minimal (Pure Persistence)

  ~/.config/cliffy/courts/
    ├── debug.json        # Just message history
    ├── refactor.json
    └── .metadata         # Last accessed times for cleanup

  Pros: Dead simple, no DB overhead
  Cons: No rich querying, manual cleanup only

  Pattern 2: LLM-style (SQLite Everywhere)

  ~/.config/cliffy/cliffy.db
    ├── courts table
    ├── messages table
    └── metadata table

  Pros: Rich queries, automatic cleanup, transactions
  Cons: Migrations, DB overhead, back to what we removed

  Pattern 3: Hybrid (JSON + Index)

  ~/.config/cliffy/
    ├── courts/
    │   ├── debug.json
    │   └── refactor.json
    └── courts.index       # Lightweight metadata only

  Pros: Balance of simplicity and features
  Cons: Two sources of truth to sync

  Recommendation: Start with Pattern 1, evolve to Pattern 3 if needed.

  ---
  6. Key Questions to Answer Before Coding

  1. Philosophy check: Does this violate "zero persistence" branding?
    - Answer: Only if it's default behavior. Opt-in is fine.
  2. User confusion: Will two modes (ephemeral vs courts) confuse users?
    - Answer: Risk is real. Mitigation: Clear docs, explicit flags, good errors.
  3. Maintenance burden: Are we re-adding complexity we intentionally removed?
    - Answer: Some, but bounded. No DB, no pubsub, no TUI integration.
  4. Alternative approaches: Could we achieve the goal differently?
    - Option A: External wrapper script that manages context
    - Option B: Pipe-based chaining: cliffy "task" | cliffy --continue
    - Option C: Integration with existing tools (tmux, screen, etc.)
  5. Killer feature or scope creep? Is this what users actually need?
    - Answer: Unknown. Needs validation with users/issues.

  ---
  7. Recommended Implementation Phases

  Phase 0: Validation (Before writing code)

  - Create GitHub discussion/issue
  - Gather user feedback on need
  - Prototype JSON schema
  - Document UX with examples

  Phase 1: Basic Persistence (MVP - 2-3 days)

  - --court <id> flag
  - JSON file storage (messages only)
  - Simple load/save on startup/exit
  - No TTL, no cleanup

  Phase 2: Lifecycle Management (Week 2)

  - --ttl flag support
  - Cleanup on startup (age-based)
  - cliffy courts list
  - cliffy courts clean

  Phase 3: Advanced Features (Week 3+)

  - Token tracking & warnings
  - Working directory validation
  - Export to markdown
  - Summarization support

  Phase 4: Polish (Week 4+)

  - Lock file for concurrent safety
  - Better error messages
  - Documentation & examples
  - Performance testing

  ---
  8. Concrete Example: What It Looks Like

  # First interaction - create court
  $ cliffy --court debug "why is auth.go failing?"
  [Cliffy analyzes and responds]
  Court 'debug' created. TTL: 30m

  # Later, continue (under 30min)
  $ cliffy --court debug "now check the database calls"
  [Loads previous context, continues conversation]

  # List active courts
  $ cliffy courts list
  ID         Messages  Tokens   Last Used        TTL
  debug      4         12.5k    2m ago          28m
  refactor   12        45.2k    1h ago          expired

  # Extend TTL if still working
  $ cliffy --court debug --ttl 1h "keep going..."

  # Clean up manually
  $ cliffy courts clean
  Removed 1 expired court (refactor)

  # Export conversation
  $ cliffy courts export debug > debug-session.md

  ---
  9. The Big Decision Matrix

  | Approach                  | Complexity | User Value | Aligns with Cliffy? |
  |---------------------------|------------|------------|---------------------|
  | Do nothing                | ✅ Minimal  | ❌ None     | ✅ Perfect           |
  | Courts (opt-in)           | ⚠️ Medium  | ✅ High     | ⚠️ Acceptable       |
  | Full session system       | ❌ High     | ⚠️ Mixed   | ❌ No - use Crush    |
  | External tool integration | ✅ Low      | ⚠️ Limited | ✅ Good              |

  ---
  10. My Recommendation

  Proceed with courts, but:

  1. Start super minimal - Phase 1 only, no TTL complexity
  2. Make it explicitly opt-in - No auto-resume, no defaults
  3. Document the tradeoffs - Costs tokens, adds state, etc.
  4. Validate with users first - GitHub issue + prototype
  5. Keep escape hatch - Easy to disable/remove if it's not valuable

  Alternative to consider: Before building courts, try a wrapper script approach to see if external
  tooling is "good enough":

  # ~/.local/bin/cliffy-continue
  #!/bin/bash
  CONTEXT_FILE="/tmp/cliffy-context-$$"
  trap "rm -f $CONTEXT_FILE" EXIT

  if [ -f "$CONTEXT_FILE" ]; then
      cliffy "$(cat $CONTEXT_FILE) ... $@"
  else
      cliffy "$@"
  fi
  echo "$@" >> $CONTEXT_FILE

  This tests the UX without committing to the architecture.
