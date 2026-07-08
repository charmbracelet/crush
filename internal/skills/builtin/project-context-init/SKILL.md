---
name: project-context-init
description: Use when initializing, updating, or designing project instruction files such as AGENTS.md, CLAUDE.md, CRUSH.md, GEMINI.md, LLM.md, .cursor/rules, or Copilot instructions.
---

# Project Context Initialization

Create instruction files from observed repository facts only.

## Discovery

1. List the project root.
2. Read existing instruction files when present:
   - AGENTS.md, agents.md, Agents.md
   - CLAUDE.md, CLAUDE.local.md
   - CRUSH.md, crush.md, local variants
   - GEMINI.md, gemini.md
   - LLM.md, LLMs.md, docs/LLMs.md
   - .cursorrules, .cursor/rules/*.md
   - .github/copilot-instructions.md
3. Identify project type from config files and directory structure.
4. Find build, run, lint, test, typecheck, deploy, and migration commands from scripts, Makefiles, CI, or docs.
5. Read representative source files to capture architecture, data flow, naming, and non-obvious conventions.

## Content Rules

- Include commands that were observed, not invented.
- Include gotchas that save future agents from trial and error.
- Avoid generic advice obvious from reading one file.
- Preserve user or project preferences from existing instruction files.
- Note verification commands and when to use each one.
- Prefer concise sections with concrete paths and commands.

## Multi-Agent Compatibility

- AGENTS.md is the broadest cross-agent default.
- CLAUDE.md is useful when Claude Code is part of the workflow.
- CRUSH.md is useful for Crush-specific local behavior.
- docs/LLMs.md or LLM.md is useful when humans and multiple agents need shared guidance.

If several files exist, do not overwrite one with another's style. Merge durable facts carefully and keep tool-specific rules in the tool-specific file.
