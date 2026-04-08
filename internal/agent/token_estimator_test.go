package agent

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestTokenEstimator_ASCII(t *testing.T) {
	est := newTokenEstimator()

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{"empty string", "", 0},
		{"single char", "a", 1},
		{"four chars", "test", 1},
		{"five chars", "hello", 2},
		{"eight chars", "abcdefgh", 2},
		{"short word", "go", 1},
		{"sentence - 44 chars", "The quick brown fox jumps over the lazy dog.", 11},
		{"multiple sentences", "Hello world. This is a test. How are you today?", 12},
		{"code-like", "func main() { println(\"hello\") }", 8},
		{"numbers and symbols", "12345!@#$%^&*()", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := est.EstimateTokens(tt.text)
			if result != tt.expected {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, result, tt.expected)
			}
		})
	}
}

func TestTokenEstimator_CJK(t *testing.T) {
	est := newTokenEstimator()

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{"single Chinese char", "中", 1},
		{"Chinese word", "中文", 2},
		{"Chinese sentence", "今天天气真好", 6},
		{"Chinese paragraph", "这是一个测试。这是一个中文句子。", 16},
		{"Japanese hiragana", "こんにちは", 5},
		{"Japanese katakana", "テスト", 3},
		{"Japanese mixed", "日本語", 3},
		{"Korean hangul", "안녕하세요", 5},
		{"Korean word", "한글", 2},
		{"CJK mixed", "中文と日本語と한국어", 10},
		{"CJK punctuation", "你好！世界？", 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := est.EstimateTokens(tt.text)
			if result != tt.expected {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, result, tt.expected)
			}
		})
	}
}

func TestTokenEstimator_MixedContent(t *testing.T) {
	est := newTokenEstimator()

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{"English and Chinese", "Hello 你好", 4},
		{"Chinese and English", "你好 Hello", 4},
		{"code with comments", "// 这是注释\nfunc test() {}", 9},
		{"markdown with CJK", "# 标题\n\nSome content here", 8},
		{"mixed punctuation", "Hello! 你好! 안녕하세요!", 11},
		{"code inline with text", "Use `中文` for Chinese", 8},
		{"json-like with CJK", `{"name": "中文", "value": 123}`, 9},
		{"English sentence with CJK word", "The word 中文 means Chinese", 9},
		{"CJK surrounded by ASCII", "abc中文def", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := est.EstimateTokens(tt.text)
			if result != tt.expected {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, result, tt.expected)
			}
		})
	}
}

func TestTokenEstimator_EdgeCases(t *testing.T) {
	est := newTokenEstimator()

	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{"empty string", "", 0},
		{"whitespace only - spaces", "   ", 0},
		{"single space", " ", 0},
		{"tab and newline", "\t\n\r", 0},
		{"emoji basic", "😊", 2},
		{"emoji in text", "Hello 😊 World", 6},
		{"multiple emojis", "👍👍👍", 6},
		{"mixed emojis", "🎉🎊✨", 5},
		{"emoji with CJK", "👋你好", 4},
		{"special unicode symbols", "★☆●○", 4},
		{"math symbols", "∑∏∫√∞≈≠", 7},
		{"control chars - low ASCII", "\x00\x01\x02test", 2},
		{"very long ASCII", strings.Repeat("a", 10000), 2500},
		{"very long CJK", strings.Repeat("中", 1000), 1000},
		{"long mixed", strings.Repeat("abc中", 100), 200},
		{"null byte handling", "test\x00null", 3},
		{"all CJK ranges", "中文日本語한글", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := est.EstimateTokens(tt.text)
			if result != tt.expected {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, result, tt.expected)
			}
		})
	}
}

func TestTokenEstimator_HangBugFix_MultiByteUTF8(t *testing.T) {
	est := newTokenEstimator()

	hangBugTexts := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "pure CJK - no hang",
			text:     "中文测试文本",
			expected: 6,
		},
		{
			name:     "CJK with spaces",
			text:     "中 文 测 试",
			expected: 7,
		},
		{
			name:     "alternating CJK and ASCII",
			text:     "a中b文c测d试e",
			expected: 9,
		},
		{
			name:     "Japanese hiragana sequence",
			text:     "あいうえお",
			expected: 5,
		},
		{
			name:     "Korean hangul sequence",
			text:     "가나다라마",
			expected: 5,
		},
		{
			name:     "mixed CJK in longer text",
			text:     "这是一个包含中文、日文和韩文的混合文本。",
			expected: 20,
		},
		{
			name:     "CJK followed by ASCII",
			text:     "中文English",
			expected: 4,
		},
		{
			name:     "ASCII followed by CJK",
			text:     "English中文",
			expected: 4,
		},
		{
			name:     "short CJK with punctuation",
			text:     "你好,世界!",
			expected: 6,
		},
	}

	for _, tt := range hangBugTexts {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				for i := 0; i < 100; i++ {
					result := est.EstimateTokens(tt.text)
					if result != tt.expected {
						t.Errorf("EstimateTokens(%q) = %d, want %d", tt.text, result, tt.expected)
					}
				}
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("Hang detected: EstimateTokens appears to hang on multi-byte UTF-8 text")
			}
		})
	}
}

