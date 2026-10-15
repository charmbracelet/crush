package dialog

// Hộp thoại Jobs (alt+j): các lệnh bash mà agent đã chạy ở workspace này,
// đang chạy hoặc vừa xong, thuộc session nào, từ lúc nào, kết quả ra sao; xem
// output và dừng job đang chạy.
//
// Quy tắc của vùng mã UI (internal/ui/AGENTS.md) áp dụng chặt ở đây vì ở chế
// độ client/server mỗi lời gọi workspace là một vòng HTTP:
//   - HandleMsg và Draw không làm IO. Mọi lời gọi ListJobs, JobOutput,
//     KillJob nằm trong tea.Cmd; closure chỉ giữ biến cục bộ (workspace,
//     ID), không giữ con trỏ hộp thoại;
//   - kết quả về hộp thoại bằng message, và chỉ HandleMsg đổi trạng thái;
//   - thời gian chạy tính lúc vẽ từ StartedAt/FinishedAt nên nhích mỗi giây
//     dù dữ liệu không đổi.
//
// Vòng làm tươi: một nhịp tea.Tick mỗi giây chạy suốt lúc hộp thoại mở. Mỗi
// nhịp nạp lại danh sách (và output đang xem nếu job còn chạy) khi lần nạp
// trước đã xong, nên một server chậm kéo dài chu kỳ chứ không chồng yêu cầu.
// Message mang số thứ tự của hộp thoại đã sinh ra nó và đi qua TargetedMsg:
// vòng lặp sống tiếp khi có hộp thoại khác mở đè, và tự tắt khi hộp thoại này
// đóng; một hộp thoại mở lại sau đó không nhận nhịp cũ.

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync/atomic"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
)

const (
	// JobsID is the identifier for the jobs dialog.
	JobsID = "jobs"

	// jobsRefreshInterval là nhịp làm tươi khi hộp thoại đang mở.
	jobsRefreshInterval = time.Second
	// jobsRequestTimeout chặn trên mỗi lời gọi workspace, để một server treo
	// không làm cờ "đang nạp" kẹt mãi.
	jobsRequestTimeout = 5 * time.Second
)

// jobsInstances đánh số mỗi hộp thoại Jobs được mở.
var jobsInstances atomic.Uint64

// jobsMsg là phần chung của mọi message mà Cmd của hộp thoại Jobs gửi về. Nó
// mang số thứ tự của hộp thoại đã sinh ra Cmd, và là TargetedMsg để Overlay
// giao đúng hộp thoại Jobs dù nó không ở trên cùng.
type jobsMsg struct{ instance uint64 }

// TargetDialogID implements [TargetedMsg].
func (jobsMsg) TargetDialogID() string { return JobsID }

type (
	// jobsLoadedMsg là kết quả ListJobs.
	jobsLoadedMsg struct {
		jobsMsg
		jobs []shell.JobInfo
		err  error
	}
	// jobsTickMsg là nhịp làm tươi.
	jobsTickMsg struct{ jobsMsg }
	// jobsOutputMsg là kết quả JobOutput đã xử lý thành dòng.
	jobsOutputMsg struct {
		jobsMsg
		view  uint64 // lần mở khung xem sinh ra yêu cầu
		jobID string
		// unchanged: output thô giống lần trước, khỏi thay dòng.
		unchanged bool
		raw       string
		lines     []string
		// longest là độ dài (byte) dòng dài nhất, tính trong Cmd để Update
		// không phải duyệt hết các dòng.
		longest int
		err     error
	}
	// jobsStopMsg là kết quả KillJob.
	jobsStopMsg struct {
		jobsMsg
		jobID string
		err   error
	}
)

// jobsMode là màn đang hiện trong hộp thoại.
type jobsMode uint8

const (
	jobsModeList jobsMode = iota
	jobsModeOutput
)

