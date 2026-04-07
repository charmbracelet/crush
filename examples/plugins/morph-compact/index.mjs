import { createInterface } from "node:readline";
import { createHash } from "node:crypto";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import path from "node:path";
import os from "node:os";
import { CompactClient } from "@morphllm/morphsdk";

const protocolVersion = 1;
const charsPerToken = 3;
const compactContextThreshold = Number.parseFloat(process.env.MORPH_COMPACT_CONTEXT_THRESHOLD || "0.7");
const compactPreserveRecent = Number.parseInt(process.env.MORPH_COMPACT_PRESERVE_RECENT || "2", 10);
const compactRatio = Number.parseFloat(process.env.MORPH_COMPACT_RATIO || "0.3");
const compactTokenLimit = process.env.MORPH_COMPACT_TOKEN_LIMIT ? Number.parseInt(process.env.MORPH_COMPACT_TOKEN_LIMIT, 10) : null;
const defaultModelContextTokens = Number.parseInt(process.env.MORPH_MODEL_CONTEXT_TOKENS || "200000", 10);
const morphApiKey = process.env.MORPH_API_KEY;
const morphApiUrl = process.env.MORPH_API_URL || "https://api.morphllm.com";
const compactTimeout = Number.parseInt(process.env.MORPH_COMPACT_TIMEOUT || "60000", 10);
const statsEnabled = process.env.MORPH_COMPACT_STATS !== "false";
const saveCompactionText = process.env.MORPH_COMPACT_SAVE_TEXT === "true";

// 统计文件路径 - 全局存储在用户目录下
const homeDir = os.homedir();
const stateDir = process.env.MORPH_COMPACT_STATE_DIR || path.join(homeDir, ".morph-compact");
const statsFile = path.join(stateDir, "stats.json");
const compactionTextDir = path.join(stateDir, "compactions");

const compactClient = morphApiKey
  ? new CompactClient({
      morphApiKey,
      morphApiUrl,
      timeout: compactTimeout,
    })
  : null;

// 内存状态存储
const stateMap = new Map();

// 压缩统计
const stats = {
  totalCompactions: 0,
  totalCharsBefore: 0,
  totalCharsAfter: 0,
  totalMs: 0,
  sessions: {},
  recentResults: [], // 最近2次压缩详情
  startTime: Date.now(),
};

// 加载已有统计
async function loadStats() {
  try {
    const data = await readFile(statsFile, "utf8");
    const loaded = JSON.parse(data);
    Object.assign(stats, loaded);
  } catch {
    // 文件不存在或解析失败，使用默认值
  }
}

// 保存统计到文件
async function saveStats() {
  if (!statsEnabled) return;
  try {
    await mkdir(stateDir, { recursive: true });
    await writeFile(statsFile, JSON.stringify(stats, null, 2), "utf8");
  } catch (err) {
    // 静默失败，不影响主功能
  }
}

function recordCompaction(sessionId, charsBefore, charsAfter, ms, compactedCount, frozenCount, recentCount, result, messagesBefore, messagesAfter) {
  if (statsEnabled) {
    stats.totalCompactions++;
    stats.totalCharsBefore += charsBefore;
    stats.totalCharsAfter += charsAfter;
    stats.totalMs += ms;

    if (!stats.sessions[sessionId]) {
      stats.sessions[sessionId] = { compactions: 0, lastCompaction: null };
    }
    stats.sessions[sessionId].compactions++;
    stats.sessions[sessionId].lastCompaction = Date.now();

    // 保留最近2次详细结果
    stats.recentResults.unshift({
      sessionId,
      timestamp: Date.now(),
      charsBefore,
      charsAfter,
      ratio: charsBefore > 0 ? charsAfter / charsBefore : 0,
      ms,
      compactedCount,
      frozenCount,
      recentCount,
      totalAfter: frozenCount + recentCount,
      usage: result?.usage,
    });
    if (stats.recentResults.length > 2) {
      stats.recentResults.pop();
    }

    // 异步保存到文件
    saveStats();
  }

  // 保存压缩前后的消息（如果启用）
  if (saveCompactionText && messagesBefore !== undefined && messagesAfter !== undefined) {
    saveCompactionTextFiles(sessionId, messagesBefore, messagesAfter);
  }
}

