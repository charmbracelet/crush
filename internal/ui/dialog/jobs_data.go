package dialog

// Hàm thuần của hộp thoại Jobs: chuẩn hoá danh sách, định dạng chữ, chuẩn bị
// output. Không IO, không đụng trạng thái hộp thoại, nên test trực tiếp được.

import (
	"cmp"
	"fmt"
	"image/color"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/stringext"
	"github.com/charmbracelet/crush/internal/ui/common"
)

const (
	// jobsOutputMaxLineBytes: dòng output dài hơn mức này bị cắt còn
	// jobsOutputMaxLineCells ô, để một dòng khổng lồ (JSON một dòng, base64)
	// không làm chậm mỗi khung hình. Server đã giới hạn tổng output nên phần
	// còn lại vẫn có trần.
	jobsOutputMaxLineBytes = 16 << 10
	jobsOutputMaxLineCells = 4096

	// shortSessionIDLen là số ký tự đầu của session ID dùng làm nhãn khi chưa
	// biết tiêu đề.
	shortSessionIDLen = 8
)

// formatJobElapsed viết thời gian chạy gọn, tối đa 6 ô: 12s, 3m05s, 2h03m,
// 1d02h. Âm coi như 0.
func formatJobElapsed(d time.Duration) string {
	secs := max(int64(d/time.Second), 0)
	switch {
	case secs < 60:
		return fmt.Sprintf("%ds", secs)
	case secs < 3600:
		return fmt.Sprintf("%dm%02ds", secs/60, secs%60)
	case secs < 86400:
		return fmt.Sprintf("%dh%02dm", secs/3600, secs%3600/60)
	default:
		return fmt.Sprintf("%dd%02dh", min(secs/86400, 99), secs%86400/3600)
	}
}

// shortSessionID rút session ID về vài ký tự đầu.
func shortSessionID(id string) string {
	return ansi.Truncate(id, shortSessionIDLen, "")
}

// jobRank xếp job đang chạy trước.
func jobRank(j shell.JobInfo) int {
	if j.Status == shell.JobRunning {
		return 0
	}
	return 1
}

// normalizeJobs chuẩn bị danh sách từ workspace cho hộp thoại:
//   - bỏ trùng ID. Server có thể trả cùng một job hai lần ở khoảnh khắc nó
//     vừa xong (còn trong bảng đang chạy lẫn lịch sử), và sau khi kết nối lại
//     cũng không được nhân đôi dòng; giữ bản đã xong vì nó mới hơn;
//   - đang chạy trước, giữ nguyên thứ tự server trong mỗi nhóm (mới nhất
//     trước), kể cả khi server cũ trả thứ tự khác.
func normalizeJobs(in []shell.JobInfo) []shell.JobInfo {
	out := make([]shell.JobInfo, 0, len(in))
	at := make(map[string]int, len(in))
	for _, j := range in {
		if j.ID == "" {
			out = append(out, j)
			continue
		}
		if i, seen := at[j.ID]; seen {
			if out[i].Status == shell.JobRunning && j.Status != shell.JobRunning {
				out[i] = j
			}
			continue
		}
		at[j.ID] = len(out)
		out = append(out, j)
	}
	slices.SortStableFunc(out, func(a, b shell.JobInfo) int {
		return cmp.Compare(jobRank(a), jobRank(b))
	})
	return out
}

// jobTags liệt kê nhãn phụ của job: bg (chạy tiếp sau khi tool bash trả về),
// mã thoát của job lỗi, killed.
func jobTags(j shell.JobInfo) []string {
	var tags []string
	if j.Background {
		tags = append(tags, "bg")
	}
	switch j.Status {
	case shell.JobFailed:
		if j.ExitCode != 0 {
			tags = append(tags, fmt.Sprintf("exit %d", j.ExitCode))
		} else {
			tags = append(tags, "failed")
		}
	case shell.JobKilled:
		tags = append(tags, "killed")
	}
	return tags
}

// joinFields gộp xuống dòng, tab và khoảng trắng liền nhau thành một khoảng
// trắng, để chữ do người hay agent viết nằm gọn trên một dòng. Mã ANSI và ký
// tự điều khiển bị bỏ: lệnh và mô tả là chữ do model viết, không được tự nhiên
// đổi màu hay làm gì khác khung hình.
func joinFields(s string) string {
	s = strings.Join(strings.Fields(ansi.Strip(s)), " ")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// jobLabel là chữ mô tả job trên một dòng: description, không có thì lệnh
// (bỏ tiền tố "cd <thư mục> && " mà agent hay thêm).
func jobLabel(j shell.JobInfo) string {
	text := strings.TrimSpace(j.Description)
	if text == "" {
		text = common.StripBashDisplayPrefix(strings.TrimSpace(j.Command), j.WorkingDir)
	}
	return joinFields(text)
}

// jobStatusWord là tên trạng thái hiện trên tiêu đề khung output.
func jobStatusWord(s shell.JobStatus) string {
	switch s {
	case shell.JobRunning:
		return "running"
	case shell.JobSucceeded:
		return "succeeded"
	case shell.JobFailed:
		return "failed"
	case shell.JobKilled:
		return "killed"
	}
	return string(s)
}

// fitCell cắt s còn đúng w ô (thêm … khi bị cắt) rồi đệm khoảng trắng cho đủ.
func fitCell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// padLeftCell đệm khoảng trắng bên trái để s rộng đúng w ô (căn phải).
func padLeftCell(s string, w int) string {
	if pad := w - ansi.StringWidth(s); pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return ansi.Truncate(s, w, "")
}

// prepareJobOutput biến output thô của job thành các dòng để hiển thị. Chạy
// trong tea.Cmd (ngoài luồng Update): output có thể tới vài MB, không được
// làm việc này trên vòng lặp vẽ.
//
// Cùng đường xử lý với khung Bash trong chat: chuẩn hoá khoảng trắng, bỏ mã
// điều khiển con trỏ (thanh tiến trình của npm/git) và đưa màu ANSI 16 về
// bảng màu của theme. Dòng "[... N bytes of output dropped ...]" của server
// là một dòng bình thường nên giữ nguyên.
func prepareJobOutput(raw string, palette [16]color.Color) []string {
	text := stringext.NormalizeSpace(raw)
	text = common.StripCursorControl(text)
	text = common.RemapANSI16(text, palette)
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if len(line) > jobsOutputMaxLineBytes {
			lines[i] = ansi.Truncate(line, jobsOutputMaxLineCells, "…")
		}
	}
	return lines
}

// longestLine trả độ dài (byte) của dòng dài nhất, chặn trên cho cuộn ngang.
func longestLine(lines []string) int {
	longest := 0
	for _, line := range lines {
		longest = max(longest, len(line))
	}
	return longest
}