// jobsStopPrompt là câu hỏi xác nhận trước khi dừng một job. Mặc định là No:
// chỉ ←/→ rồi Enter mới dừng, không phím chữ nào dừng được.
type jobsStopPrompt struct {
	jobID string
	yes   bool
}

// jobsOutputView là trạng thái khung xem output của một job.
type jobsOutputView struct {
	// seq phân biệt các lần mở khung xem: kết quả của lần trước về muộn bị
	// bỏ.
	seq uint64
	// job là ảnh chụp job đang xem, làm tươi theo mỗi lần nạp danh sách.
	job     shell.JobInfo
	raw     string
	lines   []string
	longest int

	loaded   bool
	inFlight bool
	err      error
	// fetchedRunning: lần nạp gần nhất bắt đầu khi job còn chạy. Job vừa xong
	// thì nạp thêm đúng một lần để lấy phần output cuối.
	fetchedRunning bool
	// gone: lần nạp danh sách gần nhất không còn thấy job (server khởi động
	// lại, lịch sử đã cắt). Trạng thái "đang chạy" cuối cùng không còn đáng
	// tin, nên khung xem không hiện nó như đang chạy.
	gone bool

	top, left int
	// follow: bám đuôi như tail -f, tắt khi người dùng cuộn lên.
	follow bool
	// rows và width là cỡ cửa sổ chữ ở lần vẽ gần nhất, dùng cho cuộn theo
	// trang và chặn cuộn ngang.
	rows, width int
}

// jobsKeyMap là phím của hộp thoại. Không có ô nhập nên phím chữ dùng để điều
// hướng; riêng việc dừng job thì không phím chữ nào làm được.
type jobsKeyMap struct {
	Up, Down, PageUp, PageDown, Home, End key.Binding
	Left, Right                           key.Binding
	Open                                  key.Binding
	Stop                                  key.Binding
	Close, Back                           key.Binding

	// Hộp xác nhận: chỉ phím mũi tên đổi nút (Tab, h, l không đổi), Enter làm
	// theo nút đang chọn, Esc/n giữ job chạy tiếp.
	No, Yes, Select, Keep key.Binding

	// Chỉ để hiện gợi ý.
	ChooseJob, ChooseButton, Scroll, Page, Pan key.Binding
}

func newJobsKeyMap() jobsKeyMap {
	return jobsKeyMap{
		Up:       key.NewBinding(key.WithKeys("up", "k", "ctrl+p")),
		Down:     key.NewBinding(key.WithKeys("down", "j", "ctrl+n")),
		PageUp:   key.NewBinding(key.WithKeys("pgup", "b")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "f", "space")),
		Home:     key.NewBinding(key.WithKeys("home", "g")),
		End:      key.NewBinding(key.WithKeys("end", "G")),
		Left:     key.NewBinding(key.WithKeys("left", "h")),
		Right:    key.NewBinding(key.WithKeys("right", "l")),
		Open: key.NewBinding(
			key.WithKeys("enter", "ctrl+y"),
			key.WithHelp("enter", "output"),
		),
		Stop: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("ctrl+k", "stop"),
		),
		Close: CloseKey,
		Back: key.NewBinding(
			key.WithKeys("esc", "alt+esc"),
			key.WithHelp("esc", "back"),
		),

		No:  key.NewBinding(key.WithKeys("left")),
		Yes: key.NewBinding(key.WithKeys("right")),
		Select: key.NewBinding(
			key.WithKeys("enter", "ctrl+y"),
			key.WithHelp("enter", "confirm"),
		),
		Keep: key.NewBinding(
			key.WithKeys("esc", "alt+esc", "n", "N"),
			key.WithHelp("esc", "keep running"),
		),

		// Một binding không có phím bị coi là tắt và biến mất khỏi dòng gợi
		// ý, nên các binding chỉ để hiện gợi ý vẫn khai phím.
		ChooseJob:    key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "choose")),
		ChooseButton: key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←/→", "choose")),
		Scroll:       key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "scroll")),
		Page:         key.NewBinding(key.WithKeys("pgup", "pgdown"), key.WithHelp("pgup/pgdn", "page")),
		Pan:          key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←/→", "pan")),
	}
}