async function saveCompactionTextFiles(sessionId, messagesBefore, messagesAfter) {
  try {
    await mkdir(compactionTextDir, { recursive: true });
    const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
    const sessionPrefix = sessionId.slice(0, 8);
    const beforeFile = path.join(compactionTextDir, `${sessionPrefix}_${timestamp}_before.json`);
    const afterFile = path.join(compactionTextDir, `${sessionPrefix}_${timestamp}_after.json`);
    
    const beforePayload = messagesToOpenAIFormat(messagesBefore);
    const afterPayload = messagesToOpenAIFormat(messagesAfter);
    
    await writeFile(beforeFile, JSON.stringify(beforePayload, null, 2), "utf8");
    await writeFile(afterFile, JSON.stringify(afterPayload, null, 2), "utf8");
  } catch (err) {
    // 静默失败
  }
}

function formatStats() {
  const lines = [];
  lines.push("=== Morph Compact Statistics ===\n");

  if (stats.totalCompactions === 0) {
    lines.push("No compactions performed yet.");
    lines.push(`Stats enabled: ${statsEnabled}`);
    lines.push(`Stats file: ${statsFile}`);
    return lines.join("\n");
  }

  const avgRatio = stats.totalCharsAfter / stats.totalCharsBefore;
  const avgMs = stats.totalMs / stats.totalCompactions;
  const uptimeMs = Date.now() - stats.startTime;
  const uptimeMin = Math.round(uptimeMs / 60000);

  lines.push(`Uptime: ${uptimeMin} minutes`);
  lines.push(`Total Compactions: ${stats.totalCompactions}`);
  lines.push(`Total Time: ${stats.totalMs}ms (avg: ${Math.round(avgMs)}ms)`);
  lines.push(`Compression Ratio: ${(avgRatio * 100).toFixed(1)}% kept`);
  lines.push(`Chars Saved: ${formatNumber(stats.totalCharsBefore - stats.totalCharsAfter)}`);
  lines.push(`Active Sessions: ${Object.keys(stats.sessions).length}`);
  lines.push(`Stats File: ${statsFile}`);
  lines.push("");

  // 按压缩次数排序会话
  const sortedSessions = Object.entries(stats.sessions)
    .sort((a, b) => b[1].compactions - a[1].compactions)
    .slice(0, 5);

  if (sortedSessions.length > 0) {
    lines.push("Top Sessions:");
    for (const [sessionId, sessionStats] of sortedSessions) {
      const lastTime = sessionStats.lastCompaction
        ? new Date(sessionStats.lastCompaction).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false }).replace(/\//g, "-")
        : "never";
      lines.push(`  ${sessionId.slice(0, 12)}... : ${sessionStats.compactions} compactions (last: ${lastTime})`);
    }
    lines.push("");
  }

  // 最近2次压缩详情
  if (stats.recentResults.length > 0) {
    lines.push("Recent Compactions:");
    for (let i = 0; i < stats.recentResults.length; i++) {
      const r = stats.recentResults[i];
      const time = new Date(r.timestamp).toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false }).replace(/\//g, "-");
      lines.push(`\n  [${i + 1}] ${time}`);
      lines.push(`      Session: ${r.sessionId.slice(0, 12)}...`);
      // 兼容旧格式
      if (r.compactedCount !== undefined) {
        lines.push(`      Compacted: ${r.compactedCount} → ${r.frozenCount} frozen + ${r.recentCount} recent = ${r.totalAfter} total`);
      } else {
        lines.push(`      Messages: ${r.messagesBefore} → ${r.messagesAfter}`);
      }
      lines.push(`      Chars: ${formatNumber(r.charsBefore)} → ${formatNumber(r.charsAfter)} (${(r.ratio * 100).toFixed(1)}% kept)`);
      lines.push(`      Time: ${r.ms}ms`);
      if (r.usage) {
        lines.push(`      API Ratio: ${(r.usage.compression_ratio * 100).toFixed(1)}%`);
      }
    }
  }

  return lines.join("\n");
}

function formatNumber(n) {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
  if (n >= 1_000) return (n / 1_000).toFixed(1) + "K";
  return String(n);
}

// 命令行模式：--stats 或 --reset-stats
async function handleCliArgs() {
  if (process.argv.includes("--stats")) {
    await loadStats();
    console.log(formatStats());
    process.exit(0);
  }
  if (process.argv.includes("--reset-stats")) {
    const emptyStats = {
      totalCompactions: 0,
      totalCharsBefore: 0,
      totalCharsAfter: 0,
      totalMs: 0,
      sessions: {},
      recentResults: [],
      startTime: Date.now(),
    };
    await mkdir(stateDir, { recursive: true });
    await writeFile(statsFile, JSON.stringify(emptyStats, null, 2), "utf8");
    console.log("Stats reset.");
    process.exit(0);
  }
}

