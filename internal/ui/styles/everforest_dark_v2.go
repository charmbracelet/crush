package styles

import (
	"image/color"

	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// everforestDarkV2Name là khoá của theme trong options.tui.active_theme (và
// trong hộp thoại Themes). Đây là theme mặc định của fork (DefaultThemeName);
// theme cũ "charmtone-panther" không bị sửa, nên quay lại bản cũ chỉ cần chọn
// nó trong Themes (xem docs/agent-cli-local-setup.md, mục "Theme").
const everforestDarkV2Name = "luannt-theme-everforest-dark"

// everforestTokens là bộ semantic token của "Everforest Dark v2": mọi màu của
// theme đều đi ra từ đây (không có màu hex rải rác ở nơi khác trong file này).
// Tên token theo thiết kế gốc (bg.base, fg.muted, syntax.keyword...).
type everforestTokens struct {
	// Nền: base là nền chính; sunken tối hơn base (khối code, khối bung ra,
	// "... lines hidden"); raised sáng hơn base (inline code, panel); overlay
	// là lớp nổi (viền To-Do, nút chưa chọn, huy hiệu H1).
	bgBase, bgSunken, bgRaised, bgOverlay color.Color

	// Chữ: base là chữ thường; muted là đường dẫn, metadata, chữ phụ; subtle là
	// gợi ý, placeholder, số dòng, phím tắt.
	fgBase, fgMuted, fgSubtle color.Color

	// Viền và nhấn. accent chỉ dành cho thứ tương tác hoặc đang được chọn.
	border, borderFocus, accent color.Color

	// Trạng thái.
	success, warning, error, info color.Color

	// Diff. diffChange không có nơi dùng (diffview chỉ có dòng thêm/xoá/giữ);
	// giữ lại cho đủ bộ token và cho các chỗ "đã đổi" về sau.
	diffAdd, diffDelete, diffChange color.Color

	// Cú pháp (syntax highlighting). synType là chỗ thiếu trong thiết kế gốc
	// (chỉ có 6 token cú pháp): kiểu dữ liệu dùng ANSI cyan của chính bảng ANSI
	// bên dưới thay vì thêm màu mới.
	synKeyword, synFunction, synString, synNumber, synComment, synDefault, synType color.Color

	// Bảng ANSI 16 màu (remap output thô của lệnh shell, ví dụ chế độ bang).
	ansi [16]color.Color
}

// everforestV2 là bảng màu của theme. Đây là nguồn duy nhất; đổi màu ở đây.
var everforestV2 = everforestTokens{
	bgBase:    lipgloss.Color("#141619"),
	bgSunken:  lipgloss.Color("#0F1113"),
	bgRaised:  lipgloss.Color("#1B1E22"),
	bgOverlay: lipgloss.Color("#23272D"),

	// fgBase là fg.primary (chữ chính, tên thương hiệu); chữ thường trong khối
	// code (synDefault) là fg.normal, tối hơn một bậc.
	fgBase:   lipgloss.Color("#D0D2D1"),
	fgMuted:  lipgloss.Color("#929997"),
	fgSubtle: lipgloss.Color("#7D8585"),

	border:      lipgloss.Color("#30343B"),
	borderFocus: lipgloss.Color("#82AEB1"),
	accent:      lipgloss.Color("#82AEB1"),

	success: lipgloss.Color("#8FAF87"),
	warning: lipgloss.Color("#C2A66F"),
	error:   lipgloss.Color("#C77D7D"),
	info:    lipgloss.Color("#829CB5"),

	diffAdd:    lipgloss.Color("#8FAF87"),
	diffDelete: lipgloss.Color("#C77D7D"),
	diffChange: lipgloss.Color("#C2A66F"),

	synKeyword:  lipgloss.Color("#A392B2"),
	synFunction: lipgloss.Color("#82AEB1"),
	synString:   lipgloss.Color("#8FAF87"),
	synNumber:   lipgloss.Color("#C2A66F"),
	synComment:  lipgloss.Color("#7D8585"),
	synDefault:  lipgloss.Color("#B7BBB9"),
	synType:     lipgloss.Color("#82AEB1"),

	ansi: [16]color.Color{
		lipgloss.Color("#23272D"), // black
		lipgloss.Color("#C77D7D"), // red
		lipgloss.Color("#8FAF87"), // green
		lipgloss.Color("#C2A66F"), // yellow
		lipgloss.Color("#829CB5"), // blue
		lipgloss.Color("#A392B2"), // magenta
		lipgloss.Color("#82AEB1"), // cyan
		lipgloss.Color("#B7BBB9"), // white
		lipgloss.Color("#525959"), // bright black
		lipgloss.Color("#D28E8E"), // bright red
		lipgloss.Color("#9FBC98"), // bright green
		lipgloss.Color("#CFB583"), // bright yellow
		lipgloss.Color("#95AEC4"), // bright blue
		lipgloss.Color("#B3A3C0"), // bright magenta
		lipgloss.Color("#94BDC0"), // bright cyan
		lipgloss.Color("#D0D2D1"), // bright white
	},
}

// everforestDarkV2Opts trả về palette của theme ở dạng quickStyleOpts. Các token
// của quickStyle bị dùng cho nhiều vai trò khác nhau (ví dụ "primary" vừa là
// màu chọn vừa là màu logo), nên ở đây chỉ gán giá trị hợp lý nhất cho từng
// token; chỗ nào một token phục vụ sai vai trò thì
// [everforestDarkV2Overrides] sửa lại theo vai trò.
func everforestDarkV2Opts() quickStyleOpts {
	t := everforestV2
	return quickStyleOpts{
		// "Thương hiệu" của quickStyle đều quy về màu nhấn duy nhất.
		primary:   t.accent,
		secondary: t.accent,
		accent:    t.accent,
		keyword:   t.synKeyword,

		// fgSubtle của quickStyle là chữ thân của tin nhắn trợ lý; ở theme này chữ
		// thân là fg.base. fgMoreSubtle = muted, fgMostSubtle = subtle.
		fgBase:       t.fgBase,
		fgSubtle:     t.fgBase,
		fgMoreSubtle: t.fgMuted,
		fgMostSubtle: t.fgSubtle,

		// Chữ nằm trên nền màu nhấn/đỏ/vàng/xanh phải là chữ tối.
		onPrimary: t.bgBase,

		bgBase:         t.bgBase,
		bgLeastVisible: t.bgSunken,
		bgLessVisible:  t.bgRaised,
		bgMostVisible:  t.bgOverlay,

		separator: t.border,

		destructive:   t.error,
		error:         t.error,
		warningSubtle: t.warning,
		warning:       t.warning,
		attention:     t.warning,
		busy:          t.accent, // "đang chạy" là trạng thái hiện hành, không phải cảnh báo.
		// quickStyle dùng "info" cho tên tool, tên MCP, tiêu đề: đó là chữ hành động
		// nên giữ màu nhấn (một sắc nhấn duy nhất); token info chỉ dùng cho chẩn
		// đoán LSP dạng thông tin (xem everforestDarkV2Overrides).
		info:              t.accent,
		infoMoreSubtle:    t.accent,
		infoMostSubtle:    t.accent,
		success:           t.success,
		successMoreSubtle: t.success,
		successMostSubtle: t.success,
		yolo:              t.warning, // YOLO là chế độ cần thận trọng.
		plan:              t.accent,
		planMoreSubtle:    t.bgOverlay,

		// Diff: nền dòng thêm/xoá pha từ màu diff lên bg.sunken (cùng nền với khối
		// code), cùng tỉ lệ pha như các theme khác (xem deriveDiffColors).
		diffInsertFg:       t.diffAdd,
		diffInsertCodeBg:   blendTint(t.diffAdd, t.bgSunken, diffInsertCodeBlend),
		diffInsertGutterBg: blendTint(t.diffAdd, t.bgSunken, diffInsertGutterBlend),
		diffDeleteFg:       t.diffDelete,
		diffDeleteCodeBg:   blendTint(t.diffDelete, t.bgSunken, diffDeleteCodeBlend),
		diffDeleteGutterBg: blendTint(t.diffDelete, t.bgSunken, diffDeleteGutterBlend),

		// Nút: nút đang chọn dùng màu nhấn; nút chưa chọn dùng nền overlay. Nút
		// "inactive" (cửa sổ không có focus) sáng hơn nút chưa chọn một chút, như
		// các theme khác, nên lấy bright black của bảng ANSI.
		button:         t.accent,
		buttonSubtle:   t.bgOverlay,
		buttonInactive: t.ansi[8],
		buttonHovered:  t.ansi[12],

		ansiBlack:   t.ansi[0],
		ansiRed:     t.ansi[1],
		ansiGreen:   t.ansi[2],
		ansiYellow:  t.ansi[3],
		ansiBlue:    t.ansi[4],
		ansiMagenta: t.ansi[5],
		ansiCyan:    t.ansi[6],
		ansiWhite:   t.ansi[7],

		ansiBrightBlack:   t.ansi[8],
		ansiBrightRed:     t.ansi[9],
		ansiBrightGreen:   t.ansi[10],
		ansiBrightYellow:  t.ansi[11],
		ansiBrightBlue:    t.ansi[12],
		ansiBrightMagenta: t.ansi[13],
		ansiBrightCyan:    t.ansi[14],
		ansiBrightWhite:   t.ansi[15],
	}
}

// everforestDarkV2Overrides sửa các style mà mô hình token của quickStyle không
// diễn đạt được, để mỗi thành phần đúng vai trò: chữ thường fg.base, đường
// dẫn/metadata fg.muted, gợi ý/phím tắt fg.subtle, màu nhấn chỉ cho thứ đang
// tương tác, xanh lá chỉ cho thành công, đỏ chỉ cho lỗi thật, vàng chỉ cho
// cảnh báo. Chỉ đổi màu (và vài thuộc tính màu như underline/bold của syntax);
// không đụng bố cục, nội dung hay phím tắt.
func everforestDarkV2Overrides(s Styles) Styles {
	t := everforestV2

	// --- Chỉ báo "đang làm việc" (spinner, nhãn Thinking, thời gian) ------------
	// Dải màu của ký tự quay là trung tính; nhãn và thời gian là fg.muted.
	s.WorkingGradFromColor = t.fgSubtle
	s.WorkingGradToColor = t.fgMuted
	s.WorkingLabelColor = t.fgMuted
	s.WorkingTimerColor = t.fgMuted

	// --- Header và logo: bớt nổi, không gradient tím/xanh --------------------------
	// Tên "AGENT-CLI" một màu fg.primary, dải chéo chỉ còn là hoạ tiết nền (màu
	// border). Nhãn và phiên bản không còn được vẽ (bỏ 15/10); style của chúng
	// giữ cho theme người dùng kế thừa.
	s.Header.Charm = s.Header.Charm.Foreground(t.fgSubtle)
	s.Header.Diagonals = s.Header.Diagonals.Foreground(t.border)
	s.Header.HypercreditIcon = s.Header.HypercreditIcon.Foreground(t.fgMuted)
	s.Header.GitBranch = s.Header.GitBranch.Foreground(t.fgMuted)
	s.Header.LogoGradFromColor = t.fgBase
	s.Header.LogoGradToColor = t.fgBase
	s.CompactDetails.View = s.CompactDetails.View.BorderForeground(t.border)
	s.CompactDetails.Version = s.CompactDetails.Version.Foreground(t.fgSubtle)
	s.Logo.FieldColor = t.border
	s.Logo.TitleColorA = t.fgBase
	s.Logo.TitleColorB = t.fgBase
	s.Logo.CharmColor = t.fgSubtle
	s.Logo.VersionColor = t.fgSubtle
	s.Logo.SmallCharm = s.Logo.SmallCharm.Foreground(t.fgSubtle)
	s.Logo.SmallDiagonals = s.Logo.SmallDiagonals.Foreground(t.border)
	s.Logo.SmallGradFromColor = t.fgBase
	s.Logo.SmallGradToColor = t.fgBase

	// --- Markdown và cú pháp ----------------------------------------------------
	t.restyleMarkdown(&s.Markdown)
	t.restyleMarkdown(&s.PlanMarkdown)
	// Thẻ plan: tiêu đề H2-H5 là chữ thường (quickStyle tô info), inline code
	// vẫn in đậm như plan card vẫn làm nhưng không còn màu đỏ.
	for _, h := range []*ansi.StyleBlock{&s.PlanMarkdown.H2, &s.PlanMarkdown.H3, &s.PlanMarkdown.H4, &s.PlanMarkdown.H5} {
		h.Color = hex(t.fgBase)
	}
	// Chroma được chia sẻ giữa Markdown và PlanMarkdown (cùng con trỏ).
	t.restyleChroma(s.Markdown.CodeBlock.Chroma)

	// --- Khối code và khối bung ra: nền sunken --------------------------------------
	s.Tool.ContentCodeBg = t.bgSunken
	s.Tool.ContentCodeLine = s.Tool.ContentCodeLine.Background(t.bgSunken)
	s.Tool.ContentLineNumber = s.Tool.ContentLineNumber.Foreground(t.fgSubtle).Background(t.bgSunken)
	// "… (N lines hidden)": fg.subtle trên nền sunken, đúng nền của khối chứa nó.
	s.Tool.ContentTruncation = s.Tool.ContentTruncation.Foreground(t.fgSubtle).Background(t.bgSunken)
	s.Tool.ContentCodeTruncation = s.Tool.ContentCodeTruncation.Foreground(t.fgSubtle).Background(t.bgSunken)
	s.Tool.DiffTruncation = s.Tool.DiffTruncation.Foreground(t.fgSubtle).Background(t.bgSunken)
	s.Messages.ThinkingTruncationHint = lipgloss.NewStyle().Foreground(t.fgSubtle).Background(t.bgSunken)
	s.Messages.ShellTruncation = s.Messages.ShellTruncation.Foreground(t.fgSubtle)

	// --- Diff: toàn bộ khối nằm trên nền sunken -------------------------------------
	s.Diff.DividerLine.LineNumber = s.Diff.DividerLine.LineNumber.Foreground(t.fgSubtle)
	s.Diff.DividerLine.Code = s.Diff.DividerLine.Code.Foreground(t.fgSubtle)
	s.Diff.EqualLine.LineNumber = s.Diff.EqualLine.LineNumber.Foreground(t.fgSubtle).Background(t.bgSunken)
	s.Diff.EqualLine.Code = s.Diff.EqualLine.Code.Background(t.bgSunken)
	// Tên tệp trong tiêu đề diff: fg.muted.
	s.Diff.Filename.LineNumber = s.Diff.Filename.LineNumber.Foreground(t.fgMuted)
	s.Diff.Filename.Code = s.Diff.Filename.Code.Foreground(t.fgMuted)

	// --- Dòng tool call ------------------------------------------------------------------
	// Tên tool (hành động) giữ màu nhấn từ token info. Tham số chính là đường
	// dẫn/lệnh nên fg.muted; tên tham số fg.subtle.
	s.Tool.IconPending = s.Tool.IconPending.Foreground(t.fgMuted)
	s.Tool.ParamMain = s.Tool.ParamMain.Foreground(t.fgMuted)
	// Nhãn phụ trung tính thay vì ô xanh/xanh lá: chỉ trạng thái mới có màu.
	s.Tool.AgentTaskTag = s.Tool.AgentTaskTag.Background(t.bgOverlay).Foreground(t.fgBase)
	s.Tool.AgenticFetchPromptTag = s.Tool.AgenticFetchPromptTag.Background(t.bgOverlay).Foreground(t.fgBase)
	s.Tool.JobIconPending = s.Tool.JobIconPending.Foreground(t.fgMuted)
	s.Tool.JobAction = s.Tool.JobAction.Foreground(t.fgMuted)
	s.Tool.MCPToolName = s.Tool.MCPToolName.Foreground(t.fgMuted)
	s.Tool.MCPArrow = s.Tool.MCPArrow.Foreground(t.fgSubtle)
	s.Tool.ResourceLoadedText = s.Tool.ResourceLoadedText.Foreground(t.fgMuted)
	s.Tool.ResourceLoadedIndicator = s.Tool.ResourceLoadedIndicator.Foreground(t.fgSubtle)
	s.Tool.MediaType = s.Tool.MediaType.Foreground(t.fgMuted)
	s.Tool.ActionCreate = s.Tool.ActionCreate.Foreground(t.fgMuted)
	s.Tool.ActionDestroy = s.Tool.ActionDestroy.Foreground(t.fgMuted)
	// Hook: nhãn và mũi tên là metadata; chỉ OK/Denied mang màu trạng thái. Hai
	// style cuối quickStyle tô bằng token nền nên phải đổi sang màu chữ.
	s.Tool.HookLabel = s.Tool.HookLabel.Foreground(t.fgMuted)
	s.Tool.HookName = s.Tool.HookName.Foreground(t.fgMuted)
	s.Tool.HookArrow = s.Tool.HookArrow.Foreground(t.fgSubtle)
	s.Tool.HookDeniedReason = s.Tool.HookDeniedReason.Foreground(t.fgMuted)
	s.Tool.HookRewrote = s.Tool.HookRewrote.Foreground(t.fgSubtle)

	// --- To-Do: chờ muted, xong subtle, đang làm màu nhấn --------------------------
	// Mỗi dòng chỉ có MỘT style chữ (TodoItem) cho mọi trạng thái nên trạng thái
	// nằm ở màu biểu tượng; chữ dùng fg.muted. Dòng "vừa bắt đầu" là việc đang làm.
	s.Tool.TodoRatio = s.Tool.TodoRatio.Foreground(t.fgMuted)
	s.Tool.TodoCompletedIcon = s.Tool.TodoCompletedIcon.Foreground(t.fgSubtle)
	s.Tool.TodoInProgressIcon = s.Tool.TodoInProgressIcon.Foreground(t.accent)
	s.Tool.TodoPendingIcon = s.Tool.TodoPendingIcon.Foreground(t.fgMuted)
	s.Tool.TodoItem = s.Tool.TodoItem.Foreground(t.fgMuted)
	s.Tool.TodoJustStarted = s.Tool.TodoJustStarted.Foreground(t.accent)
	s.Pills.TodoCurrentTask = s.Pills.TodoCurrentTask.Foreground(t.accent)
	s.Pills.TodoSpinner = s.Pills.TodoSpinner.Foreground(t.accent)
	// Hàng đợi: dải tam giác trung tính (quickStyle tô từ đỏ sang hồng).
	// Viền panel To-Do/hàng đợi là token border (overlay chỉ dành cho nền nổi).
	s.Pills.Focused = s.Pills.Focused.BorderForeground(t.border)
	s.Pills.QueueGradFromColor = t.fgSubtle
	s.Pills.QueueGradToColor = t.fgMuted

	// --- Ô nhập (editor) -------------------------------------------------------------------------
	// Prompt ">" và dấu ":::" của ô đang focus là màu nhấn; placeholder, số dòng
	// của textarea đã là fg.subtle theo token.
	s.Editor.PromptNormalIconFocused = s.Editor.PromptNormalIconFocused.Foreground(t.accent)
	s.Editor.PromptNormalFocused = s.Editor.PromptNormalFocused.Foreground(t.accent)
	// YOLO: huy hiệu vàng cảnh báo (token busy là màu nhấn nên phải đổi nền).
	s.Editor.PromptYoloIconFocused = s.Editor.PromptYoloIconFocused.Background(t.warning)
	// Huy hiệu hỏi: chữ tối trên nền màu nhấn.
	s.Editor.PromptQuestionIconFocused = s.Editor.PromptQuestionIconFocused.Foreground(t.bgBase)
	s.Editor.QuestionRadioOff = s.Editor.QuestionRadioOff.Foreground(t.fgMuted)
	s.Editor.QuestionCheckOff = s.Editor.QuestionCheckOff.Foreground(t.fgMuted)
	s.Radio.On = s.Radio.On.Foreground(t.accent)
	s.Radio.Off = s.Radio.Off.Foreground(t.fgMuted)
	s.Radio.Label = s.Radio.Label.Foreground(t.fgBase)

	// Tab của form hỏi nhiều câu: tab đang chọn màu nhấn, tab còn lại trầm.
	s.Tab.ActiveBorder, _ = everforestTabBorders(t.accent)
	_, s.Tab.InactiveBorder = everforestTabBorders(t.fgSubtle)
	s.Tab.ActiveBorderBlurred, _ = everforestTabBorders(t.fgMuted)
	_, s.Tab.InactiveBorderBlurred = everforestTabBorders(t.fgSubtle)

	// --- Sidebar yên tĩnh -------------------------------------------------------------------------
	// Tiêu đề mục fg.muted (kể cả "Modified Files", vẽ bằng Section.Title), tên
	// phiên fg.base, đường dẫn/tên tệp fg.muted, phụ fg.subtle. Biểu tượng
	// trạng thái không tô xanh lá: một skill hay một server khoẻ là trạng thái
	// bình thường, chỉ lỗi/cảnh báo mới có màu.
	s.Section.Title = s.Section.Title.Foreground(t.fgMuted)
	s.Resource.Heading = s.Resource.Heading.Foreground(t.fgMuted)
	s.Files.SectionTitle = s.Files.SectionTitle.Foreground(t.fgMuted)
	s.Sidebar.SessionTitle = s.Sidebar.SessionTitle.Foreground(t.fgBase)
	s.Resource.OfflineIcon = s.Resource.OfflineIcon.Foreground(t.fgSubtle)
	s.Resource.OnlineIcon = s.Resource.OnlineIcon.Foreground(t.fgMuted)
	s.Resource.DisabledIcon = s.Resource.DisabledIcon.Foreground(t.fgSubtle)
	// Phiên đang xem (và duy nhất nó) lấy foreground của RowTitleBase làm màu tiêu đề.
	s.Resource.RowTitleBase = s.Resource.RowTitleBase.Foreground(t.accent)
	s.LSP.HintDiagnostic = s.LSP.HintDiagnostic.Foreground(t.fgSubtle)
	s.LSP.InfoDiagnostic = s.LSP.InfoDiagnostic.Foreground(t.info)
	s.ModelInfo.HypercreditIcon = s.ModelInfo.HypercreditIcon.Foreground(t.fgMuted)

	// --- Khung chat ---------------------------------------------------------------------------------
	// Thanh trái của tin nhắn người dùng/shell trầm khi chưa chọn; thanh của mục
	// đang focus (tin trợ lý, tool call) là màu nhấn.
	s.Messages.UserBlurred = s.Messages.UserBlurred.BorderForeground(t.fgSubtle)
	s.Messages.ShellBarBlurred = s.Messages.ShellBarBlurred.BorderForeground(t.fgSubtle)
	s.Messages.AssistantFocused = s.Messages.AssistantFocused.BorderForeground(t.accent)
	s.Messages.ToolCallFocused = s.Messages.ToolCallFocused.BorderForeground(t.accent)
	s.Messages.ErrorDetails = s.Messages.ErrorDetails.Foreground(t.fgMuted)
	s.Messages.ShellOutput = s.Messages.ShellOutput.Foreground(t.fgMuted)
	s.Messages.AssistantInfoDuration = s.Messages.AssistantInfoDuration.Foreground(t.fgMuted)
	s.Messages.AssistantCanceled = s.Messages.AssistantCanceled.Foreground(t.fgMuted)
	s.Messages.ThinkingFooterDuration = s.Messages.ThinkingFooterDuration.Foreground(t.fgMuted)
	s.Messages.PlanBox = s.Messages.PlanBox.BorderForeground(t.fgSubtle)

	// --- Thanh trạng thái -----------------------------------------------------------------------------
	// Chữ trên nền màu nhấn phải tối. YOLO là vàng. Thông báo "info" không dùng
	// xanh lá (xanh lá dành cho Success); huy hiệu của nó dùng màu nhấn.
	s.Status.ModeBadgePlan = s.Status.ModeBadgePlan.Foreground(t.bgBase)
	s.Status.ModeBannerPlanBadge = s.Status.ModeBadgePlan
	s.Status.ModeBadgeYolo = s.Status.ModeBadgeYolo.Background(t.warning)
	s.Status.ModeBannerYoloBadge = s.Status.ModeBadgeYolo
	s.Status.InfoIndicator = s.Status.SuccessIndicator.Background(t.accent)
	s.Status.UpdateIndicator = s.Status.InfoIndicator.SetString("HEY!")
	s.Status.InfoMessage = s.Status.SuccessMessage.Foreground(t.fgBase).Background(t.bgOverlay)
	s.Status.UpdateMessage = s.Status.InfoMessage

	// --- Thành phần chọn/đính kèm ---------------------------------------------------------------------
	s.Attachments.Image = s.Attachments.Image.Background(t.fgMuted).Foreground(t.bgBase)
	s.Attachments.Text = s.Attachments.Text.Background(t.fgMuted).Foreground(t.bgBase)
	s.Attachments.Skill = s.Attachments.Skill.Background(t.fgMuted).Foreground(t.bgBase)
	s.Attachments.Normal = s.Attachments.Normal.Background(t.bgOverlay).Foreground(t.fgBase)
	s.Attachments.Remove = s.Attachments.Remove.Foreground(t.fgBase)
	s.Attachments.Deleting = s.Attachments.Deleting.Foreground(t.bgBase)
	s.Attachments.More = s.Attachments.More.Foreground(t.fgMuted)
	s.FilePicker.Directory = s.FilePicker.Directory.Foreground(t.fgBase).Bold(true)
	s.FilePicker.File = s.FilePicker.File.Foreground(t.fgMuted)
	s.FilePicker.Selected = s.FilePicker.Selected.Foreground(t.bgBase)

	// --- Hộp thoại -------------------------------------------------------------------------------------------
	// Khung hộp thoại là border.focus (hộp thoại đang giữ focus). Tiêu đề là chữ
	// thường, đường chéo sau tiêu đề mờ dần; chữ trên dòng đang chọn (nền màu
	// nhấn) phải tối.
	s.Dialog.Title = s.Dialog.Title.Foreground(t.fgBase)
	s.Dialog.TitleText = s.Dialog.TitleText.Foreground(t.fgBase)
	s.Dialog.TitleAccent = s.Dialog.TitleAccent.Foreground(t.accent)
	s.Dialog.TitleGradFromColor = t.fgSubtle
	s.Dialog.TitleGradToColor = t.border
	s.Dialog.View = s.Dialog.View.BorderForeground(t.borderFocus)
	s.Dialog.PrimaryText = s.Dialog.PrimaryText.Foreground(t.fgBase)
	s.Dialog.Spinner = s.Dialog.Spinner.Foreground(t.fgMuted)
	s.Dialog.ScrollbarThumb = s.Dialog.ScrollbarThumb.Foreground(t.fgSubtle)
	// Panel hiện lệnh/tham số JSON của hộp thoại xin quyền là khối code: nền sunken.
	s.Dialog.ContentPanel = s.Dialog.ContentPanel.Background(t.bgSunken)
	s.Dialog.ContentPanelBg = t.bgSunken
	s.Dialog.Permissions.ParamsBg = t.bgSunken
	s.Dialog.ListItem.InfoFocused = s.Dialog.ListItem.InfoFocused.Foreground(t.bgBase)
	s.Dialog.APIKey.Spinner = s.Dialog.APIKey.Spinner.Foreground(t.fgMuted)
	s.Dialog.AuthMethod.CardFocused = s.Dialog.AuthMethod.CardFocused.BorderForeground(t.accent).Foreground(t.accent)
	s.Dialog.OAuth.Spinner = s.Dialog.OAuth.Spinner.Foreground(t.fgMuted)
	s.Dialog.OAuth.Link = s.Dialog.OAuth.Link.Foreground(t.accent)
	s.Dialog.OAuth.Enter = s.Dialog.OAuth.Enter.Foreground(t.fgBase).Bold(true)
	s.Dialog.Sessions.InfoFocused = s.Dialog.Sessions.InfoFocused.Foreground(t.bgBase)
	// Đổi tên là thao tác bình thường, không phải cảnh báo: khung màu nhấn.
	s.Dialog.Sessions.RenamingingTitle = s.Dialog.Sessions.RenamingingTitle.Foreground(t.fgBase)
	s.Dialog.Sessions.RenamingView = s.Dialog.Sessions.RenamingView.BorderForeground(t.accent)
	s.Dialog.Sessions.RenamingTitleGradientFromColor = t.fgSubtle
	s.Dialog.Sessions.RenamingTitleGradientToColor = t.border
	s.Dialog.Sessions.DeletingTitleGradientToColor = t.border
	// Màn khởi tạo tô đường dẫn và phím tắt bằng Initialize.Accent: ở đây chỉ nhấn
	// bằng độ sáng (fg.base trên nền chữ fg.muted), không thêm màu.
	s.Initialize.Accent = s.Initialize.Accent.Foreground(t.fgBase)

	return s
}

// restyleMarkdown áp các quy tắc vai trò cho một cấu hình markdown: tiêu đề là
// chữ thường (không tô màu nhấn), liên kết là màu nhấn (thứ tương tác), inline
// code là fg.base trên nền raised (không cam, không đỏ).
//
// Luôn gán con trỏ mới thay vì ghi vào *string cũ: Markdown và PlanMarkdown
// dùng chung nhiều con trỏ từ lúc quickStyle sao chép struct.
func (t everforestTokens) restyleMarkdown(m *ansi.StyleConfig) {
	m.Heading.Color = hex(t.fgBase)
	m.H1.Color = hex(t.fgBase)
	m.H1.BackgroundColor = hex(t.bgRaised)
	m.H6.Color = hex(t.fgMuted)
	m.LinkText.Color = hex(t.accent)
	m.Code.Color = hex(t.fgBase)
	m.Code.BackgroundColor = hex(t.bgRaised)
}

// restyleChroma gán màu cú pháp theo 6 token syntax.* (kiểu dữ liệu dùng
// synType). Mọi nhóm token khác kế thừa từ nhóm cha như mặc định của chroma.
func (t everforestTokens) restyleChroma(c *ansi.Chroma) {
	if c == nil {
		return
	}
	set := func(p *ansi.StylePrimitive, col color.Color) { p.Color = hex(col) }

	set(&c.Text, t.synDefault)
	set(&c.Comment, t.synComment)
	set(&c.CommentPreproc, t.synKeyword)
	set(&c.Keyword, t.synKeyword)
	set(&c.KeywordReserved, t.synKeyword)
	set(&c.KeywordNamespace, t.synKeyword)
	set(&c.KeywordType, t.synType)
	set(&c.Operator, t.synDefault)
	set(&c.Punctuation, t.synDefault)
	set(&c.Name, t.synDefault)
	set(&c.NameBuiltin, t.synFunction)
	set(&c.NameTag, t.synKeyword)
	set(&c.NameAttribute, t.synType)
	set(&c.NameClass, t.synType)
	set(&c.NameDecorator, t.synKeyword)
	set(&c.NameFunction, t.synFunction)
	set(&c.LiteralNumber, t.synNumber)
	set(&c.LiteralString, t.synString)
	set(&c.LiteralStringEscape, t.synNumber)
	set(&c.GenericDeleted, t.diffDelete)
	set(&c.GenericInserted, t.diffAdd)
	set(&c.GenericSubheading, t.fgMuted)

	// quickStyle gạch chân và in đậm tên lớp; ở đây chỉ tô màu cho yên tĩnh.
	c.NameClass.Underline = nil
	c.NameClass.Bold = nil

	// Nền mặc định của khối code là sunken, khớp với khối tool.
	c.Background.BackgroundColor = hex(t.bgSunken)
}

// everforestTabBorders dựng cặp viền tab (đang chọn, không chọn) cùng một màu,
// giống cách quickStyle dựng viền tab của form hỏi nhiều câu.
func everforestTabBorders(c color.Color) (active, inactive uv.Border) {
	st := uv.Style{Fg: c}
	inactive = uv.RoundedBorder().Style(st)
	inactive.BottomLeft = uv.Side{Content: "┴", Style: st}
	inactive.BottomRight = uv.Side{Content: "┴", Style: st}
	active = uv.RoundedBorder().Style(st)
	active.Bottom = uv.Side{Content: " ", Style: st}
	active.BottomLeft = uv.Side{Content: "┘", Style: st}
	active.BottomRight = uv.Side{Content: "└", Style: st}
	return active, inactive
}

// Màu Everforest Dark v2 cho các màn ngoài TUI chính (login/logout, banner
// lúc thoát, dòng in ra của lệnh CLI), để mọi bề mặt dùng cùng bảng màu với
// theme mặc định thay vì màu Charmtone viết cứng.
var (
	ColorFgBase   = everforestV2.fgBase
	ColorFgMuted  = everforestV2.fgMuted
	ColorFgSubtle = everforestV2.fgSubtle
	ColorAccent   = everforestV2.accent
	ColorSuccess  = everforestV2.success
	ColorWarning  = everforestV2.warning
	ColorError    = everforestV2.error
	ColorBgBase   = everforestV2.bgBase
	ColorBgRaised = everforestV2.bgRaised
	ColorOverlay  = everforestV2.bgOverlay
)
