# System Overview

## Purpose
Crush is a professional AI-powered coding assistant designed for the terminal. It provides a terminal-based user interface (TUI) that integrates seamlessly with your development environment, allowing you to interact with various Large Language Models (LLMs) to help with coding tasks, debugging, and exploration.

## Key Features
- **Multi-Model Support:** Choose from a wide range of LLMs (GPT-4, Claude 3.5 Sonnet, Gemini, etc.) or add your own via OpenAI or Anthropic-compatible APIs.
- **Session-Based Workflows:** Maintain multiple independent work sessions and contexts per project.
- **LSP Integration:** Enhances context by using Language Server Protocol (LSP) to provide information about code symbols, references, and diagnostics.
- **MCP Extensibility:** Supports the Model Context Protocol (MCP) to extend capabilities with external tools and services.
- **Built-in Coding Tools:** Includes a rich set of tools for file operations (`view`, `ls`, `grep`, `edit`, `multiedit`, `write`), web search, and bash execution.
- **Automatic Context Management:** Automatically summarizes long conversations to stay within LLM context windows and manages project-specific context files (e.g., `AGENTS.md`).
- **Privacy and Control:** Respects `.gitignore` and `.crushignore`, prompts for tool permissions by default, and allows opting out of metrics.

## Target Audience
- Developers who prefer working in the terminal.
- Engineers looking for a customizable AI coding assistant.
- Teams wanting to integrate AI tools into their terminal-based workflows.
