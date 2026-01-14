# Crush Documentation

## Quick Navigation
- [System Overview](overview.md)
- [Architecture & Tech Stack](architecture.md)
- [Interfaces & APIs](interfaces/index.md)
  - [AI Provider APIs (REST)](interfaces/rest-apis.md)
  - [External Systems (LSP, MCP, etc.)](interfaces/external-systems.md)
- [Use Cases](use-cases/index.md)
  - [Interactive Coding Assistance](use-cases/interactive-coding.md)
- [Data Flows](data-flows/index.md)
  - [Request-Response Flow](data-flows/request-response.md)
  - [Tool Execution Flow](data-flows/tool-execution.md)
- [Configuration Reference](configuration.md)
- [Deployment Guide](deployment.md)
- [CI/CD Documentation](ci-cd.md)

## About This Documentation
Generated on: 2025-05-14
Repository analyzed: github.com/charmbracelet/crush

## Quick Start
Crush is an AI-powered coding assistant for your terminal. To get started:

1. **Install Crush:** Use your preferred package manager (e.g., `brew install charmbracelet/tap/crush`).
2. **Setup API Key:** Set up an API key for your preferred LLM provider (e.g., `export ANTHROPIC_API_KEY=your_key`).
3. **Run Crush:** Execute `crush` in your project root to start a session.
4. **Interact:** Ask the agent to help with tasks like "explain this project" or "write a test for this function".
