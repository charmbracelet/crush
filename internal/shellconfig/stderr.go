package shellconfig

import (
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"mvdan.cc/sh/v3/syntax"
)

// maxStderrTailBytes là số byte cuối của stderr mà crushrc đã in được giữ lại
// để đính vào lỗi nạp cấu hình. Đủ chứa vài dòng lý do, không đủ làm ngập log.
const maxStderrTailBytes = 2048

// stderrTail là io.Writer chỉ nhớ maxBytes byte ghi sau cùng, nên script ồn ào
// (in cả megabyte) cũng không làm phình bộ nhớ. Write không bao giờ trả lỗi: ghi
// stderr không được phép làm hỏng việc nạp cấu hình.
//
// Lý do thật của một lần crushrc thất bại gần như luôn nằm ở cuối stderr, vì
// vậy giữ phần đuôi chứ không giữ phần đầu.
type stderrTail struct {
	mu        sync.Mutex
	maxBytes  int
	buf       []byte
	truncated bool // đã bỏ bớt phần đầu vì vượt maxBytes
}

func newStderrTail(maxBytes int) *stderrTail {
	return &stderrTail{maxBytes: maxBytes}
}

// Write implements io.Writer. Mutex vì trình thông dịch có thể ghi từ nhiều
// goroutine (lệnh nền `cmd &`, hoặc tiến trình con có stdout và stderr riêng).
func (t *stderrTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	n := len(p)
	if n >= t.maxBytes {
		t.truncated = t.truncated || n > t.maxBytes || len(t.buf) > 0
		t.buf = append(t.buf[:0], p[n-t.maxBytes:]...)
		return n, nil
	}
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - t.maxBytes; over > 0 {
		t.buf = append(t.buf[:0], t.buf[over:]...)
		t.truncated = true
	}
	return n, nil
}