// Jobs là hộp thoại liệt kê job của workspace.
type Jobs struct {
	com      *common.Common
	instance uint64
	// now và interval để test thay; mặc định là time.Now và nhịp 1 giây.
	now      func() time.Time
	interval time.Duration
	help     help.Model
	keyMap   jobsKeyMap

	// titles ánh xạ session ID sang tiêu đề, lấy từ danh sách session UI đã
	// nạp sẵn. Session lạ hiện bằng vài ký tự đầu của ID.
	titles map[string]string

	// jobs đã chuẩn hoá (xem normalizeJobs). loaded: đã có kết quả nạp đầu
	// tiên, đúng hay sai. loadErr: lần nạp gần nhất lỗi.
	jobs         []shell.JobInfo
	loaded       bool
	loadErr      error
	listInFlight bool
	// selectedID theo ID, không theo chỉ số, để lựa chọn đứng yên khi danh
	// sách đổi thứ tự.
	selectedID string
	listTop    int
	listRows   int

	mode    jobsMode
	view    jobsOutputView
	viewSeq uint64

	confirm *jobsStopPrompt
	// stopping: job đã gửi lệnh dừng mà chưa có kết quả.
	stopping map[string]bool
}

var _ Dialog = (*Jobs)(nil)

// NewJobs tạo hộp thoại Jobs. Không làm IO: Cmd trả về nạp danh sách lần đầu
// và bắt đầu vòng làm tươi. titles là bản chụp tiêu đề session; nil được.
func NewJobs(com *common.Common, titles map[string]string) (*Jobs, tea.Cmd) {
	return newJobs(com, titles, jobsRefreshInterval, time.Now)
}

// newJobs nhận nhịp làm tươi và đồng hồ từ ngoài để test không phải chờ một
// giây thật và dựng được thời gian chạy cố định.
func newJobs(com *common.Common, titles map[string]string, interval time.Duration, now func() time.Time) (*Jobs, tea.Cmd) {
	d := &Jobs{
		com:      com,
		instance: jobsInstances.Add(1),
		now:      now,
		interval: interval,
		titles:   maps.Clone(titles),
		listRows: 8,
		keyMap:   newJobsKeyMap(),
		stopping: map[string]bool{},
	}
	d.help = help.New()
	d.help.Styles = com.Styles.DialogHelpStyles()
	d.listInFlight = true
	return d, tea.Batch(d.loadJobsCmd(), d.tickCmd())
}

// ID implements Dialog.
func (d *Jobs) ID() string { return JobsID }

// SetSessionTitles thay bản chụp tiêu đề session, khi UI nạp lại danh sách.
func (d *Jobs) SetSessionTitles(titles map[string]string) {
	d.titles = maps.Clone(titles)
}

// HandleMsg implements Dialog.
func (d *Jobs) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case jobsLoadedMsg:
		return d.onJobsLoaded(msg)
	case jobsTickMsg:
		return d.onTick(msg)
	case jobsOutputMsg:
		return d.onOutput(msg)
	case jobsStopMsg:
		return d.onStopped(msg)
	case common.CoalescedWheelMsg:
		d.onWheel(msg)
	case tea.KeyPressMsg:
		return d.onKey(msg)
	}
	return nil
}

// --- Cmd: mọi lời gọi workspace nằm ở đây ---

// tickCmd hẹn nhịp làm tươi kế tiếp.
func (d *Jobs) tickCmd() tea.Cmd {
	instance := d.instance
	return tea.Tick(d.interval, func(time.Time) tea.Msg {
		return jobsTickMsg{jobsMsg{instance}}
	})
}

// loadJobsCmd nạp danh sách job.
func (d *Jobs) loadJobsCmd() tea.Cmd {
	ws, instance := d.com.Workspace, d.instance
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), jobsRequestTimeout)
		defer cancel()
		jobs, err := ws.ListJobs(ctx)
		return jobsLoadedMsg{jobsMsg: jobsMsg{instance}, jobs: jobs, err: err}
	}
}

