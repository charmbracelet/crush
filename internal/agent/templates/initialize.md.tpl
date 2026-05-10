Create or update **{{.Config.Options.InitializeAs}}** to help future agents work in this codebase.

First: check if directory is empty or only contains config. If so, stop and say "Directory appears empty or only contains config. Add source code first."

**Discovery**: Explore the codebase to understand commands, architecture, conventions, and gotchas. Look at config files, scripts, CI config, and representative source. If {{.Config.Options.InitializeAs}} already exists, read it and improve it.

**Include**: essential commands (build/test/lint/run), code organization and data flow, naming/style patterns, testing approach, non-obvious gotchas, and project-specific context from existing rule files.

**Avoid**: obvious details an agent would pick up from reading one file, invented facts you didn't actually observe, and generic advice. Focus on what saves the agent from trial-and-error: surprising flags, implicit conventions, context not self-evident from a single file.

Format as clear Markdown sections. Prioritize non-obvious knowledge over comprehensive coverage.
