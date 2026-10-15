package dialog

// Vẽ hộp thoại Jobs: danh sách, khung xem output và câu hỏi dừng job. Mọi hàm
// ở đây chỉ đọc trạng thái (trừ vài con số cuộn cần biết chiều cao thật) và
// không làm IO.
//
// Dùng token style có sẵn: màu trạng thái của header job bash
// (Tool.JobIcon*), chữ phụ của dialog, và diện mạo "huỷ diệt" của khung xoá
// session cho câu hỏi dừng job.

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

const (
	jobsDialogMaxWidth = 100
	jobsOutputMaxWidth = 140
	jobsListMaxRows    = 14
	jobsOutputMaxRows  = 60

	// Bề rộng các cột của một dòng: ô thời gian cố định, cột ID, cột nhãn
	// (bg, exit N, killed) và cột session có trần; phần còn lại thuộc về mô
	// tả.
	jobsElapsedWidth    = 6
	jobsIDMaxWidth      = 6
	jobsTagsMaxWidth    = 16
	jobsSessionMaxWidth = 24
	jobsMinSessionWidth = 4
	jobsMinDescWidth    = 14

	// jobIconKilled là biểu tượng job bị dừng; ba trạng thái còn lại dùng
	// lại biểu tượng của tool call.
	jobIconKilled = "■"
)

// Draw implements [Dialog].
func (d *Jobs) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	var view string
	if d.mode == jobsModeOutput {
		view = d.outputView(area)
	} else {
		view = d.listView(area)
	}
	DrawCenterCursor(scr, area, view, nil)
	return nil
}

// newRenderContext dựng khung; câu hỏi dừng job đổi sang viền đỏ như khung
// xoá session.
func (d *Jobs) newRenderContext(width int) *RenderContext {
	t := d.com.Styles
	rc := NewRenderContext(t, width)
	if d.confirm != nil {
		rc.TitleStyle = t.Dialog.Sessions.DeletingTitle
		rc.TitleGradientFromColor = t.Dialog.Sessions.DeletingTitleGradientFromColor
		rc.TitleGradientToColor = t.Dialog.Sessions.DeletingTitleGradientToColor
		rc.ViewStyle = t.Dialog.Sessions.DeletingView
	}
	return rc
}

// fitHeight dựng khung với rows dòng thân rồi bớt dòng cho tới khi vừa vùng
// vẽ, để không phụ thuộc vào việc ước lượng phần viền và tiêu đề của từng
// theme.
func fitHeight(build func(rows int) string, areaHeight, rows int) string {
	rows = max(rows, 1)
	view := build(rows)
	for over := lipgloss.Height(view) - areaHeight; over > 0 && rows > 1; over = lipgloss.Height(view) - areaHeight {
		rows = max(rows-over, 1)
		view = build(rows)
	}
	return view
}

// mutedStyle là chữ phụ không padding.
func (d *Jobs) mutedStyle() lipgloss.Style {
	return d.com.Styles.Dialog.SecondaryText.UnsetPadding()
}

// errorLine là một dòng báo lỗi cắt vừa chiều rộng.
func (d *Jobs) errorLine(text string, innerWidth int) string {
	return d.com.Styles.Dialog.TitleError.Padding(0, 1).
		Render(ansi.Truncate(joinFields(text), max(0, innerWidth-2), "…"))
}

// jobIcon trả biểu tượng và style màu của trạng thái job.
func (d *Jobs) jobIcon(j shell.JobInfo) (string, lipgloss.Style) {
	t := d.com.Styles
	switch j.Status {
	case shell.JobRunning:
		return styles.ToolPending, t.Tool.JobIconPending
	case shell.JobSucceeded:
		return styles.ToolSuccess, t.Tool.JobIconSuccess
	case shell.JobFailed:
		return styles.ToolError, t.Tool.JobIconError
	case shell.JobKilled:
		return jobIconKilled, t.Tool.StateCancelled
	}
	return "?", t.Tool.StateCancelled
}

// tagsOf là nhãn phụ của job, thêm "stopping" khi lệnh dừng đã gửi đi.
func (d *Jobs) tagsOf(j shell.JobInfo) []string {
	tags := jobTags(j)
	if d.stopping[j.ID] && j.Status == shell.JobRunning {
		tags = append([]string{"stopping"}, tags...)
	}
	return tags
}

// summary là số job đang chạy và đã xong, hiện cạnh tiêu đề.
func (d *Jobs) summary() string {
	if len(d.jobs) == 0 {
		return ""
	}
	running := 0
	for _, j := range d.jobs {
		if j.Status == shell.JobRunning {
			running++
		}
	}
	return d.mutedStyle().Render(fmt.Sprintf(" %d running · %d finished", running, len(d.jobs)-running))
}

// --- Danh sách ---

