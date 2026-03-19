import { mkdir, readFile, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import path from "node:path";
import { CompactClient } from "@morphllm/morphsdk";

const protocolVersion = 1;
const charsPerToken = 3;
const compactContextThreshold = Number.parseFloat(process.env.MORPH_COMPACT_CONTEXT_THRESHOLD || "0.7");
const compactPreserveRecent = Number.parseInt(process.env.MORPH_COMPACT_PRESERVE_RECENT || "1", 10);
const compactRatio = Number.parseFloat(process.env.MORPH_COMPACT_RATIO || "0.3");
const compactTokenLimit = process.env.MORPH_COMPACT_TOKEN_LIMIT ? Number.parseInt(process.env.MORPH_COMPACT_TOKEN_LIMIT, 10) : null;
const modelContextTokens = Number.parseInt(process.env.MORPH_MODEL_CONTEXT_TOKENS || "200000", 10);
const morphApiKey = process.env.MORPH_API_KEY;
const morphApiUrl = process.env.MORPH_API_URL || "https://api.morphllm.com";
const compactTimeout = Number.parseInt(process.env.MORPH_COMPACT_TIMEOUT || "60000", 10);
const workingDir = process.env.CRUSH_WORKING_DIR || process.cwd();
const stateDir = process.env.MORPH_COMPACT_STATE_DIR || path.join(workingDir, ".crush", "plugins", "morph-compact");

const compactClient = morphApiKey
  ? new CompactClient({
      morphApiKey,
      morphApiUrl,
      timeout: compactTimeout,
    })
  : null;

async function main() {
  const raw = await readStdin();
  const request = JSON.parse(raw);
  if (request.version !== protocolVersion) {
    return writeResponse({ error: `unsupported protocol version: ${request.version}` });
  }

  const input = request.input || {};
  const output = request.output || {};

  if (request.event === "chat_messages_transform") {
    return handleChatMessagesTransform(input, output);
  }
  if (request.event === "session_compacting") {
    return handleSessionCompacting(output);
  }
  return writeResponse({ output });
}

async function handleChatMessagesTransform(input, output) {
  if (!compactClient) {
    return writeResponse({ output });
  }
  if (input.purpose === "summarize") {
    return writeResponse({ output });
  }
  const messages = Array.isArray(output.messages) ? output.messages : [];
  if (messages.length <= compactPreserveRecent) {
    return writeResponse({ output });
  }

  const sessionId = input.session_id || input.sessionId || "default";
  const state = await readState(sessionId);
  const charThreshold = compactTokenLimit
    ? compactTokenLimit * charsPerToken
    : modelContextTokens * compactContextThreshold * charsPerToken;

  if (state) {
    const validFrozenMessages = Array.isArray(state.frozenMessages) ? state.frozenMessages : [];
    const validFrozenChars = Number.isFinite(state.frozenChars) ? state.frozenChars : estimateTotalChars(validFrozenMessages);
    const rawIndex = Number.isInteger(state.compactedUpToIndex) ? state.compactedUpToIndex : 0;
    const staleState = rawIndex < 0 || rawIndex > messages.length;
    const frozenMessages = staleState ? [] : validFrozenMessages;
    const frozenChars = staleState ? 0 : validFrozenChars;
    const compactedUpToIndex = staleState ? 0 : rawIndex;
    if (staleState) {
      await writeState(sessionId, {
        frozenMessages,
        compactedUpToIndex,
        frozenChars,
      });
    }

    const uncompacted = messages.slice(compactedUpToIndex);
    const effectiveChars = frozenChars + estimateTotalChars(uncompacted);
    if (effectiveChars < charThreshold) {
      return writeResponse({ output: { ...output, messages: [...frozenMessages, ...uncompacted] } });
    }
    if (uncompacted.length <= compactPreserveRecent) {
      return writeResponse({ output: { ...output, messages: [...frozenMessages, ...uncompacted] } });
    }
    const next = await compactMessages(sessionId, messages, uncompacted, false);
    return writeResponse({ output: { ...output, messages: next } });
  }

  if (estimateTotalChars(messages) < charThreshold) {
    return writeResponse({ output });
  }

  const next = await compactMessages(sessionId, messages, messages, true);
  return writeResponse({ output: { ...output, messages: next } });
}

async function compactMessages(sessionId, allMessages, sourceMessages, firstCompaction) {
  const toCompact = sourceMessages.slice(0, -compactPreserveRecent);
  const recent = sourceMessages.slice(-compactPreserveRecent);
  if (toCompact.length === 0) {
    return allMessages;
  }

  const compactInput = messagesToCompactInput(toCompact);
  if (compactInput.length === 0) {
    return allMessages;
  }

  try {
    const result = await compactClient.compact({
      messages: compactInput,
      compressionRatio: compactRatio,
      preserveRecent: 0,
    });
    const frozen = buildCompactedMessages(toCompact, result, sessionId);
    const frozenChars = estimateTotalChars(frozen);
    const compactedUpToIndex = firstCompaction ? allMessages.length - recent.length : allMessages.length - recent.length;
    await writeState(sessionId, {
      frozenMessages: frozen,
      compactedUpToIndex,
      frozenChars,
    });
    return [...frozen, ...recent];
  } catch {
    const state = await readState(sessionId);
    if (state) {
      const uncompacted = allMessages.slice(state.compactedUpToIndex);
      return [...state.frozenMessages, ...uncompacted];
    }
    return allMessages;
  }
}

async function handleSessionCompacting(output) {
  const context = Array.isArray(output.context) ? [...output.context] : [];
  context.push("Morph compact plugin is active. Older messages may already be compressed.");
  return writeResponse({ output: { ...output, context } });
}

function buildCompactedMessages(originalMessages, result, sessionId) {
  if (!Array.isArray(result.messages) || result.messages.length !== originalMessages.length) {
    const template = originalMessages[0];
    return [buildSyntheticMessage(template, result.output || "", sessionId, "user")];
  }
  return result.messages.map((item, index) => {
    const original = originalMessages[index];
    return buildSyntheticMessage(original, item.content || "", sessionId, item.role || original.role || "user");
  });
}

function buildSyntheticMessage(original, text, sessionId, role) {
  const sourceId = original?.id || createHash("sha256").update(text).digest("hex").slice(0, 12);
  return {
    ...original,
    id: `morph-compact-${sourceId}`,
    role,
    session_id: original?.session_id || sessionId,
    parts: [
      {
        type: "text",
        data: {
          text,
        },
      },
    ],
  };
}

function messagesToCompactInput(messages) {
  return messages
    .map((message) => ({
      role: message.role,
      content: (message.parts || []).map(serializePart).join("\n"),
    }))
    .filter((message) => message.content.length > 0);
}

function serializePart(part) {
  if (!part || !part.type) {
    return "";
  }
  if (part.type === "text") {
    return part.data?.text || "";
  }
  if (part.type === "reasoning") {
    return `[Reasoning] ${part.data?.thinking || ""}`;
  }
  if (part.type === "tool_call") {
    return `[ToolCall: ${part.data?.name || "unknown"}] ${serializeField(part.data?.input)}`;
  }
  if (part.type === "tool_result") {
    return `[ToolResult: ${part.data?.name || "unknown"}] ${serializeField(part.data?.content)}`;
  }
  if (part.type === "finish") {
    return "";
  }
  return `[${part.type}]`;
}

function serializeField(value) {
  if (value === undefined || value === null) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean" || typeof value === "bigint") {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function estimateTotalChars(messages) {
  let total = 0;
  for (const message of messages) {
    for (const part of message.parts || []) {
      total += serializePart(part).length;
    }
  }
  return total;
}

async function readState(sessionId) {
  try {
    const statePath = getStatePath(sessionId);
    const raw = await readFile(statePath, "utf8");
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

async function writeState(sessionId, state) {
  const statePath = getStatePath(sessionId);
  await mkdir(path.dirname(statePath), { recursive: true });
  await writeFile(statePath, JSON.stringify(state), "utf8");
}

function getStatePath(sessionId) {
  const safe = sessionId.replace(/[^a-zA-Z0-9._-]/g, "_");
  return path.join(stateDir, `${safe}.json`);
}

function readStdin() {
  return new Promise((resolve, reject) => {
    const chunks = [];
    process.stdin.on("data", (chunk) => chunks.push(chunk));
    process.stdin.on("end", () => resolve(Buffer.concat(chunks).toString("utf8")));
    process.stdin.on("error", reject);
  });
}

function writeResponse(response) {
  process.stdout.write(JSON.stringify(response));
}

main().catch((error) => {
  writeResponse({ error: error instanceof Error ? error.message : String(error) });
  process.exitCode = 1;
});
