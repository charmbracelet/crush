# Crushcl Architect + Executor 協作文件

## 目標
實現真正的 L3FullCompact 功能（Fork Agent Summarization）

## 現狀問題
agent.go:346-351 中，L3FullCompact 被降級到 L2：
```go
case kctx.L3FullCompact:
    slog.Warn("L3 Compression: FullCompact requested (deferred)")
    prepared.Messages = a.compactor.L2AutoCompact(prepared.Messages, call.SessionID)
```

## 需要實現
1. 非同步 fork agent 摘要生成
2. 當 token > 190,000 (95%) 時觸發
3. 使用 fantasy SDK 的子 agent 功能
4. 失敗時 fallback 到 L2

## 請給出
1. 具體的函數簽名
2. 關鍵實現邏輯
3. 與現有 compactor 的整合方式

## 輸出格式
直接給出可粘貼到 compactor.go 的完整函數代碼。