func (d *Jobs) listView(area uv.Rectangle) string {
	t := d.com.Styles
	width := max(0, min(jobsDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	build := func(rows int) string {
		rc := d.newRenderContext(width)
		rc.Title = "Jobs"
		rc.TitleInfo = d.summary()
		rc.Gap = 1
		if d.confirm != nil {
			rc.AddPart(d.stopPrompt(innerWidth))
		}
		rc.AddPart(d.listBody(innerWidth, rows))
		rc.Help = renderDialogHelp(t, &d.help, d, innerWidth)
		return rc.Render()
	}
	return fitHeight(build, area.Dy(), min(max(len(d.jobs), 1), jobsListMaxRows))
}

// listBody vẽ rows dòng job (cuộn theo lựa chọn), hoặc chữ thay thế khi chưa
// có gì để liệt kê.
func (d *Jobs) listBody(innerWidth, rows int) string {
	if len(d.jobs) == 0 {
		return d.listPlaceholder(innerWidth)
	}
	d.listRows = rows
	d.scrollListIntoView(rows)

	bar := len(d.jobs) > rows
	outer := innerWidth
	if bar {
		outer-- // Một cột cho thanh cuộn.
	}
	rowWidth := max(0, outer-2) // Mỗi dòng có padding một ô hai bên.
	cols := d.columns(rowWidth)

	end := min(d.listTop+rows, len(d.jobs))
	lines := make([]string, 0, end-d.listTop)
	for _, j := range d.jobs[d.listTop:end] {
		lines = append(lines, d.jobRow(j, j.ID == d.selectedID, cols, rowWidth))
	}
	body := strings.Join(lines, "\n")
	if bar {
		body = joinScrollbar(d.com.Styles, body, len(lines), len(d.jobs), rows, d.listTop)
	}
	if d.loadErr != nil {
		body += "\n" + d.errorLine("Refresh failed: "+d.loadErr.Error(), innerWidth)
	}
	return body
}

func (d *Jobs) listPlaceholder(innerWidth int) string {
	muted := d.com.Styles.Dialog.SecondaryText
	switch {
	case d.loadErr != nil:
		return d.errorLine("Could not list jobs: "+d.loadErr.Error(), innerWidth)
	case !d.loaded:
		return muted.Render("Loading jobs…")
	}
	hint := ansi.Truncate("Commands the agent runs with bash appear here.", max(0, innerWidth-2), "…")
	return muted.Render("No jobs yet.") + "\n" + muted.Render(hint)
}

// scrollListIntoView cuộn để dòng đang chọn nằm trong rows dòng hiện.
func (d *Jobs) scrollListIntoView(rows int) {
	i := max(d.selectedIndex(), 0)
	if i < d.listTop {
		d.listTop = i
	}
	if i >= d.listTop+rows {
		d.listTop = i - rows + 1
	}
	d.listTop = min(max(d.listTop, 0), max(len(d.jobs)-rows, 0))
}

// jobsColumns là bề rộng các cột có thể co của một dòng; 0 là ẩn cột.
type jobsColumns struct{ id, tags, session int }

// fixedWidth là phần dòng không thuộc mô tả: biểu tượng, thời gian, các cột
// phụ và khoảng cách.
func (c jobsColumns) fixedWidth() int {
	w := 1 + 1 + jobsElapsedWidth + 1
	if c.id > 0 {
		w += c.id + 1
	}
	if c.tags > 0 {
		w += c.tags + 1
	}
	if c.session > 0 {
		w += 2 + c.session
	}
	return w
}

// columns chọn bề rộng các cột phụ theo nội dung hiện có, nhường chỗ cho mô
// tả trước: khi màn hình hẹp, cột session co lại rồi biến mất, sau đó tới cột
// ID rồi cột nhãn.
func (d *Jobs) columns(rowWidth int) jobsColumns {
	var c jobsColumns
	for _, j := range d.jobs {
		c.id = max(c.id, ansi.StringWidth(j.ID))
		c.tags = max(c.tags, ansi.StringWidth(strings.Join(d.tagsOf(j), " ")))
		c.session = max(c.session, ansi.StringWidth(d.sessionLabel(j.SessionID)))
	}
	c.id = min(c.id, jobsIDMaxWidth)
	c.tags = min(c.tags, jobsTagsMaxWidth)
	c.session = min(c.session, jobsSessionMaxWidth)

	// Cột session lấy phần còn lại sau khi dành chỗ tối thiểu cho mô tả. Bị
	// ép hẹp quá mức thì bỏ hẳn, nhưng nội dung vốn ngắn (tiêu đề "OK") thì
	// giữ.
	spare := rowWidth - jobsColumns{id: c.id, tags: c.tags}.fixedWidth() - jobsMinDescWidth - 2
	natural := c.session
	c.session = min(natural, spare)
	if c.session < min(natural, jobsMinSessionWidth) {
		c.session = 0
	}
	if rowWidth-c.fixedWidth() < jobsMinDescWidth {
		c.id = 0
	}
	if rowWidth-c.fixedWidth() < jobsMinDescWidth {
		c.tags = 0
	}
	return c
}

// jobRow dựng một dòng: biểu tượng trạng thái, ID (khớp với `agent-cli jobs`
// và dòng "Job ... PID" trong chat), thời gian chạy (tính lúc vẽ), nhãn, mô
// tả và session chủ. Dòng đang chọn đi qua style chọn bằng chữ trơn:
// chữ có màu riêng sẽ chèn mã reset làm mất nền chọn ở phần còn lại của dòng.
func (d *Jobs) jobRow(j shell.JobInfo, selected bool, cols jobsColumns, rowWidth int) string {
	t := d.com.Styles
	icon, iconStyle := d.jobIcon(j)
	elapsed := padLeftCell(formatJobElapsed(j.Elapsed(d.now())), jobsElapsedWidth)
	label := fitCell(jobLabel(j), rowWidth-cols.fixedWidth())
	session := fitCell(d.sessionLabel(j.SessionID), cols.session)

	var b strings.Builder
	if selected {
		b.WriteString(icon + " ")
		if cols.id > 0 {
			b.WriteString(fitCell(j.ID, cols.id) + " ")
		}
		b.WriteString(elapsed + " ")
		if cols.tags > 0 {
			b.WriteString(fitCell(strings.Join(d.tagsOf(j), " "), cols.tags) + " ")
		}
		b.WriteString(label)
		if cols.session > 0 {
			b.WriteString("  " + session)
		}
		return t.Dialog.SelectedItem.Render(ansi.Truncate(b.String(), rowWidth, ""))
	}

	muted := d.mutedStyle()
	b.WriteString(iconStyle.Render(icon) + " ")
	if cols.id > 0 {
		b.WriteString(muted.Render(fitCell(j.ID, cols.id)) + " ")
	}
	b.WriteString(muted.Render(elapsed) + " ")
	if cols.tags > 0 {
		b.WriteString(d.styledTags(j, cols.tags) + " ")
	}
	b.WriteString(t.Dialog.NormalItem.UnsetPadding().Render(label))
	if cols.session > 0 {
		b.WriteString("  " + muted.Render(session))
	}
	return lipgloss.NewStyle().Padding(0, 1).Render(ansi.Truncate(b.String(), rowWidth, ""))
}

// styledTags tô màu từng nhãn rồi đệm cho đủ width ô: bg màu hành động, mã
// thoát đỏ, killed và stopping nhạt.
func (d *Jobs) styledTags(j shell.JobInfo, width int) string {
	t := d.com.Styles
	tags := d.tagsOf(j)
	parts := make([]string, len(tags))
	for i, tag := range tags {
		style := t.Tool.JobAction
		switch {
		case tag == "killed" || tag == "stopping":
			style = t.Tool.StateCancelled
		case tag == "failed" || strings.HasPrefix(tag, "exit "):
			style = t.Tool.JobIconError
		}
		parts[i] = style.Render(tag)
	}
	out := ansi.Truncate(strings.Join(parts, " "), width, "…")
	return out + strings.Repeat(" ", max(width-ansi.StringWidth(out), 0))
}

// --- Câu hỏi dừng job ---

// promptJob là job mà câu hỏi dừng job nhắm tới.
func (d *Jobs) promptJob() shell.JobInfo {
	if i := d.indexOf(d.confirm.jobID); i >= 0 {
		return d.jobs[i]
	}
	return d.view.job
}

// stopPrompt dựng câu hỏi và hai nút; nút No được chọn sẵn.
func (d *Jobs) stopPrompt(innerWidth int) string {
	t := d.com.Styles
	c := d.confirm
	textW := max(0, innerWidth-2)

	question := t.Dialog.Sessions.DeletingMessage.Padding(0, 1).
		Render(fmt.Sprintf("Stop job %s?", c.jobID))
	detail := t.Dialog.SecondaryText.Render(ansi.Truncate(jobLabel(d.promptJob()), textW, "…"))

	no := common.Button(t, common.ButtonOpts{Text: "No, keep running", UnderlineIndex: -1, Selected: !c.yes})
	yes := t.Button.Blurred
	if c.yes {
		yes = t.Button.Negative
	}
	stop := yes.Padding(0, 2).Render("Yes, stop it")
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, no, "  ", stop)
	if lipgloss.Width(buttons) > textW {
		buttons = lipgloss.JoinVertical(lipgloss.Left, no, stop)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		question, detail, "", lipgloss.NewStyle().Padding(0, 1).Render(buttons))
}

// --- Khung xem output ---

func (d *Jobs) outputView(area uv.Rectangle) string {
	t := d.com.Styles
	width := max(0, min(jobsOutputMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	build := func(rows int) string {
		rc := d.newRenderContext(width)
		rc.Title = "Job " + d.view.job.ID
		rc.TitleInfo = d.outputTitleInfo()
		if d.confirm != nil {
			rc.AddPart(d.stopPrompt(innerWidth))
			rc.AddPart(" ")
		}
		rc.AddPart(d.outputHeader(innerWidth))
		rc.AddPart(" ")
		rc.AddPart(d.outputBody(innerWidth, rows))
		rc.AddPart(" ")
		rc.Help = renderDialogHelp(t, &d.help, d, innerWidth)
		return rc.Render()
	}
	return fitHeight(build, area.Dy(), min(max(area.Dy()-8, 1), jobsOutputMaxRows))
}

// outputTitleInfo là trạng thái và thời gian chạy của job đang xem.
func (d *Jobs) outputTitleInfo() string {
	j := d.view.job
	if d.view.gone {
		return " " + d.mutedStyle().Render("no longer listed")
	}
	icon, style := d.jobIcon(j)
	elapsed := formatJobElapsed(j.Elapsed(d.now()))
	return " " + style.Render(icon) + d.mutedStyle().Render(" "+jobStatusWord(j.Status)+" "+elapsed)
}

// outputHeader gồm hai dòng: lệnh, và mô tả cùng session cùng nhãn; bên phải
// dòng sau cho biết khung xem có đang bám đuôi không.
func (d *Jobs) outputHeader(innerWidth int) string {
	j := d.view.job
	textW := max(0, innerWidth-2)
	muted := d.mutedStyle()

	command := "$ " + joinFields(common.StripBashDisplayPrefix(strings.TrimSpace(j.Command), j.WorkingDir))
	line1 := muted.Render(ansi.Truncate(command, textW, "…"))

	var meta []string
	if desc := joinFields(j.Description); desc != "" {
		meta = append(meta, desc)
	}
	meta = append(meta, d.sessionLabel(j.SessionID))
	meta = append(meta, d.tagsOf(j)...)
	left := strings.Join(meta, " · ")

	right := ""
	if d.view.live() {
		right = "following"
		if !d.view.follow {
			right = "paused (end to follow)"
		}
	}
	room := textW - ansi.StringWidth(right) - 1
	if right == "" || room < 8 {
		room, right = textW, ""
	}
	left = ansi.Truncate(left, room, "…")
	line2 := muted.Render(left)
	if right != "" {
		line2 += strings.Repeat(" ", max(textW-ansi.StringWidth(left)-ansi.StringWidth(right), 1)) + muted.Render(right)
	}
	return lipgloss.NewStyle().Padding(0, 1).Render(line1 + "\n" + line2)
}

// outputBody vẽ rows dòng của cửa sổ output. Chỉ các dòng hiện ra mới bị cắt
// theo chiều ngang, nên chi phí mỗi khung hình không phụ thuộc độ dài log.
func (d *Jobs) outputBody(innerWidth, rows int) string {
	t := d.com.Styles
	v := &d.view
	v.rows = rows
	muted := t.Dialog.SecondaryText

	switch {
	case v.err != nil && len(v.lines) == 0:
		return d.errorLine("Could not read the output: "+v.err.Error(), innerWidth)
	case !v.loaded:
		return muted.Render("Loading output…")
	case len(v.lines) == 0 && v.live():
		return muted.Render("(no output yet)")
	case len(v.lines) == 0:
		return muted.Render("(no output)")
	}

	textW := max(0, innerWidth-2)
	bar := len(v.lines) > rows
	if bar {
		textW-- // Một cột cho thanh cuộn.
	}
	v.width = textW
	top := v.effectiveTop()
	end := min(top+rows, len(v.lines))
	out := make([]string, 0, end-top)
	for _, line := range v.lines[top:end] {
		seg := ansi.Cut(line, v.left, v.left+textW)
		pad := strings.Repeat(" ", max(textW-ansi.StringWidth(seg), 0))
		if strings.IndexByte(seg, 0x1b) >= 0 {
			seg += "\x1b[m" // Màu của dòng này không được tràn sang dòng sau.
		}
		// Đệm đủ bề rộng, kể cả một ô padding bên phải như các dòng của danh
		// sách, để thanh cuộn nằm sát mép phải dù dòng ngắn.
		out = append(out, " "+seg+pad+" ")
	}
	body := strings.Join(out, "\n")
	if bar {
		body = joinScrollbar(t, body, len(out), len(v.lines), rows, top)
	}
	if v.err != nil {
		body += "\n" + d.errorLine("Refresh failed: "+v.err.Error(), innerWidth)
	}
	return body
}