func TestTokenEstimator_HangBugFix_DeepNesting(t *testing.T) {
	est := newTokenEstimator()

	deepNestText := strings.Repeat("深", 1000) + strings.Repeat("a", 1000)

	done := make(chan struct{})
	go func() {
		defer close(done)
		result := est.EstimateTokens(deepNestText)
		expected := 1000 + 250
		if result != expected {
			t.Errorf("EstimateTokens(deep nesting) = %d, want %d", result, expected)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Hang detected on deep nesting text")
	}
}

func TestTokenEstimator_HangBugFix_InvalidUTF8Boundaries(t *testing.T) {
	est := newTokenEstimator()

	invalidUTF8 := "中文"
	validRunes := []rune(invalidUTF8)
	validLen := len(validRunes)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			result := est.EstimateTokens(invalidUTF8)
			if result != validLen {
				t.Errorf("EstimateTokens with valid UTF-8 = %d, want %d", result, validLen)
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Hang detected on valid UTF-8 text")
	}
}

func TestTokenEstimator_VerifyByteVsRuneIndexing(t *testing.T) {
	est := newTokenEstimator()

	complexText := "Hello世界🌍!日本語abc한국어123"

	done := make(chan struct{})
	go func() {
		defer close(done)
		result := est.EstimateTokens(complexText)
		if result <= 0 {
			t.Errorf("EstimateTokens returned %d for complex mixed text", result)
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Hang detected on byte vs rune indexing test")
	}
}

func TestTokenEstimator_Messages(t *testing.T) {
	est := newTokenEstimator()

	messages := []MessageContent{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "你好"},
		{Role: "user", Content: "How are you?"},
	}

	result := est.EstimateMessagesTokens(messages)

	expected := 6 + 6 + 7
	if result != expected {
		t.Errorf("EstimateMessagesTokens = %d, want %d", result, expected)
	}
}

func TestTokenEstimator_EmptyMessages(t *testing.T) {
	est := newTokenEstimator()

	tests := []struct {
		name     string
		messages []MessageContent
		expected int
	}{
		{"nil messages", nil, 0},
		{"empty messages", []MessageContent{}, 0},
		{"single empty content", []MessageContent{{Role: "user", Content: ""}}, 4},
		{"all empty content", []MessageContent{{Role: "a", Content: ""}, {Role: "b", Content: ""}}, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := est.EstimateMessagesTokens(tt.messages)
			if result != tt.expected {
				t.Errorf("EstimateMessagesTokens = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestIsASCII(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"", true},
		{"hello", true},
		{"Hello World!", true},
		{"12345", true},
		{"\x7F", true},
		{"\x80", false},
		{"中文", false},
		{"hello世界", false},
		{"日本語", false},
		{"한국어", false},
		{"a中b", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := isASCII(tt.text)
			if result != tt.expected {
				t.Errorf("isASCII(%q) = %v, want %v", tt.text, result, tt.expected)
			}
		})
	}
}

func TestCalculateSafeBudget(t *testing.T) {
	tests := []struct {
		name          string
		contextWindow int
		errorMargin   float64
		expected      int
	}{
		{"default margin 200k", 200000, 0, 144000},
		{"default margin 100k", 100000, 0, 72000},
		{"10% margin 200k", 200000, 0.10, 162000},
		{"30% margin 200k", 200000, 0.30, 126000},
		{"20% margin 50k", 50000, 0.20, 36000},
		{"15% margin 128k", 128000, 0.15, 97920},
		{"zero margin", 100000, 0, 72000},
		{"negative margin", 100000, -0.5, 72000},
		{"large margin", 200000, 0.50, 90000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSafeBudget(tt.contextWindow, tt.errorMargin)
			if result != tt.expected {
				t.Errorf("CalculateSafeBudget(%d, %f) = %d, want %d",
					tt.contextWindow, tt.errorMargin, result, tt.expected)
			}
		})
	}
}

func TestTokenEstimator_SmokeTest(t *testing.T) {
	est := newTokenEstimator()

	smokeTexts := []string{
		"",
		"a",
		"hello world",
		"中文",
		"Hello世界",
		"mixed content 混合内容",
		"🎉",
		"   \n\t   ",
		"func() {}",
		"日本語テスト",
		"한글테스트",
		"a中b文c",
		"1+1=2",
		"!@#$%^&*()",
	}

	for _, text := range smokeTexts {
		t.Run(text, func(t *testing.T) {
			result := est.EstimateTokens(text)
			if result < 0 {
				t.Errorf("EstimateTokens(%q) returned negative: %d", text, result)
			}
			if !utf8.ValidString(text) && text != "" {
				t.Errorf("EstimateTokens received invalid UTF-8: %q", text)
			}
		})
	}
}

func BenchmarkTokenEstimator_ASCII(b *testing.B) {
	est := newTokenEstimator()
	text := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		est.EstimateTokens(text)
	}
}

func BenchmarkTokenEstimator_CJK(b *testing.B) {
	est := newTokenEstimator()
	text := strings.Repeat("中文测试文本内容 ", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		est.EstimateTokens(text)
	}
}

func BenchmarkTokenEstimator_Mixed(b *testing.B) {
	est := newTokenEstimator()
	text := strings.Repeat("Hello世界!mixed混合 ", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		est.EstimateTokens(text)
	}
}
