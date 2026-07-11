# Release binaries

Prebuilt Windows and Linux binaries are published on the GitHub Releases page:

https://github.com/reitaard/crush-re.configured/releases

The binaries are release assets rather than Git objects because each build is
larger than GitHub's 100 MB per-file repository limit.

MCP servers and language servers are external tools. Crush loads their
configuration and auto-detects available language servers, but does not install
third-party runtimes without user consent.
