# Interfaces Overview

Crush interacts with several external and internal interfaces to provide its functionality.

## External Interfaces
- **AI Provider APIs:** Consumes REST APIs from various LLM providers.
- **LSP Servers:** Communicates with local or remote Language Servers via JSON-RPC.
- **MCP Servers:** Communicates with Model Context Protocol servers via stdio, HTTP, or SSE.
- **Sourcegraph API:** Consumes the Sourcegraph GraphQL API for remote code search.

## Internal Interfaces
- **SQLite Database:** Local persistent storage for session and message history.
- **Filesystem:** Interacts with the local project files and configuration directories.
- **OS Shell:** Executes bash commands via the `bash` tool.

### Interface Details
- [AI Provider APIs](rest-apis.md)
- [External Systems (LSP, MCP, etc.)](external-systems.md)