// Parent process liveness check
const crushPid = Number.parseInt(process.env.CRUSH_PID || "", 10);
if (Number.isFinite(crushPid) && crushPid > 0) {
  const livenessTimer = setInterval(() => {
    try {
      process.kill(crushPid, 0);
    } catch {
      process.exit(0);
    }
  }, 5000);
  livenessTimer.unref?.();
}

async function main() {
  // 先检查命令行参数
  await handleCliArgs();

  // 加载已有统计
  if (statsEnabled) {
    await loadStats();
  }

  const rl = createInterface({ input: process.stdin, crlfDelay: Infinity });

  for await (const line of rl) {
    if (!line.trim()) continue;
    await handleRequest(line);
  }
}

async function handleRequest(raw) {
  let id;
  try {
    const request = JSON.parse(raw);
    id = request.id;

    if (request.version !== protocolVersion) {
      return writeResponse({ id, error: `unsupported protocol version: ${request.version}` });
    }

    const input = request.input || {};
    const output = request.output || {};

    if (request.event === "chat_messages_transform") {
      return handleChatMessagesTransform(id, input, output);
    }
    if (request.event === "session_compacting") {
      return handleSessionCompacting(id, output);
    }
    return writeResponse({ id, output });
  } catch (error) {
    return writeResponse({ id, error: error instanceof Error ? error.message : String(error) });
  }
}

async function handleChatMessagesTransform(id, input, output) {
  if (!compactClient) {
    return writeResponse({ id, output });
  }
  const purpose = input.purpose || "request";
  // Allow compaction for request, recover, AND summarize to prevent context overflow
  const allowCompaction = purpose === "request" || purpose === "recover" || purpose === "summarize";
  const messages = Array.isArray(output.messages) ? output.messages : [];
  if (messages.length <= compactPreserveRecent) {
    return writeResponse({ id, output });
  }

  const sessionId = input.session_id || input.sessionId || "default";

  // 优先使用动态传入的模型上下文，回退到环境变量
  const modelContextTokens = input.model?.context_window > 0
    ? input.model.context_window
    : defaultModelContextTokens;

  const charThreshold = compactTokenLimit
    ? compactTokenLimit * charsPerToken
    : modelContextTokens * compactContextThreshold * charsPerToken;

  const state = stateMap.get(sessionId);

  // Check if we have frozen messages from previous compaction
  if (state && Array.isArray(state.frozenMessages) && state.frozenMessages.length > 0) {
    // Build a map from original IDs to their compacted versions
    // frozenMessages contains messages with morph-compact- prefixed IDs
    // We need to match incoming messages by their original ID (stored in the compacted message)
    const originalToCompacted = new Map();
    for (const msg of state.frozenMessages) {
      // The ID format is: morph-compact-<originalId>
      // Extract the original ID
      let originalId = msg.id;
      if (originalId.startsWith("morph-compact-")) {
        originalId = originalId.slice("morph-compact-".length);
      }
      originalToCompacted.set(originalId, msg);
      // Also map the compacted ID to itself for direct matching
      originalToCompacted.set(msg.id, msg);
    }

    // Separate messages into: already compacted (use frozen version) vs truly new
    const frozenMessages = [];
    const newMessages = [];

    for (const msg of messages) {
      const compacted = originalToCompacted.get(msg.id);
      if (compacted) {
        // Use the compacted version instead of the original
        frozenMessages.push(compacted);
      } else {
        // This is a new message we haven't seen
        newMessages.push(msg);
      }
    }

    // Combine frozen (compacted) + new messages
    const combinedMessages = [...frozenMessages, ...newMessages];

    if (!allowCompaction) {
      return writeResponse({ id, output: { ...output, messages: combinedMessages } });
    }

    // Check if compaction is needed
    const totalChars = estimateTotalChars(combinedMessages);
    if (totalChars < charThreshold) {
      return writeResponse({ id, output: { ...output, messages: combinedMessages } });
    }

    // Only compact the new messages (not already frozen ones)
    if (newMessages.length <= compactPreserveRecent) {
      return writeResponse({ id, output: { ...output, messages: combinedMessages } });
    }

    // Compact new messages
    const compactedNew = await compactNewMessages(sessionId, frozenMessages, estimateTotalChars(frozenMessages), newMessages, charThreshold);
    return writeResponse({ id, output: { ...output, messages: compactedNew } });
  }

  // No previous frozen messages - check if compaction is needed
  if (!allowCompaction) {
    return writeResponse({ id, output });
  }

  const totalChars = estimateTotalChars(messages);
  if (totalChars < charThreshold) {
    return writeResponse({ id, output });
  }

  // First-time compaction
  const next = await compactMessages(sessionId, messages);
  return writeResponse({ id, output: { ...output, messages: next } });
}

