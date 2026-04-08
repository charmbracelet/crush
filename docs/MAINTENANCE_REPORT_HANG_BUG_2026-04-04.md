# CrushCL Hang Bug 維修報告

**日期**: 2026-04-04  
**問題**: CrushCL 執行 `run "hello"` 時 Hang 無回覆  
**耗時**: 約 2 小時

---

## 1. 問題描述

### 症狀
- 執行 `./crushcl run "hello"` 後，程式無輸出
- 10 秒後 timeout
- 沒有任何錯誤訊息

### 環境
- CrushCL 改造版 (module: `github.com/charmbracelet/crushcl`)
- MiniMax API
- Windows 環境

---

## 2. 排查過程

### 2.1 編譯問題修復

一開始遇到 import cycle 問題，修復了：
- `swarm_ext.go` - import path 錯誤
- `guardian.go` - import path 錯誤  
- `guardian_ext.go` - unused variable

### 2.2 Debug 方法

**關鍵洞察**: `slog.Debug()` 是緩衝日誌，進程 hang 住時無法刷新輸出

**解決方案**: 使用 `fmt.Fprintf(os.Stderr, ...)` 確保即時輸出

### 2.3 追蹤結果

```
1. ✅ resolveSession 完成
2. ✅ AutoApproveSession 完成
3. ✅ done channel 創建
4. ✅ Goroutine 啟動
5. ✅ coordinator.Run 進入
6. ✅ readyWg.Wait() 通過
7. ✅ UpdateModels() 成功
8. ✅ sessionAgent.Run 進入
9. ✅ createUserMessage 完成
10. ✅ preparePrompt 完成
11. ✅ eventPromptSent 完成
12. ✅ agent.Stream 進入
13. ✅ PrepareStep 進入
14. ⏳ Hang 發生在 estimateTokenCount → EstimateTokens
```

---

## 3. 根本原因

**檔案**: `internal/agent/token_estimator.go`

**Bug 程式碼**:
```go
case cp >= 0x00 && cp <= 0x7F:
    start := i
    for i < len(runes) {
        if i > 100 {  // <-- 這行 debug code 導致問題
            break      // <-- 提前退出，沒 advance i
        }
        r2, _ := utf8.DecodeRuneInString(text[i:])
        if r2 > 0x7F {
            break
        }
        i++
    }
    asciiLen := i - start
    totalTokens += (asciiLen + 3) / 4
```

**問題分析**:
1. 外層 `for i := 0; i < len(runes);` 迴圈
2. 內層迴圈在 `i > 100` 時提前 break
3. 外層迴圈變數 `i` 未被正確更新，永遠停在 101
4. 結果：外層迴圈永遠無法跑完，形成無窮迴圈

**為何之前沒發現**:
- `slog.Debug()` 是緩衝日誌，進程 hang 時無法刷新輸出
- 必須用 `fmt.Fprintf(os.Stderr)` 才能看到診斷輸出

---

## 4. 修復方案

**移除 debug code**:
```go
case cp >= 0x00 && cp <= 0x7F:
    start := i
    for i < len(runes) {       // 移除 if i > 100 { break }
        r2, _ := utf8.DecodeRuneInString(text[i:])
        if r2 > 0x7F {
            break
        }
        i++
    }
    asciiLen := i - start
    totalTokens += (asciiLen + 3) / 4
```

---

## 5. 修改的檔案

| 檔案 | 變更類型 | 說明 |
|------|----------|------|
| `internal/agent/token_estimator.go` | Bug Fix | 移除導致無窮迴圈的 debug code |
| `internal/agent/coordinator.go` | Bug Fix | 修復 slog.Debug arg 格式 |
| `internal/app/app.go` | Cleanup | 移除 debug 輸出 |
| `internal/agent/coordinator.go` | Cleanup | 移除 debug 輸出 |
| `internal/agent/agent.go` | Cleanup | 移除 debug 輸出 |
| `CHANGELOG.md` | Update | 更新為已完成狀態 |

---

## 6. 測試結果

### 功能測試
| 測試 | 結果 |
|------|------|
| `./crushcl run "hello"` | ✅ 成功 |
| `./crushcl run "What is 2+2?"` | ✅ 成功 |
| `./crushcl run "List files"` | ✅ 成功 |
| `./crushcl run "Chinese greeting"` | ✅ 成功 |
| `./crushcl run "Explain code"` | ✅ 成功 |
| 3x `./crushcl run "Say hi"` | ✅ 全部成功 |
| `./crushcl run "count Go lines"` | ✅ 成功 |

### 真實工作任務
| 任務 | 產出 |
|------|------|
| 創建架構文檔 | `docs/ARCHITECTURE.md` (695行) |
| 掃描 TODO/FIXME | `docs/TODO_REVIEW.md` (211行, 13項) |
| 審計 token_estimator | 確認無其他無窮迴圈 |
| 分析並發模式 | 7個檔案，6種模式 |
| 創建測試套件 | `internal/agent/token_estimator_test.go` (441行) |

### Unit Test (token_estimator_test.go)
| 測試 | 結果 |
|------|------|
| `TestTokenEstimator_HangBugFix_MultiByteUTF8` | ✅ 100次迭代無hang |
| `TestTokenEstimator_HangBugFix_DeepNesting` | ✅ 5秒內完成 |
| `TestTokenEstimator_HangBugFix_InvalidUTF8Boundaries` | ✅ 1000次迭代無hang |
| `TestTokenEstimator_VerifyByteVsRuneIndexing` | ✅ 複雜文本無hang |
| `TestTokenEstimator_SmokeTest` | ✅ 全部通過 |

---

## 7. 部署

修復後編譯為 `crushcl_final.exe`，並替換 `crushcl.exe`：
```bash
cp crushcl_final.exe crushcl.exe
```

---

## 8. 預防措施建議

1. **避免在迴圈中加 debug break** - 容易造成無窮迴圈
2. **使用非緩衝輸出用於診斷** - `fmt.Fprintf(os.Stderr)` 而非 `slog.Debug()`
3. **添加 Unit Test** - `token_estimator_test.go` 已建立，防止迴歸
4. **考慮用 Go race detector** - 檢測並發問題

---

## 9. 結論

**根本原因**: 一行 debug code `if i > 100 { break }` 導致無窮迴圈

**修復**: 移除該行

**驗證**: 
- 功能測試全部通過
- 真實工作任務成功執行
- Unit test 確認無迴歸

**狀態**: ✅ 已完成修復

---

*報告生成時間: 2026-04-04 20:35*
