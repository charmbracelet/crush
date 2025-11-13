Analyze this codebase and create/update **{{.Config.Options.InitializeAs}}** to help future agents work effectively in this repository.

**First**: Check if directory is empty or contains only config files. If so, stop and say "Directory appears empty or only contains config. Add source code first, then run this command to generate {{.Config.Options.InitializeAs}}."

**Goal**: Document what an agent needs to know to work in this codebase - commands, patterns, conventions, gotchas.

**Discovery process**:

1. Check directory contents with `ls`
2. Look for existing rule files (`.cursor/rules/*.md`, `.cursorrules`, `.github/copilot-instructions.md`, `claude.md`, `agents.md`) - only read if they exist
3. Identify project type from config files and directory structure
4. Find build/test/lint commands from config files, scripts, Makefiles, or CI configs
5. Read representative source files to understand code patterns
6. If {{.Config.Options.InitializeAs}} exists, read and improve it

**Content to include**:

- Essential commands (build, test, run, deploy, etc.) - whatever is relevant for this project
- Code organization and structure
- Naming conventions and style patterns
- Testing approach and patterns
- Important gotchas or non-obvious patterns
- Any project-specific context from existing rule files

**Format**: Clear markdown sections. Use your judgment on structure based on what you find. Aim for completeness over brevity - include everything an agent would need to know.

**Critical**: Only document what you actually observe. Never invent commands, patterns, or conventions. If you can't find something, don't include it.
