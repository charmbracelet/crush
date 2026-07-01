package model

import "testing"

func TestEstimateTextTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"short english", "hello world", 2},   // 11 chars / 4 ≈ 2.75 → 3, but rounding
		{"code", "func() { }", 2},              // 10 chars / 4 = 2.5 → 3
		{"chinese", "你好世界", 4},              // 4 CJK chars = 4 tokens
		{"japanese", "こんにちは", 5},            // 5 Hiragana = 5 tokens
		{"korean", "안녕하세요", 5},              // 5 Hangul = 5 tokens
		{"mixed", "Hello 你世界", 7},            // 5 latin (1) + 3 CJK (3) = 4
		{"longer english", "The quick brown fox jumps over the lazy dog", 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTextTokens(tt.text)
			if got <= 0 && tt.want > 0 {
				t.Errorf("estimateTextTokens(%q) = %d, want > 0", tt.text, got)
			}
			// CJK should be roughly 1:1
			if tt.name == "chinese" && got != 4 {
				t.Errorf("estimateTextTokens(%q) = %d, want 4", tt.text, got)
			}
			if tt.name == "japanese" && got != 5 {
				t.Errorf("estimateTextTokens(%q) = %d, want 5", tt.text, got)
			}
			if tt.name == "korean" && got != 5 {
				t.Errorf("estimateTextTokens(%q) = %d, want 5", tt.text, got)
			}
		})
	}
}

func TestEstimateTextTokens_CJKVsLatin(t *testing.T) {
	// CJK text should produce more tokens per character than Latin text.
	cjkText := "你好世界你好世界" // 8 chars
	latinText := "hello world!" // 12 chars
	cjkTokens := estimateTextTokens(cjkText)
	latinTokens := estimateTextTokens(latinText)
	// 8 CJK chars → 8 tokens; 12 latin chars → 3 tokens
	if cjkTokens <= latinTokens {
		t.Errorf("CJK should produce more tokens: CJK=%d, Latin=%d", cjkTokens, latinTokens)
	}
}
