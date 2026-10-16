package session

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// maxDerivedTitleWidth là độ dài tối đa của tiêu đề suy ra từ prompt,
// trùng giới hạn 50 cột terminal mà title prompt LLM phải theo.
const maxDerivedTitleWidth = 50

// DerivedTitle suy ra tiêu đề session từ prompt đầu tiên: một dòng, khoảng
// trắng gọn lại, tối đa 50 cột terminal. Trả về rỗng khi prompt không còn chữ nào
// dùng được, để caller tự rơi về tên mặc định của nó.
func DerivedTitle(prompt string) string {
	line := strings.ReplaceAll(ansi.Strip(prompt), "\n", " ")
	line = strings.ReplaceAll(line, "\r", " ")
	line = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return -1
		}
		return r
	}, line)
	line = strings.Join(strings.Fields(line), " ")
	if line == "" {
		return ""
	}
	return ansi.Truncate(line, maxDerivedTitleWidth, "…")
}