// outputCmd nạp output của job đang xem và biến nó thành dòng ngay trong Cmd,
// vì output có thể tới vài MB. Output giống lần trước thì khỏi xử lý lại.
func (d *Jobs) outputCmd() tea.Cmd {
	ws, instance := d.com.Workspace, d.instance
	view, jobID, prev := d.view.seq, d.view.job.ID, d.view.raw
	palette := d.com.Styles.ANSI
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), jobsRequestTimeout)
		defer cancel()
		msg := jobsOutputMsg{jobsMsg: jobsMsg{instance}, view: view, jobID: jobID}
		raw, err := ws.JobOutput(ctx, jobID)
		switch {
		case err != nil:
			msg.err = err
		case raw == prev:
			msg.unchanged = true
		default:
			msg.raw, msg.lines = raw, prepareJobOutput(raw, palette)
			msg.longest = longestLine(msg.lines)
		}
		return msg
	}
}

// stopCmd dừng job. Kết quả đi hai đường: jobsStopMsg về hộp thoại để bỏ cờ
// "stopping" và nạp lại danh sách, và một thông báo lên thanh trạng thái
// không gửi đích danh. Nhờ vậy người dùng đóng hộp thoại ngay sau khi xác
// nhận vẫn thấy job có dừng được hay không; thông báo của Overlay cho hộp
// thoại đã đóng thì bị bỏ.
func (d *Jobs) stopCmd(jobID string) tea.Cmd {
	ws, instance := d.com.Workspace, d.instance
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), jobsRequestTimeout)
		defer cancel()
		err := ws.KillJob(ctx, jobID)
		report := util.ReportInfo(fmt.Sprintf("Stopped job %s", jobID))
		if err != nil {
			report = util.ReportError(fmt.Errorf("could not stop job %s: %w", jobID, err))
		}
		return tea.BatchMsg{
			util.CmdHandler(jobsStopMsg{jobsMsg: jobsMsg{instance}, jobID: jobID, err: err}),
			report,
		}
	}
}

// --- Làm tươi ---

// refreshListCmd bắt đầu một lần nạp danh sách, trừ khi lần trước chưa xong.
func (d *Jobs) refreshListCmd() tea.Cmd {
	if d.listInFlight {
		return nil
	}
	d.listInFlight = true
	return d.loadJobsCmd()
}

// refreshOutputCmd bắt đầu một lần nạp output của job đang xem, trừ khi lần
// trước chưa xong. Job đang chạy thì nạp mỗi nhịp; job đã xong chỉ nạp thêm
// một lần nếu lần nạp gần nhất còn thấy nó chạy, để lấy phần output cuối.
func (d *Jobs) refreshOutputCmd() tea.Cmd {
	v := &d.view
	if v.inFlight {
		return nil
	}
	running := v.live()
	if !running && !v.fetchedRunning {
		return nil
	}
	v.inFlight = true
	v.fetchedRunning = running
	return d.outputCmd()
}

func (d *Jobs) onTick(msg jobsTickMsg) Action {
	if msg.instance != d.instance {
		return nil
	}
	cmds := []tea.Cmd{d.tickCmd(), d.refreshListCmd()}
	if d.mode == jobsModeOutput {
		cmds = append(cmds, d.refreshOutputCmd())
	}
	return ActionCmd{tea.Batch(cmds...)}
}