// lines trả về phần đuôi đã làm sạch, tách theo dòng, và cho biết phần đầu có bị
// cắt bỏ hay không. Làm sạch gồm: bỏ chuỗi ANSI, bỏ byte UTF-8 hỏng (kể cả rune
// bị cắt dở ở đầu đuôi), bỏ ký tự điều khiển, gộp khoảng trắng, bỏ dòng rỗng.
func (t *stderrTail) lines() (lines []string, truncated bool) {
	t.mu.Lock()
	raw := string(t.buf)
	truncated = t.truncated
	t.mu.Unlock()

	raw = ansi.Strip(strings.ToValidUTF8(raw, ""))
	raw = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(raw)

	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.Join(strings.FieldsFunc(strings.Map(printableOrSpace, line), unicode.IsSpace), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, truncated
}

// printableOrSpace giữ ký tự in được, đổi mọi khoảng trắng (tab, NBSP...) thành
// dấu cách và bỏ phần còn lại (ký tự điều khiển, NUL, DEL).
func printableOrSpace(r rune) rune {
	switch {
	case unicode.IsSpace(r):
		return ' '
	case unicode.IsPrint(r):
		return r
	default:
		return -1
	}
}

// summary gộp các dòng thành một dòng duy nhất, nối bằng " | ", để log và thông
// báo lỗi của client không phải xử lý xuống dòng. Trả "" khi không có gì.
func (t *stderrTail) summary() string {
	lines, truncated := t.lines()
	if len(lines) == 0 {
		return ""
	}
	text := strings.Join(lines, " | ")
	if truncated {
		text = "..." + text
	}
	return text
}

// errorSuffix dựng phần đuôi ": <stderr>" để nối vào err, hoặc "" khi không có gì
// đáng nối.
//
// Không nối khi err đã chứa nguyên văn phần đuôi: các builtin cấu hình (provider,
// model...) vừa in lý do ra stderr vừa trả chính lý do đó làm lỗi, nối thêm sẽ
// lặp lại hai lần.
func errorSuffix(err error, summary string) string {
	if summary == "" || strings.Contains(err.Error(), summary) {
		return ""
	}
	return ": " + summary
}

// devStderrPaths là các đường dẫn trỏ tới stderr THẬT của tiến trình. Trình
// thông dịch mở chúng như tệp thường (os.OpenFile), nên ghi vào đó đi thẳng ra
// stderr của tiến trình chứ không qua writer mà ta truyền cho shell.Run.
var devStderrPaths = map[string]bool{
	"/dev/stderr":     true,
	"/dev/fd/2":       true,
	"/proc/self/fd/2": true,
}

// redirectDevStderr viết lại các chuyển hướng tới stderr thật của tiến trình
// (`2>/dev/stderr`, `>/dev/stderr`, `&>/dev/stderr`...) thành nhân bản fd của
// chính trình thông dịch (`2>&2`, `>&2`), để chúng chảy vào stderrTail như stderr
// thường. `N>/dev/stderr` vốn có nghĩa là `N>&2` trong Bash, nên ngữ nghĩa giữ
// nguyên; chỉ khác là không còn mở tệp thật.
//
// Mở thật có hai hậu quả xấu. Một là ta không thu được lý do để đính vào lỗi.
// Hai là `>` mở với O_TRUNC: ở chế độ client/server, stderr của server là tệp
// stderr.log, nên lần mở này xoá sạch tệp và lý do bị chính thông báo lỗi cuối
// của server ghi đè từ đầu tệp.
//
// Chỉ viết lại ở mức văn bản nguồn theo vị trí byte do parser cung cấp, nên phần
// còn lại của script (chú thích, định dạng, số dòng) giữ nguyên từng byte. Script
// lỗi cú pháp được trả về nguyên vẹn để shell.Run báo lỗi như trước. Chuyển hướng
// nằm trong tệp được `source` hoặc có đích tính lúc chạy (`2>"$LOG"`) không được
// phát hiện.
func redirectDevStderr(src string) string {
	file, err := syntax.NewParser().Parse(strings.NewReader(src), "")
	if err != nil {
		return src
	}

	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	syntax.Walk(file, func(n syntax.Node) bool {
		rd, ok := n.(*syntax.Redirect)
		if !ok || rd.Word == nil || rd.Hdoc != nil {
			return true
		}
		target, isLiteral := literalWord(rd.Word)
		if !isLiteral || !devStderrPaths[target] {
			return true
		}

		var text string
		switch rd.Op {
		case syntax.RdrOut, syntax.AppOut:
			// Chỉ fd 1, 2 hoặc mặc định; `3>` hay `{fd}>` để nguyên.
			fd := ""
			if rd.N != nil {
				fd = rd.N.Value
			}
			if fd != "" && fd != "1" && fd != "2" {
				return true
			}
			text = fd + ">&2"
		case syntax.RdrAll, syntax.AppAll:
			// `&>/dev/stderr`: stdout về stderr, stderr giữ nguyên.
			text = ">&2"
		default:
			return true
		}

		// Kiểm lại trước khi thay: đoạn nguồn theo vị trí parser phải thật sự
		// chứa đích chuyển hướng. Lệch thì bỏ qua, thà không bắt được còn hơn
		// sửa hỏng script của người dùng (vd đích viết kèm `\` xuống dòng).
		start, end := int(rd.Pos().Offset()), int(rd.End().Offset())
		if start >= end || end > len(src) || !strings.Contains(src[start:end], target) {
			return true
		}
		edits = append(edits, edit{start: start, end: end, text: text})
		return true
	})
	if len(edits) == 0 {
		return src
	}

	// Thay từ cuối về đầu để vị trí các đoạn phía trước không bị lệch.
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := src
	for _, e := range edits {
		out = out[:e.start] + e.text + out[e.end:]
	}
	return out
}

// literalWord trả về giá trị chữ của w khi w chỉ gồm chữ thường và chuỗi trong
// nháy đơn/kép không chứa mở rộng (`/dev/stderr`, `"/dev/stderr"`). Có biến hay
// lệnh con thì ok=false vì giá trị chỉ biết lúc chạy.
func literalWord(w *syntax.Word) (value string, ok bool) {
	var sb strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			if p.Dollar {
				return "", false
			}
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, inner := range p.Parts {
				lit, isLit := inner.(*syntax.Lit)
				if !isLit {
					return "", false
				}
				sb.WriteString(lit.Value)
			}
		default:
			return "", false
		}
	}
	return sb.String(), true
}
