package flowrag

import (
	"regexp"
	"strings"
)

var completionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*(ok|okay|好的|好|行|可以|没问题|就这样|就这样了|搞定|完成|好了|行了|没问题了|ok了|fine|done|great|perfect|cool|sounds good|alright)[\s，。,.!！吧啦喽咯嘛呢啊呀哎嗨呵嘿哟噢]*$`),
	regexp.MustCompile(`(?i)^\s*(ok|okay|好的|好|行|可以|没问题|就这样|就这样了|搞定|完成|好了|行了|没问题了|ok了)\s*[，,]\s*(就|就这样|这样|没问题|ok)[\s，。,.!！吧啦喽咯嘛呢啊呀哎嗨呵嘿哟噢]*$`),
}

var taskCompleteKeywords = []string{
	"task complete", "task completed", "task done",
	"workflow complete", "workflow completed",
	"save workflow", "save this workflow", "save the workflow",
	"remember this", "remember for next time",
	"save for future", "save for later",
	"store workflow", "store this flow",
}

type CompletionDetector struct {
	patterns []*regexp.Regexp
}

func NewCompletionDetector() *CompletionDetector {
	return &CompletionDetector{patterns: completionPatterns}
}

func (d *CompletionDetector) IsCompletionPhrase(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	for _, pattern := range d.patterns {
		if pattern.MatchString(trimmed) {
			return true
		}
	}
	return false
}

func (d *CompletionDetector) IsTaskCompleteMarker(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, kw := range taskCompleteKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func (d *CompletionDetector) ShouldTriggerFlowRAG(text string) bool {
	return d.IsCompletionPhrase(text) || d.IsTaskCompleteMarker(text)
}