async function compactMessages(sessionId, messages) {
  // First-time compaction: compact all but recent messages
  const toCompact = messages.slice(0, -compactPreserveRecent);
  const recent = messages.slice(-compactPreserveRecent);
  if (toCompact.length === 0) {
    return messages;
  }

  const compactInput = messagesToCompactInput(toCompact);
  if (compactInput.length === 0) {
    return messages;
  }

  const charsBefore = estimateTotalChars(toCompact);
  const startMs = Date.now();

  try {
    const result = await compactClient.compact({
      messages: compactInput,
      compressionRatio: compactRatio,
      preserveRecent: 0,
    });
    const ms = Date.now() - startMs;
    const frozen = buildCompactedMessages(toCompact, result, sessionId);
    const frozenChars = estimateTotalChars(frozen);

    // All messages we return (frozen + recent) become the "known" set for next time
    const returnedMessages = [...frozen, ...recent];
    const returnedChars = frozenChars + estimateTotalChars(recent);

    // Store only frozen (compacted) messages for future incremental compaction
    // Recent messages are NOT stored - they will be re-evaluated next time
    stateMap.set(sessionId, {
      frozenMessages: frozen,
      frozenChars,
    });

    const messagesBefore = saveCompactionText ? toCompact : undefined;
    const messagesAfter = saveCompactionText ? frozen : undefined;

    recordCompaction(sessionId, charsBefore, frozenChars, ms, toCompact.length, frozen.length, recent.length, result, messagesBefore, messagesAfter);

    return returnedMessages;
  } catch {
    return messages;
  }
}

async function compactNewMessages(sessionId, existingFrozen, existingFrozenChars, newMessages, charThreshold) {
  // Check if we need to compact new messages
  const newChars = estimateTotalChars(newMessages);
  const totalChars = existingFrozenChars + newChars;

  if (totalChars < charThreshold) {
    // No compaction needed, return frozen + new
    const returned = [...existingFrozen, ...newMessages];
    // Only store frozen messages in state
    stateMap.set(sessionId, {
      frozenMessages: existingFrozen,
      frozenChars: existingFrozenChars,
    });
    return returned;
  }

  if (newMessages.length <= compactPreserveRecent) {
    // Not enough new messages to compact
    const returned = [...existingFrozen, ...newMessages];
    // Only store frozen messages in state
    stateMap.set(sessionId, {
      frozenMessages: existingFrozen,
      frozenChars: existingFrozenChars,
    });
    return returned;
  }

  // Compact new messages
  const toCompact = newMessages.slice(0, -compactPreserveRecent);
  const recent = newMessages.slice(-compactPreserveRecent);

  if (toCompact.length === 0) {
    // Nothing to compact, only store frozen messages
    stateMap.set(sessionId, {
      frozenMessages: existingFrozen,
      frozenChars: existingFrozenChars,
    });
    return [...existingFrozen, ...newMessages];
  }

  const compactInput = messagesToCompactInput(toCompact);
  if (compactInput.length === 0) {
    // Nothing to compact, only store frozen messages
    stateMap.set(sessionId, {
      frozenMessages: existingFrozen,
      frozenChars: existingFrozenChars,
    });
    return [...existingFrozen, ...newMessages];
  }

  const charsBefore = estimateTotalChars(toCompact);
  const startMs = Date.now();

  try {
    const result = await compactClient.compact({
      messages: compactInput,
      compressionRatio: compactRatio,
      preserveRecent: 0,
    });
    const ms = Date.now() - startMs;
    const newFrozen = buildCompactedMessages(toCompact, result, sessionId);
    const newFrozenChars = estimateTotalChars(newFrozen);

    // Merge with existing frozen messages
    const combinedFrozen = [...existingFrozen, ...newFrozen];
    const recentChars = estimateTotalChars(recent);
    const combinedChars = existingFrozenChars + newFrozenChars + recentChars;

    // All returned messages (combinedFrozen + recent) become the new known set
    const returnedMessages = [...combinedFrozen, ...recent];

    // Only update state if we actually compacted something
    // Store only frozen messages - recent ones will be re-evaluated next time
    stateMap.set(sessionId, {
      frozenMessages: combinedFrozen,
      frozenChars: existingFrozenChars + newFrozenChars,
    });

    const messagesBefore = saveCompactionText ? toCompact : undefined;
    const messagesAfter = saveCompactionText ? newFrozen : undefined;

    recordCompaction(sessionId, charsBefore, newFrozenChars, ms, toCompact.length, newFrozen.length, recent.length, result, messagesBefore, messagesAfter);

    return returnedMessages;
  } catch {
    // On error, return frozen + new messages without compaction
    // Only store frozen messages - new messages will be re-evaluated next time
    stateMap.set(sessionId, {
      frozenMessages: existingFrozen,
      frozenChars: existingFrozenChars,
    });
    return [...existingFrozen, ...newMessages];
  }
}

