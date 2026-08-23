# Pair a phone with Build Remote Agent

Crush can use **Build Remote Agent** as a pairing device: the iOS/Android app
spectates (and can inject into) this Crush session through the free MIT
`gbr-agent`. Phone and PC never open ports to each other.

Website: https://grokbuildremote.com/
Agent: https://github.com/LinespottingOrg/GrokBuildRemote-Agents (MIT)
Protocol: `gbr/1` · need agent **v0.6.0+**

Independent product. Not affiliated with xAI or SpaceX.

This is not a second pair protocol and not `crush serve`. Pairing stays
`gbr-agent pair` + `gbr-agent run`. Attach is loopback only.

## Install + pair

```bash
# macOS / Linux
curl -fsSL https://grokbuildremote.com/install.sh | bash
gbr-agent version          # must print v0.6.0 or newer
gbr-agent pair             # QR in browser + printed 8-char code
gbr-agent run              # leave running
```

```powershell
# Windows
irm https://grokbuildremote.com/install.ps1 | iex
gbr-agent version
gbr-agent pair
gbr-agent run
```

Phone: open Build Remote Agent → **Scan QR from computer** (or type the 8-char
code). Sessions appear in the app. **Unpair** in Settings before changing PCs.
Force-close is not enough.

## Attach from Crush (MCP)

After `gbr-agent run`, add a stdio MCP that talks to the Bot API on
`http://127.0.0.1:8788`:

```bash
# crushrc  (~/.config/crush/crushrc)

git clone https://github.com/LinespottingOrg/GrokBuildRemote-Agents.git \
  "$HOME/src/GrokBuildRemote-Agents"
cd "$HOME/src/GrokBuildRemote-Agents/mcp/gbr-mcp" && npm install

mcp add gbr --command node \
  --args "$HOME/src/GrokBuildRemote-Agents/mcp/gbr-mcp/bin/gbr-mcp.js"
```

Diagnose without Crush:

```bash
node "$HOME/src/GrokBuildRemote-Agents/mcp/gbr-mcp/bin/gbr-mcp.js" --diagnose
curl -sS http://127.0.0.1:8788/health
curl -sS http://127.0.0.1:8788/v1/sessions
```

Phone is spectator + veto. Orchestration stays in Crush.

Do not commit mailbox keys. Phone **Settings → Bot API** is the only place the
relay key is copied.

Loop: diagnose → open/attach → lock → inject → wait idle → harvest excerpt →
iterate or close.