func (d *Jobs) onJobsLoaded(msg jobsLoadedMsg) Action {
	if msg.instance != d.instance {
		return nil
	}
	d.listInFlight = false
	d.loaded = true
	var cmds []tea.Cmd
	if msg.err != nil {
		// Chỉ báo ở lần lỗi đầu của một chuỗi lỗi: nhịp nào cũng báo thì
		// thanh trạng thái bị nháy mỗi giây.
		if d.loadErr == nil {
			cmds = append(cmds, util.ReportWarn(fmt.Sprintf("Could not list jobs: %v", msg.err)))
		}
		d.loadErr = msg.err
	} else {
		d.loadErr = nil
		d.setJobs(msg.jobs)
		if cmd := d.syncAfterLoad(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return ActionCmd{tea.Batch(cmds...)}
}

// setJobs thay danh sách và giữ lựa chọn theo ID. Job đang chọn biến mất thì
// lựa chọn ở lại gần vị trí cũ.
func (d *Jobs) setJobs(jobs []shell.JobInfo) {
	prev := d.selectedIndex()
	d.jobs = normalizeJobs(jobs)
	if d.selectedIndex() >= 0 {
		return
	}
	if len(d.jobs) == 0 {
		d.selectedID = ""
		return
	}
	d.selectedID = d.jobs[min(max(prev, 0), len(d.jobs)-1)].ID
}

// syncAfterLoad cập nhật những thứ phụ thuộc danh sách vừa về: job đang xem,
// câu hỏi dừng job đã không còn đúng.
func (d *Jobs) syncAfterLoad() tea.Cmd {
	if d.confirm != nil {
		// Job đã xong hoặc biến mất thì không còn gì để dừng.
		if i := d.indexOf(d.confirm.jobID); i < 0 || d.jobs[i].Status != shell.JobRunning {
			d.confirm = nil
		}
	}
	if d.mode != jobsModeOutput {
		return nil
	}
	i := d.indexOf(d.view.job.ID)
	d.view.gone = i < 0
	if i >= 0 {
		d.view.job = d.jobs[i]
	}
	// Job vừa xong hoặc vừa biến mất: lấy ngay phần output cuối, khỏi đợi
	// nhịp sau.
	if d.view.fetchedRunning && !d.view.live() {
		return d.refreshOutputCmd()
	}
	return nil
}

func (d *Jobs) onOutput(msg jobsOutputMsg) Action {
	v := &d.view
	if msg.instance != d.instance || d.mode != jobsModeOutput || msg.view != v.seq {
		return nil
	}
	v.inFlight = false
	v.loaded = true
	if msg.err != nil {
		first := v.err == nil
		v.err = msg.err
		if first {
			return ActionCmd{util.ReportWarn(fmt.Sprintf("Could not read the output of job %s: %v", v.job.ID, msg.err))}
		}
		return nil
	}
	v.err = nil
	if !msg.unchanged {
		v.raw, v.lines, v.longest = msg.raw, msg.lines, msg.longest
	}
	return nil
}

// onStopped nhận kết quả dừng job: bỏ cờ "stopping" và nạp lại danh sách ngay,
// khỏi đợi nhịp sau. Thông báo cho người dùng đã đi riêng (xem stopCmd).
func (d *Jobs) onStopped(msg jobsStopMsg) Action {
	if msg.instance != d.instance {
		return nil
	}
	delete(d.stopping, msg.jobID)
	return ActionCmd{d.refreshListCmd()}
}

// --- Danh sách và lựa chọn ---

func (d *Jobs) indexOf(id string) int {
	if id == "" {
		return -1
	}
	return slices.IndexFunc(d.jobs, func(j shell.JobInfo) bool { return j.ID == id })
}

func (d *Jobs) selectedIndex() int { return d.indexOf(d.selectedID) }

func (d *Jobs) selectedJob() (shell.JobInfo, bool) {
	i := d.selectedIndex()
	if i < 0 {
		return shell.JobInfo{}, false
	}
	return d.jobs[i], true
}

// moveSelection dời lựa chọn delta dòng, dừng ở hai đầu.
func (d *Jobs) moveSelection(delta int) {
	if len(d.jobs) == 0 {
		return
	}
	i := d.selectedIndex()
	if i < 0 {
		i = 0
	} else {
		i = min(max(i+delta, 0), len(d.jobs)-1)
	}
	d.selectedID = d.jobs[i].ID
}

// sessionLabel là nhãn session chủ của job: tiêu đề nếu biết, không thì vài
// ký tự đầu của ID.
func (d *Jobs) sessionLabel(sessionID string) string {
	if sessionID == "" {
		return "-"
	}
	if title := d.titles[sessionID]; title != "" {
		return joinFields(title)
	}
	return shortSessionID(sessionID)
}

// --- Phím ---

func (d *Jobs) onKey(msg tea.KeyPressMsg) Action {
	switch {
	case d.confirm != nil:
		return d.onConfirmKey(msg)
	case d.mode == jobsModeOutput:
		return d.onOutputKey(msg)
	}
	return d.onListKey(msg)
}

func (d *Jobs) onListKey(msg tea.KeyPressMsg) Action {
	km := &d.keyMap
	page := max(d.listRows-1, 1)
	switch {
	case key.Matches(msg, km.Close):
		return ActionClose{}
	case key.Matches(msg, km.Up):
		d.moveSelection(-1)
	case key.Matches(msg, km.Down):
		d.moveSelection(1)
	case key.Matches(msg, km.PageUp):
		d.moveSelection(-page)
	case key.Matches(msg, km.PageDown):
		d.moveSelection(page)
	case key.Matches(msg, km.Home):
		d.moveSelection(-len(d.jobs))
	case key.Matches(msg, km.End):
		d.moveSelection(len(d.jobs))
	case key.Matches(msg, km.Open):
		return d.openOutput()
	case key.Matches(msg, km.Stop):
		return d.askStop()
	}
	return nil
}

func (d *Jobs) onOutputKey(msg tea.KeyPressMsg) Action {
	km, v := &d.keyMap, &d.view
	page := max(v.rows-1, 1)
	switch {
	case key.Matches(msg, km.Back):
		d.leaveOutput()
	case key.Matches(msg, km.Up):
		v.scrollBy(-1)
	case key.Matches(msg, km.Down):
		v.scrollBy(1)
	case key.Matches(msg, km.PageUp):
		v.scrollBy(-page)
	case key.Matches(msg, km.PageDown):
		v.scrollBy(page)
	case key.Matches(msg, km.Home):
		v.scrollTo(0)
	case key.Matches(msg, km.End):
		v.follow = true
	case key.Matches(msg, km.Left):
		v.pan(-jobsOutputPanStep)
	case key.Matches(msg, km.Right):
		v.pan(jobsOutputPanStep)
	case key.Matches(msg, km.Stop):
		return d.askStop()
	}
	return nil
}

// onConfirmKey xử lý câu hỏi dừng job. Mặc định là No và không phím chữ nào
// chọn Yes: như hộp thoại xin quyền, thứ nguy hiểm đòi một thao tác có chủ ý
// (mũi tên sang Yes rồi Enter), vì hộp thoại có thể bật lên giữa lúc người
// dùng đang gõ.
func (d *Jobs) onConfirmKey(msg tea.KeyPressMsg) Action {
	km, c := &d.keyMap, d.confirm
	switch {
	case key.Matches(msg, km.Keep):
		d.confirm = nil
	case key.Matches(msg, km.No):
		c.yes = false
	case key.Matches(msg, km.Yes):
		c.yes = true
	case key.Matches(msg, km.Select):
		d.confirm = nil
		if c.yes {
			return d.stop(c.jobID)
		}
	}
	return nil
}

func (d *Jobs) onWheel(msg common.CoalescedWheelMsg) {
	if d.confirm != nil {
		return
	}
	dy, dx := int(msg.DeltaY), int(msg.DeltaX)
	if d.mode == jobsModeOutput {
		d.view.scrollBy(dy)
		d.view.pan(dx)
		return
	}
	d.moveSelection(dy)
}

// --- Hành động ---

// openOutput mở khung xem output của job đang chọn.
func (d *Jobs) openOutput() Action {
	job, ok := d.selectedJob()
	if !ok {
		return nil
	}
	d.viewSeq++
	d.mode = jobsModeOutput
	d.view = jobsOutputView{
		seq:            d.viewSeq,
		job:            job,
		follow:         true,
		inFlight:       true,
		fetchedRunning: job.Status == shell.JobRunning,
	}
	return ActionCmd{d.outputCmd()}
}

// leaveOutput về danh sách, bỏ khung xem.
func (d *Jobs) leaveOutput() {
	d.mode = jobsModeList
	d.view = jobsOutputView{}
}

// stopTarget là job mà phím dừng nhắm tới: job đang xem, hoặc job đang chọn.
func (d *Jobs) stopTarget() (shell.JobInfo, bool) {
	if d.mode == jobsModeOutput {
		return d.view.job, d.view.job.ID != ""
	}
	return d.selectedJob()
}

// askStop mở câu hỏi xác nhận dừng job. Chưa dừng gì cả.
func (d *Jobs) askStop() Action {
	job, ok := d.stopTarget()
	if !ok {
		return nil
	}
	switch {
	case job.Status != shell.JobRunning || (d.mode == jobsModeOutput && d.view.gone):
		return ActionCmd{util.ReportInfo(fmt.Sprintf("Job %s is not running", job.ID))}
	case d.stopping[job.ID]:
		return ActionCmd{util.ReportInfo(fmt.Sprintf("Job %s is already being stopped", job.ID))}
	}
	d.confirm = &jobsStopPrompt{jobID: job.ID}
	return nil
}

// stop gửi lệnh dừng job đã được xác nhận.
func (d *Jobs) stop(jobID string) Action {
	d.stopping[jobID] = true
	return ActionCmd{d.stopCmd(jobID)}
}

// --- Khung xem output ---

const jobsOutputPanStep = 8

// live: job còn chạy theo thông tin mới nhất mà workspace báo.
func (v *jobsOutputView) live() bool {
	return v.job.Status == shell.JobRunning && !v.gone
}

func (v *jobsOutputView) maxTop() int { return max(0, len(v.lines)-max(v.rows, 1)) }

// effectiveTop là dòng đầu đang hiện: đuôi khi bám đuôi.
func (v *jobsOutputView) effectiveTop() int {
	if v.follow {
		return v.maxTop()
	}
	return min(v.top, v.maxTop())
}

// scrollTo đặt dòng đầu. Chạm đáy thì bám đuôi trở lại, cuộn lên thì dừng
// bám.
func (v *jobsOutputView) scrollTo(top int) {
	maxTop := v.maxTop()
	v.top = min(max(top, 0), maxTop)
	v.follow = v.top >= maxTop
}

func (v *jobsOutputView) scrollBy(n int) {
	if n != 0 {
		v.scrollTo(v.effectiveTop() + n)
	}
}

// pan cuộn ngang, dừng khi cuối dòng dài nhất chạm mép phải.
func (v *jobsOutputView) pan(n int) {
	v.left = min(max(v.left+n, 0), max(v.longest-v.width, 0))
}

// --- help.KeyMap ---

// ShortHelp implements [help.KeyMap].
func (d *Jobs) ShortHelp() []key.Binding {
	km := &d.keyMap
	stop := km.Stop
	switch {
	case d.confirm != nil:
		return []key.Binding{km.ChooseButton, km.Select, km.Keep}
	case d.mode == jobsModeOutput:
		stop.SetEnabled(d.view.live())
		return []key.Binding{km.Scroll, km.Page, km.Pan, stop, km.Back}
	}
	job, ok := d.selectedJob()
	stop.SetEnabled(ok && job.Status == shell.JobRunning)
	open := km.Open
	open.SetEnabled(ok)
	return []key.Binding{km.ChooseJob, open, stop, km.Close}
}

// FullHelp implements [help.KeyMap].
func (d *Jobs) FullHelp() [][]key.Binding {
	return [][]key.Binding{d.ShortHelp()}
}