async function handleSessionCompacting(id, output) {
  const context = Array.isArray(output.context) ? [...output.context] : [];
  context.push("Note: Morph compact plugin is active. Older messages may already be compressed.");
  return writeResponse({ id, output: { ...output, context } });
}

function buildCompactedMessages(originalMessages, result, sessionId) {
  if (!Array.isArray(result.messages) || result.messages.length !== originalMessages.length) {
    const template = originalMessages[0];
    return [buildSyntheticMessage(template, result.output || "", sessionId, "user")];
  }
  return result.messages.map((item, index) => {
    const original = originalMessages[index];
    return buildSyntheticMessage(original, item.content || "", sessionId, item.role || "user");
  });
}

function buildSyntheticMessage(original, text, sessionId, role) {
  // Use original ID (without morph-compact prefix) as source
  let sourceId = original?.id || "";
  // If already a morph-compact ID, extract the original part
  if (sourceId.startsWith("morph-compact-")) {
    sourceId = sourceId.slice("morph-compact-".length);
  }
  // If no valid ID, generate from text hash
  if (!sourceId) {
    sourceId = createHash("sha256").update(text).digest("hex").slice(0, 12);
  }
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
    const inputStr = serializeField(part.data?.input).slice(0, 500);
    return `[ToolCall: ${part.data?.name || "unknown"}] ${inputStr}`;
  }
  if (part.type === "tool_result") {
    const contentStr = serializeField(part.data?.content).slice(0, 2000);
    return `[ToolResult: ${part.data?.name || "unknown"}] ${contentStr}`;
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
      if (part.type === "text") {
        total += (part.data?.text || "").length;
      } else if (part.type === "reasoning") {
        total += (part.data?.thinking || "").length;
      } else if (part.type === "tool_call") {
        total += serializeField(part.data?.input).length;
      } else if (part.type === "tool_result") {
        total += serializeField(part.data?.content).length;
      }
    }
  }
  return total;
}

function messagesToOpenAIFormat(messages) {
  return {
    model: "unknown",
    messages: messages.map((msg) => {
      const content = [];
      const toolCalls = [];
      const toolCallId = msg.role === "tool" ? (msg.parts?.[0]?.data?.tool_call_id || null) : null;
      
      for (const part of msg.parts || []) {
        if (part.type === "text" && part.data?.text) {
          content.push({ type: "text", text: part.data.text });
        } else if (part.type === "reasoning" && part.data?.thinking) {
          content.push({ type: "text", text: `[Reasoning] ${part.data.thinking}` });
        } else if (part.type === "tool_call") {
          const tc = part.data || {};
          toolCalls.push({
            id: tc.id || `call_${Math.random().toString(36).slice(2, 11)}`,
            type: "function",
            function: {
              name: tc.name || "unknown",
              arguments: typeof tc.input === "string" ? tc.input : JSON.stringify(tc.input || {})
            }
          });
        } else if (part.type === "tool_result") {
          const tr = part.data || {};
          const toolResultContent = tr.content || "";
          content.push({
            type: "text",
            text: typeof toolResultContent === "string" ? toolResultContent : JSON.stringify(toolResultContent)
          });
        } else if (part.type === "binary" && part.data?.data) {
          content.push({
            type: "image_url",
            image_url: {
              url: `data:${part.data.mime_type || "application/octet-stream"};base64,${part.data.data}`
            }
          });
        }
      }
      
      const result = { role: msg.role };
      
      if (toolCallId) {
        result.tool_call_id = toolCallId;
      }
      
      if (content.length > 0) {
        if (content.length === 1 && content[0].type === "text") {
          result.content = content[0].text;
        } else {
          result.content = content;
        }
      }
      
      if (toolCalls.length > 0) {
        result.tool_calls = toolCalls;
      }
      
      return result;
    }).filter((msg) => Object.keys(msg).length > 1 || msg.role)
  };
}

function writeResponse(response) {
  process.stdout.write(JSON.stringify(response) + "\n");
}

main().catch((error) => {
  process.stderr.write((error instanceof Error ? error.message : String(error)) + "\n");
  process.exitCode = 1;
});
