# Research Brief: CLI Agent Tool Interoperability

**Status: Research (Complete)**

This research identified which CLI tools can participate in our dispatch system. Results are summarized in [future/multi-agent-orchestration.md](../future/multi-agent-orchestration.md).

---

## Objective

We are building a **platform-agnostic orchestration system** where multiple CLI AI agents can:

1. Register themselves as available agents
2. Receive tasks from a central dispatch queue
3. Communicate results back to requesting agents
4. Share a common session/context format

## What We Need to Know

For each CLI agent tool, gather the following:

### 1. Invocation Interface

- **How is it invoked from command line?**
  - Example command syntax
  - Can it read prompts from stdin?
  - Can it write output to a specific file?
  - Does it support `--session` or `--context` flags for continuity?

### 2. Session/Context Persistence

- **How does it store conversation history?**
  - File format (JSON, SQLite, custom binary, markdown)?
  - Location (project directory, global config, temp)?
  - Can an external tool resume a session by ID or file path?
  - Is the format documented or reverse-engineerable?

### 3. Tool/Function Calling

- **Does it support structured tool use?**
  - Can it call custom tools/functions?
  - How are tools defined (JSON schema, code, config file)?
  - Can tools be added at runtime or only at startup?
  - Does it support "skills" or "extensions"?

### 4. Non-Interactive Mode

- **Can it run without a TUI/REPL?**
  - Does it have a `--non-interactive` or `--headless` mode?
  - Can it be run in a pipeline?
  - How does it handle permissions in non-interactive mode?
  - Does it accept all input upfront or can it be streamed?

### 5. Output Format

- **What format does it output?**
  - Plain text only?
  - JSON output flag (e.g., `--json`)?
  - Streaming output vs batch?
  - Does it include metadata (token usage, model, etc.)?

### 6. External Control

- **Can it be controlled by other processes?**
  - Does it have an API (HTTP, Unix socket)?
  - Can it be configured via environment variables?
  - Does it support hooks (pre/post task)?
  - Can config be loaded from a file at runtime?

### 7. Agent Identity

- **Does it have a concept of "agent identity"?**
  - Can you give it a custom name/role?
  - Can you set a custom system prompt?
  - Can multiple independent instances run simultaneously?

### 8. Inter-Agent Communication

- **Does it support standard protocols?**
- **Does it have any native agent-to-agent communication?**
- **Can it be configured to poll for tasks?**

### 9. Compatibility Score

- Rate 1-10 how well this tool could integrate with our dispatch system
- What would be required to make it compatible?

---

## Output Format

For each tool, provide a structured report:

```yaml
tool_name: example-agent
version: 1.x
repo: https://github.com/example/agent

invocation:
  cli_syntax: 'agent [flags] "prompt"'
  stdin_support: yes/no
  output_file_flag: "--output"
  session_flag: "--resume <session-id>"

session_persistence:
  format: json|sqlite|custom
  location: "path/to/sessions"
  external_resume: yes/no
  format_documented: yes/no

tool_calling:
  supports_tools: yes/no
  tool_definition: schema|mcp|config
  runtime_tools: yes/no
  extension_mechanism: description

non_interactive:
  supported: yes/no
  flag: "--no-interactive"
  permission_mode: description

output:
  format: text|json
  json_output: flag or none
  streaming: yes/no

external_control:
  api: yes/no
  env_config: yes/no
  hooks: yes/no
  runtime_config: yes/no

agent_identity:
  custom_name: yes/no
  custom_system_prompt: yes/no
  independent_instances: yes/no

inter_agent_comm:
  protocol_support: description
  other: description

compatibility_score: 1-10
notes: "Brief assessment"
```

---

## Success Criteria

We can integrate a CLI tool if it supports:

| Requirement | Minimum | Preferred |
|-------------|---------|-----------|
| Non-interactive mode | Yes | Yes |
| Session persistence | Read-only access | Full read/write |
| Tool definitions | Static config | Runtime registration |
| Output format | Parseable text | Structured (JSON) |
| Custom system prompt | Yes | Yes |

---

## Additional Questions

1. Are there any **standards** emerging for CLI agent interoperability?

2. Are there **existing orchestration systems** that already do multi-agent coordination?

3. What **message queue formats** are commonly used (if any)?

4. Are there **benchmark suites** for evaluating CLI agent capabilities?

5. What **security models** do these tools use for sandboxing?

---

## Our Proposed Protocol (For Context)

We are considering a file-based protocol with SQLite as the backing store:

```
/project/.agents/
├── registry.json       # Agent definitions (human-editable)
├── dispatch.db         # SQLite queue (machine-readable)
└── sessions/
    └── {session-id}/   # Per-session context
        ├── context.md  # Human-readable context
        └── state.json  # Machine-readable state
```

**Dispatch Message Schema:**

```json
{
  "id": "msg-abc123",
  "from_agent": "architect",
  "to_agent": "coder",
  "session_id": "sess-xyz",
  "task": "Implement OAuth2 login",
  "context": {},
  "status": "pending",
  "result": null,
  "created_at": "2026-03-05T10:00:00Z",
  "completed_at": null
}
```

**Agent Registry Schema:**

```json
{
  "name": "coder",
  "description": "Writes and modifies code",
  "model_requirements": {
    "supports_tools": true,
    "min_context": 32000
  },
  "capabilities": ["edit", "bash", "read"],
  "system_prompt": "You are a coder agent...",
  "cli_command": "{agent_cli} --system '{system_prompt}' --resume {session_id}",
  "created_at": "2026-03-05T10:00:00Z"
}
```

The goal is that ANY CLI agent that can:

1. Read JSON config
2. Query SQLite
3. Execute non-interactively

...can participate in this system.
