package styles

import (
	"fmt"
	"image/color"
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/stretchr/testify/require"
)

// loadEverforestV2 nạp theme mới qua đường LoadTheme (như app), cô lập thư mục
// theme người dùng để một tệp everforest-dark-v2.json thật trên máy không che
// bản dựng sẵn. Test dùng nó không được chạy song song (xem setTestThemeDirs).
func loadEverforestV2(t *testing.T) Styles {
	t.Helper()
	setTestThemeDirs(t, []string{t.TempDir()})
	s, err := LoadTheme(everforestDarkV2Name)
	require.NoError(t, err)
	return s
}

// hexOf trả về "#rrggbb" viết thường của một màu.
func hexOf(c color.Color) string { return strings.ToLower(*hex(c)) }

// isSetColor cho biết màu có được đặt thật (không nil, không NoColor).
func isSetColor(c color.Color) bool {
	if c == nil {
		return false
	}
	_, none := c.(lipgloss.NoColor)
	return !none
}

// colorUse là một chỗ dùng màu trong Styles: đường dẫn trường và vai trò của màu.
type colorUse struct {
	path string // ví dụ "Tool.IconSuccess"
	role string // "fg", "bg", "border" hoặc "color" (trường color.Color trần)
	hex  string
}

var lipglossStyleType = reflect.TypeOf(lipgloss.Style{})

// collectColorUses duyệt toàn bộ Styles (chỉ trường export) và gom mọi màu đã
// đặt: fg/bg/viền của lipgloss.Style, và các trường color.Color trần như
// Logo.*, Working*, ANSI[i]. Cấu hình markdown (ansi.StyleConfig) không nằm
// trong đây vì là con trỏ chuỗi, xem TestEverforestDarkV2_MarkdownUsesPaletteOnly.
func collectColorUses(s Styles) []colorUse {
	var uses []colorUse
	var walk func(path string, v reflect.Value)
	walk = func(path string, v reflect.Value) {
		if v.Type() == lipglossStyleType {
			st := v.Interface().(lipgloss.Style)
			add := func(role string, c color.Color) {
				if isSetColor(c) {
					uses = append(uses, colorUse{path: path, role: role, hex: hexOf(c)})
				}
			}
			add("fg", st.GetForeground())
			add("bg", st.GetBackground())
			add("border", st.GetBorderTopForeground())
			return
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := range v.NumField() {
				f := v.Type().Field(i)
				if !f.IsExported() {
					continue
				}
				// Cấu hình markdown dựng từ con trỏ chuỗi, có test riêng.
				if f.Name == "Markdown" || f.Name == "QuietMarkdown" || f.Name == "PlanMarkdown" {
					continue
				}
				next := f.Name
				if path != "" {
					next = path + "." + f.Name
				}
				walk(next, v.Field(i))
			}
		case reflect.Array, reflect.Slice:
			for i := range v.Len() {
				walk(fmt.Sprintf("%s[%d]", path, i), v.Index(i))
			}
		case reflect.Interface:
			if v.IsNil() {
				return
			}
			if c, ok := v.Interface().(color.Color); ok {
				if isSetColor(c) {
					uses = append(uses, colorUse{path: path, role: "color", hex: hexOf(c)})
				}
			}
		}
	}
	walk("", reflect.ValueOf(s))
	return uses
}

// allowedPalette là tập màu hợp lệ của theme: các token trong everforestV2, bảng
// ANSI, và bốn nền pha của diff.
func allowedPalette() map[string]bool {
	t := everforestV2
	allowed := map[string]bool{}
	for _, c := range []color.Color{
		t.bgBase, t.bgSunken, t.bgRaised, t.bgOverlay,
		t.fgBase, t.fgMuted, t.fgSubtle,
		t.border, t.borderFocus, t.accent,
		t.success, t.warning, t.error, t.info,
		t.diffAdd, t.diffDelete, t.diffChange,
		t.synKeyword, t.synFunction, t.synString, t.synNumber, t.synComment, t.synDefault, t.synType,
		blendTint(t.diffAdd, t.bgSunken, diffInsertCodeBlend),
		blendTint(t.diffAdd, t.bgSunken, diffInsertGutterBlend),
		blendTint(t.diffDelete, t.bgSunken, diffDeleteCodeBlend),
		blendTint(t.diffDelete, t.bgSunken, diffDeleteGutterBlend),
	} {
		allowed[hexOf(c)] = true
	}
	for _, c := range t.ansi {
		allowed[hexOf(c)] = true
	}
	return allowed
}

func TestEverforestDarkV2_RegisteredAsBuiltin(t *testing.T) {
	setTestThemeDirs(t, []string{t.TempDir()})

	require.Contains(t, BuiltinThemeNames(), "everforest-dark-v2")
	require.True(t, IsBuiltinTheme("everforest-dark-v2"))
	require.True(t, IsBuiltinTheme("Everforest-Dark-V2"), "tên theme không phân biệt hoa thường")
	require.NoError(t, ValidateThemeName("my-everforest"), "tên người dùng khác vẫn tạo được")
	require.Error(t, ValidateThemeName("everforest-dark-v2"), "không cho tạo theme người dùng trùng tên bản dựng sẵn")

	var found bool
	for _, info := range ListAllThemes() {
		if info.Name == "everforest-dark-v2" {
			found = true
			require.Equal(t, ThemeSourceBuiltin, info.Source)
			require.False(t, info.Overridden)
		}
	}
	require.True(t, found, "theme phải hiện trong hộp thoại Themes (ListAllThemes)")

	s, err := LoadTheme("Everforest-Dark-V2")
	require.NoError(t, err)
	require.Equal(t, "#121316", hexOf(s.Background))
}

// Everforest Dark v2 là mặc định (người dùng chốt 15/10); theme cũ không đổi
// và vẫn chọn được, nên "quay lại" chỉ là chọn charmtone-panther trong Themes.
func TestEverforestDarkV2_IsTheDefaultAndLegacyStaysSelectable(t *testing.T) {
	setTestThemeDirs(t, []string{t.TempDir()})

	def, err := LoadTheme("")
	require.NoError(t, err)
	legacy, err := LoadTheme("charmtone-panther")
	require.NoError(t, err)
	v2, err := LoadTheme(everforestDarkV2Name)
	require.NoError(t, err)

	require.Equal(t, everforestDarkV2Name, DefaultThemeName)
	require.Equal(t, hexOf(v2.Background), hexOf(def.Background), "tên rỗng ra theme mới")
	require.Equal(t, hexOf(v2.Background), hexOf(DefaultTheme().Background))
	require.NotEqual(t, hexOf(legacy.Background), hexOf(v2.Background), "theme cũ vẫn chọn được và khác theme mới")
	require.Equal(t, hexOf(v2.Background), hexOf(ThemeFromConfig("").Background))
	require.Equal(t, hexOf(v2.Background), hexOf(ThemeFromConfig("không-có-theme-này").Background),
		"tên sai rơi về theme mặc định mới")
	require.Equal(t, hexOf(v2.Background), hexOf(ThemeForProvider("").Background))
	require.Equal(t, hexOf(v2.Background), hexOf(ThemeForProvider("hyper").Background), "mọi provider cùng một theme")
	require.Equal(t, "default", ThemeKeyForProvider("hyper"))

	// Palette gốc của hai theme cũ không bị đụng tới.
	p, err := ThemePalette("charmtone-panther")
	require.NoError(t, err)
	require.Equal(t, "Charple", p.Primary)
	require.Equal(t, "Pepper", p.BgBase)
	require.Equal(t, "Sash", p.FgBase)
	g, err := ThemePalette("gruvbox-dark")
	require.NoError(t, err)
	require.Equal(t, "#fabd2f", g.Primary)
	require.Equal(t, "#282828", g.BgBase)
}

// Mọi semantic token trong thiết kế phải có mặt đúng giá trị trong palette
// xuất ra (cũng là thứ trình soạn theme và ExportResolvedPalette nhìn thấy).
func TestEverforestDarkV2_PaletteMatchesSpec(t *testing.T) {
	setTestThemeDirs(t, []string{t.TempDir()})

	p, err := ThemePalette(everforestDarkV2Name)
	require.NoError(t, err)
	require.NoError(t, p.Validate())

	want := map[string]string{
		"bg_base":          "#121316",
		"bg_least_visible": "#0c0d10", // bg.sunken
		"bg_less_visible":  "#1c1e23", // bg.raised
		"bg_most_visible":  "#272a31", // bg.overlay
		"fg_base":          "#d4d7dd",
		"fg_more_subtle":   "#a1a6b0", // fg.muted
		"fg_most_subtle":   "#7b818c", // fg.subtle
		"separator":        "#272a31", // border
		"primary":          "#7fa6e8", // accent
		"accent":           "#7fa6e8",
		"info":             "#7fa6e8",
		"success":          "#8fbf7f",
		"warning":          "#d9b36c",
		"error":            "#e07a7a",
		"diff_insert_fg":   "#8fbf7f",
		"diff_delete_fg":   "#e07a7a",
		"keyword":          "#b79ce8", // syntax.keyword
		"ansi_black":       "#272a31",
		"ansi_red":         "#e07a7a",
		"ansi_green":       "#8fbf7f",
		"ansi_yellow":      "#d9b36c",
		"ansi_blue":        "#7fa6e8",
		"ansi_magenta":     "#b79ce8",
		"ansi_cyan":        "#79b8b0",
		"ansi_white":       "#d4d7dd",

		"ansi_bright_black":   "#555c66",
		"ansi_bright_red":     "#e07a7a",
		"ansi_bright_green":   "#9cc58a",
		"ansi_bright_yellow":  "#e3bd78",
		"ansi_bright_blue":    "#91b5ed",
		"ansi_bright_magenta": "#c3aaec",
		"ansi_bright_cyan":    "#8ac5bd",
		"ansi_bright_white":   "#e1e3e7",
	}
	for _, f := range PaletteFields() {
		if w, ok := want[f.Name]; ok {
			require.Equal(t, w, strings.ToLower(f.Get(p)), "palette field %s", f.Name)
			delete(want, f.Name)
		}
	}
	require.Empty(t, want, "trường palette không tồn tại: %v", want)

	// Không bao giờ dùng đen/trắng tinh làm nền hay chữ.
	for _, f := range PaletteFields() {
		v := strings.ToLower(f.Get(p))
		require.NotEqual(t, "#000000", v, "%s là đen tinh", f.Name)
		require.NotEqual(t, "#ffffff", v, "%s là trắng tinh", f.Name)
	}
}

// Bản fork qua trình soạn theme (user theme có base = theme này) vẫn nạp được,
// kể cả khi sửa palette; style override chỉ áp cho bản nguyên trạng.
func TestEverforestDarkV2_UserForkLoads(t *testing.T) {
	setTestThemeDirs(t, []string{t.TempDir()})

	pure, err := LoadPaletteTheme(everforestDarkV2Name, Palette{})
	require.NoError(t, err)
	require.Equal(t, "#d4d7dd", *pure.Markdown.Code.Color, "bản nguyên trạng có override (inline code = fg.base)")

	forked, err := LoadPaletteTheme(everforestDarkV2Name, Palette{Accent: "#ff8800"})
	require.NoError(t, err)
	require.NotNil(t, forked.WorkingGradFromColor)

	exported, err := ExportResolvedPalette(everforestDarkV2Name)
	require.NoError(t, err)
	require.Equal(t, everforestDarkV2Name, exported.Base)
	require.NoError(t, exported.Validate())
	require.Equal(t, "#121316", strings.ToLower(exported.BgBase))
}

func TestEverforestDarkV2_CloneKeepsColors(t *testing.T) {
	s := loadEverforestV2(t)
	clone := s.Clone()
	require.Equal(t, *s.Markdown.Code.Color, *clone.Markdown.Code.Color)
	require.Equal(t, s.ChromaTheme()[chroma.Keyword], clone.ChromaTheme()[chroma.Keyword])
	newColor := "#ff0000"
	clone.Markdown.Document.Color = &newColor
	require.Equal(t, "#d4d7dd", *s.Markdown.Document.Color, "sửa bản clone không được chạm bản gốc")
}

// Vai trò -> màu, đúng theo bảng quy tắc của thiết kế (Phase 4/5).
func TestEverforestDarkV2_SemanticRoles(t *testing.T) {
	s := loadEverforestV2(t)
	p := everforestV2

	fg := func(st lipgloss.Style) string { return hexOf(st.GetForeground()) }
	bg := func(st lipgloss.Style) string { return hexOf(st.GetBackground()) }
	border := func(st lipgloss.Style) string { return hexOf(st.GetBorderTopForeground()) }
	base, muted, subtle := hexOf(p.fgBase), hexOf(p.fgMuted), hexOf(p.fgSubtle)
	accent, sunken, raised := hexOf(p.accent), hexOf(p.bgSunken), hexOf(p.bgRaised)
	success, errc := hexOf(p.success), hexOf(p.error)

	t.Run("nen_chinh", func(t *testing.T) {
		require.Equal(t, hexOf(p.bgBase), hexOf(s.Background))
	})

	t.Run("chu_thuong_la_fg_base", func(t *testing.T) {
		require.Equal(t, base, *s.Markdown.Document.Color, "thân tin nhắn trợ lý")
		require.Equal(t, base, fg(s.Messages.NoContent), "tin nhắn người dùng")
		require.Equal(t, base, fg(s.Editor.Textarea.Focused.Text), "chữ đang gõ")
		require.Equal(t, base, fg(s.Tool.ContentCodeLine))
		require.Equal(t, base, *s.Markdown.Heading.Color, "tiêu đề markdown không tô màu nhấn")
		require.Equal(t, base, *s.Markdown.H1.Color)
	})

	t.Run("metadata_la_fg_muted", func(t *testing.T) {
		require.Equal(t, muted, fg(s.Sidebar.WorkingDir), "đường dẫn thư mục làm việc")
		require.Equal(t, muted, fg(s.Files.Path), "tên tệp trong Modified Files")
		require.Equal(t, muted, fg(s.Tool.ParamMain), "tham số chính của tool (đường dẫn/lệnh)")
		require.Equal(t, muted, fg(s.ModelInfo.Provider))
		require.Equal(t, muted, fg(s.Header.WorkingDir))
		require.Equal(t, muted, fg(s.Tool.ContentLine), "output bung ra")
	})

	t.Run("goi_y_la_fg_subtle", func(t *testing.T) {
		require.Equal(t, subtle, fg(s.Editor.Textarea.Focused.Placeholder), "placeholder ô nhập")
		require.Equal(t, subtle, fg(s.Editor.Textarea.Focused.LineNumber), "số dòng của ô nhập")
		require.Equal(t, subtle, fg(s.TextInput.Focused.Placeholder))
		require.Equal(t, subtle, fg(s.Tool.ContentLineNumber), "số dòng khối code")
		require.Equal(t, subtle, fg(s.Tool.StateWaiting))
		require.Equal(t, subtle, fg(s.Pills.HelpText))
		require.Equal(t, subtle, fg(s.Header.KeystrokeTip))
	})

	t.Run("thanh_phim_tat", func(t *testing.T) {
		require.Equal(t, muted, fg(s.Help.ShortKey), "phím")
		require.Equal(t, subtle, fg(s.Help.ShortDesc), "mô tả")
		require.Equal(t, muted, fg(s.Help.FullKey))
		require.Equal(t, subtle, fg(s.Help.FullDesc))
		require.Equal(t, muted, fg(s.Dialog.Help.ShortKey))
		require.Equal(t, subtle, fg(s.Dialog.Help.ShortDesc))
		require.Equal(t, muted, fg(s.Pills.HelpKey))
	})

	t.Run("o_nhap", func(t *testing.T) {
		require.Equal(t, accent, fg(s.Editor.PromptNormalIconFocused), "prompt >")
		require.Equal(t, accent, fg(s.Editor.Textarea.Focused.Prompt))
		require.Equal(t, accent, fg(s.TextInput.Focused.Prompt))
		require.Equal(t, accent, hexOf(s.Editor.Textarea.Cursor.Color), "con trỏ")
		require.Equal(t, muted, fg(s.Editor.PromptNormalIconBlurred), "mất focus thì prompt trầm lại")
	})

	t.Run("thinking_va_thoi_gian", func(t *testing.T) {
		require.Equal(t, muted, hexOf(s.WorkingLabelColor), "nhãn Thinking")
		require.Equal(t, muted, hexOf(s.WorkingTimerColor), "thời gian đã trôi")
		require.Equal(t, muted, *s.QuietMarkdown.Document.Color, "nội dung thinking")
		require.Equal(t, sunken, *s.QuietMarkdown.Document.BackgroundColor)
		require.Equal(t, muted, fg(s.Messages.ThinkingFooterTitle))
		require.Equal(t, muted, fg(s.Messages.ThinkingFooterDuration))
		require.Equal(t, muted, fg(s.Messages.AssistantInfoDuration))
	})

	t.Run("khoi_code_va_khoi_bung_ra_nen_sunken", func(t *testing.T) {
		require.Equal(t, sunken, hexOf(s.Tool.ContentCodeBg))
		require.Equal(t, sunken, bg(s.Tool.ContentCodeLine))
		require.Equal(t, sunken, bg(s.Tool.ContentLine))
		require.Equal(t, sunken, bg(s.Tool.ContentLineNumber))
		require.Equal(t, sunken, bg(s.Diff.EqualLine.Code))
		require.Equal(t, sunken, bg(s.Diff.EqualLine.LineNumber))
		require.Equal(t, sunken, *s.Markdown.CodeBlock.Chroma.Background.BackgroundColor)
	})

	t.Run("lines_hidden_la_subtle_tren_sunken", func(t *testing.T) {
		for name, st := range map[string]lipgloss.Style{
			"Tool.ContentTruncation":          s.Tool.ContentTruncation,
			"Tool.ContentCodeTruncation":      s.Tool.ContentCodeTruncation,
			"Tool.DiffTruncation":             s.Tool.DiffTruncation,
			"Messages.ThinkingTruncationHint": s.Messages.ThinkingTruncationHint,
		} {
			require.Equal(t, subtle, fg(st), name)
			require.Equal(t, sunken, bg(st), name)
		}
	})

	t.Run("inline_code_khong_cam_khong_do", func(t *testing.T) {
		for name, md := range map[string]struct{ fg, bg *string }{
			"Markdown":      {s.Markdown.Code.Color, s.Markdown.Code.BackgroundColor},
			"PlanMarkdown":  {s.PlanMarkdown.Code.Color, s.PlanMarkdown.Code.BackgroundColor},
			"QuietMarkdown": {s.QuietMarkdown.Code.Color, s.QuietMarkdown.Code.BackgroundColor},
		} {
			require.Equal(t, base, *md.fg, name)
			require.Equal(t, raised, *md.bg, name)
		}
	})

	t.Run("todo", func(t *testing.T) {
		require.Equal(t, muted, fg(s.Tool.TodoPendingIcon), "đang chờ")
		require.Equal(t, muted, fg(s.Tool.TodoItem), "chữ mục To-Do")
		require.Equal(t, subtle, fg(s.Tool.TodoCompletedIcon), "đã xong")
		require.Equal(t, accent, fg(s.Tool.TodoInProgressIcon), "đang làm")
		require.Equal(t, accent, fg(s.Tool.TodoJustStarted))
		require.Equal(t, accent, fg(s.Pills.TodoCurrentTask))
		require.Equal(t, hexOf(p.border), border(s.Pills.Focused), "viền panel")
	})

	t.Run("sidebar_yen_tinh", func(t *testing.T) {
		require.Equal(t, muted, fg(s.Section.Title), "tiêu đề mục")
		require.Equal(t, muted, fg(s.Resource.Heading))
		require.Equal(t, muted, fg(s.Files.SectionTitle))
		require.Equal(t, base, fg(s.Sidebar.SessionTitle))
		require.Equal(t, hexOf(p.border), fg(s.Section.Line))
		require.Equal(t, accent, fg(s.Resource.RowTitleBase), "phiên đang xem")
		require.NotEqual(t, success, fg(s.Resource.OnlineIcon), "bullet skill/server không tô xanh lá")
		require.NotEqual(t, success, fg(s.Tool.ResourceLoadedText))
		require.Equal(t, success, fg(s.Files.Additions), "số dòng thêm")
		require.Equal(t, errc, fg(s.Files.Deletions), "số dòng xoá")
	})

	t.Run("trang_thai", func(t *testing.T) {
		require.Equal(t, success, fg(s.Tool.IconSuccess))
		require.Equal(t, errc, fg(s.Tool.IconError))
		require.Equal(t, hexOf(p.warning), bg(s.Status.WarnIndicator), "sessionGlyph lấy màu 'đang chờ' từ đây")
		require.Equal(t, success, fg(s.Tool.JobIconSuccess), "sessionGlyph lấy màu 'xong' từ đây")
		require.Equal(t, errc, fg(s.Tool.JobIconError), "sessionGlyph lấy màu 'lỗi' từ đây")
		require.Equal(t, accent, fg(s.Resource.BusyIcon), "sessionGlyph lấy màu 'đang chạy' từ đây")
		require.Equal(t, hexOf(p.warning), bg(s.Status.ModeBadgeYolo), "YOLO là vàng cảnh báo")
		require.Equal(t, hexOf(p.warning), bg(s.Editor.PromptYoloIconFocused))
	})

	t.Run("tool_action_la_accent", func(t *testing.T) {
		require.Equal(t, accent, fg(s.Tool.NameNormal))
	})

	t.Run("diff", func(t *testing.T) {
		require.Equal(t, hexOf(p.diffAdd), fg(s.Diff.InsertLine.LineNumber))
		require.Equal(t, hexOf(p.diffAdd), fg(s.Diff.InsertLine.Symbol))
		require.Equal(t, hexOf(p.diffDelete), fg(s.Diff.DeleteLine.LineNumber))
		require.Equal(t, hexOf(p.diffDelete), fg(s.Diff.DeleteLine.Symbol))
		require.Equal(t, muted, fg(s.Diff.Filename.Code), "tên tệp trong tiêu đề diff")
	})

	t.Run("cu_phap", func(t *testing.T) {
		th := s.ChromaTheme()
		for tok, want := range map[chroma.TokenType]string{
			chroma.Keyword:          hexOf(p.synKeyword),
			chroma.KeywordNamespace: hexOf(p.synKeyword),
			chroma.NameFunction:     hexOf(p.synFunction),
			chroma.LiteralString:    hexOf(p.synString),
			chroma.LiteralNumber:    hexOf(p.synNumber),
			chroma.Comment:          hexOf(p.synComment),
			chroma.Text:             hexOf(p.synDefault),
			chroma.Name:             hexOf(p.synDefault),
			chroma.Operator:         hexOf(p.synDefault),
			chroma.Punctuation:      hexOf(p.synDefault),
		} {
			require.Contains(t, th[tok], want, "token %v", tok)
		}
	})

	t.Run("bang_ansi", func(t *testing.T) {
		for i, c := range p.ansi {
			require.Equal(t, hexOf(c), hexOf(s.ANSI[i]), "ANSI[%d]", i)
		}
	})

	t.Run("logo_khong_gradient_khong_nhay", func(t *testing.T) {
		require.Equal(t, hexOf(s.Logo.TitleColorA), hexOf(s.Logo.TitleColorB), "tên một màu")
		require.Equal(t, hexOf(s.Logo.SmallGradFromColor), hexOf(s.Logo.SmallGradToColor))
		require.Equal(t, hexOf(s.Header.LogoGradFromColor), hexOf(s.Header.LogoGradToColor))
		require.Equal(t, muted, hexOf(s.Logo.TitleColorA))
		require.Equal(t, subtle, hexOf(s.Logo.VersionColor), "phiên bản")
		require.NotEqual(t, accent, hexOf(s.Logo.TitleColorA), "logo không dùng màu nhấn")
		require.Equal(t, hexOf(p.border), hexOf(s.Logo.FieldColor), "dải chéo chỉ là hoạ tiết")
	})
}

// Mỗi màu trạng thái chỉ được xuất hiện ở đúng vai trò của nó. Danh sách dưới
// đây là toàn bộ chỗ được phép; thêm chỗ mới phải sửa test này một cách có chủ ý.
func TestEverforestDarkV2_StatusColorsOnlyWhereTheyBelong(t *testing.T) {
	s := loadEverforestV2(t)
	p := everforestV2

	usesOf := func(c color.Color) []string {
		var out []string
		for _, u := range collectColorUses(s) {
			if u.hex == hexOf(c) {
				out = append(out, u.path+"["+u.role+"]")
			}
		}
		return out
	}

	cases := []struct {
		name    string
		c       color.Color
		allowed []string
	}{
		{
			name: "success chỉ cho thành công/khoẻ/dòng thêm",
			c:    p.success,
			allowed: []string{
				"ANSI[2][color]",
				"Diff.InsertLine.LineNumber[fg]", "Diff.InsertLine.Symbol[fg]",
				"Dialog.OAuth.Success[fg]",
				"Files.Additions[fg]",
				"Resource.OnlineText[fg]",
				"Status.SuccessIndicator[bg]", "Status.SuccessMessage[bg]",
				"Tool.HookOK[fg]", "Tool.IconSuccess[fg]", "Tool.JobIconSuccess[fg]",
				"ToolCallSuccess[fg]",
			},
		},
		{
			name: "error chỉ cho lỗi thật/dòng xoá/hành động huỷ",
			c:    p.error,
			allowed: []string{
				"ANSI[1][color]", "ANSI[9][color]",
				"Attachments.Deleting[bg]",
				"Button.Negative[bg]",
				"Diff.DeleteLine.LineNumber[fg]", "Diff.DeleteLine.Symbol[fg]",
				"Dialog.OAuth.ErrorText[fg]", "Dialog.Sessions.DeletingItemFocused[bg]",
				"Dialog.Sessions.DeletingTitle[fg]", "Dialog.Sessions.DeletingView[border]",
				"Dialog.Sessions.DeletingTitleGradientFromColor[color]",
				"Dialog.TitleError[fg]",
				"Files.Deletions[fg]",
				"LSP.ErrorDiagnostic[fg]",
				"Messages.ErrorTag[bg]", "Messages.ShellExitCode[fg]",
				"Resource.ErrorIcon[fg]",
				"Status.ErrorIndicator[bg]", "Status.ErrorMessage[bg]",
				"Tool.ErrorTag[bg]", "Tool.HookDenied[fg]", "Tool.HookDeniedLabel[fg]",
				"Tool.IconError[fg]", "Tool.JobIconError[fg]",
			},
		},
		{
			name: "warning chỉ cho cảnh báo/thận trọng",
			c:    p.warning,
			allowed: []string{
				"ANSI[3][color]",
				"Editor.PromptYoloDotsFocused[fg]", "Editor.PromptYoloIconFocused[bg]",
				"LSP.WarningDiagnostic[fg]",
				"Resource.NeedsAuthIcon[fg]",
				"Status.ModeBadgeYolo[bg]", "Status.ModeBannerYolo[bg]", "Status.ModeBannerYoloBadge[bg]",
				"Status.WarnIndicator[bg]", "Status.WarnMessage[bg]",
				"Tool.WarnTag[bg]",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.ElementsMatch(t, tc.allowed, usesOf(tc.c))
		})
	}
}

// Màu nhấn chỉ dành cho thứ tương tác/đang hiện hành. Kiểm theo tiền tố đường
// dẫn để bắt trường hợp màu nhấn lan sang header, sidebar, tiêu đề mục...
func TestEverforestDarkV2_AccentIsRestrained(t *testing.T) {
	s := loadEverforestV2(t)
	accent := hexOf(everforestV2.accent)

	allowedPrefixes := []string{
		"ANSI[4]",             // blue của bảng ANSI
		"Button.Focused",      // nút đang chọn
		"Completions.Focused", // dòng gợi ý đang chọn
		"Dialog.Arguments.InputRequiredMarkFocused", // ô đang nhập
		"Dialog.AuthMethod.CardFocused",             // thẻ đang chọn
		"Dialog.OAuth.Link",                         // liên kết bấm được
		"Dialog.Quit.Frame",                         // hộp thoại đang giữ focus
		"Dialog.SelectedItem",                       // dòng đang chọn
		"Dialog.Sessions.RenamingView",              // ô đang đổi tên
		"Dialog.TitleAccent",                        // từ khoá cần bấm trong hộp thoại
		"Dialog.View",                               // khung hộp thoại đang giữ focus
		"Editor.",                                   // prompt, con trỏ, chip chế độ, lựa chọn câu hỏi
		"FilePicker.Selected",                       // tệp đang chọn
		"LSP.InfoDiagnostic",                        // trạng thái info
		"Messages.AssistantFocused",                 // mục chat đang focus
		"Messages.ShellBarFocused",                  //
		"Messages.ShellPrompt",                      // prompt $ của chế độ bang
		"Messages.ToolCallFocused",                  //
		"Messages.UserFocused",                      //
		"Pills.TodoCurrentTask",                     // việc đang làm
		"Pills.TodoSpinner",                         //
		"Radio.On",                                  // lựa chọn đã chọn
		"Resource.BusyIcon",                         // đang chạy
		"Resource.RowTitleBase",                     // phiên hiện tại
		"Status.InfoIndicator",                      // huy hiệu info
		"Status.ModeBadgePlan",                      // chip chế độ plan
		"Status.ModeBannerPlanBadge",                //
		"Status.UpdateIndicator",                    //
		"Tab.ActiveBorder",                          // tab đang chọn
		"TextInput.Cursor",                          // con trỏ
		"TextInput.Focused.Prompt",                  //
		"TextSelection",                             // vùng đang bôi chọn
		"Tool.JobToolName",                          // tên tool
		"Tool.MCPName",                              //
		"Tool.NameNested",                           //
		"Tool.NameNormal",                           //
		"Tool.NoteTag",                              // nhãn info
		"Tool.TodoInProgressIcon",                   // việc đang làm
		"Tool.TodoJustStarted",                      //
	}
	for _, u := range collectColorUses(s) {
		if u.hex != accent {
			continue
		}
		ok := false
		for _, pre := range allowedPrefixes {
			if strings.HasPrefix(u.path, pre) {
				ok = true
				break
			}
		}
		require.True(t, ok, "màu nhấn xuất hiện ở chỗ không được phép: %s [%s]", u.path, u.role)
	}
}

// Không có màu nào ngoài bảng màu của theme (không sót màu Charmtone).
func TestEverforestDarkV2_OnlyPaletteColors(t *testing.T) {
	s := loadEverforestV2(t)
	allowed := allowedPalette()
	for _, u := range collectColorUses(s) {
		require.True(t, allowed[u.hex], "%s [%s] dùng màu ngoài bảng màu: %s", u.path, u.role, u.hex)
		require.NotEqual(t, "#000000", u.hex, "%s [%s] là đen tinh", u.path, u.role)
		require.NotEqual(t, "#ffffff", u.hex, "%s [%s] là trắng tinh", u.path, u.role)
	}
}

// Màu trong cấu hình markdown (glamour) và chroma là chuỗi hex, không đi qua
// lipgloss.Style, nên kiểm riêng.
func TestEverforestDarkV2_MarkdownUsesPaletteOnly(t *testing.T) {
	s := loadEverforestV2(t)
	allowed := allowedPalette()

	var found int
	for name, ptrs := range map[string][]*string{
		"Markdown": {
			s.Markdown.Document.Color, s.Markdown.Heading.Color, s.Markdown.H1.Color, s.Markdown.H1.BackgroundColor,
			s.Markdown.H6.Color, s.Markdown.Link.Color, s.Markdown.LinkText.Color, s.Markdown.Image.Color,
			s.Markdown.ImageText.Color, s.Markdown.Code.Color, s.Markdown.Code.BackgroundColor,
			s.Markdown.HorizontalRule.Color, s.Markdown.CodeBlock.Color,
		},
		"PlanMarkdown": {
			s.PlanMarkdown.Heading.Color, s.PlanMarkdown.H1.Color, s.PlanMarkdown.H2.Color, s.PlanMarkdown.H3.Color,
			s.PlanMarkdown.H4.Color, s.PlanMarkdown.H5.Color, s.PlanMarkdown.H6.Color, s.PlanMarkdown.LinkText.Color,
			s.PlanMarkdown.Code.Color, s.PlanMarkdown.Code.BackgroundColor, s.PlanMarkdown.CodeBlock.Color,
		},
		"QuietMarkdown": {
			s.QuietMarkdown.Document.Color, s.QuietMarkdown.Document.BackgroundColor,
			s.QuietMarkdown.Code.Color, s.QuietMarkdown.Code.BackgroundColor,
		},
	} {
		for i, p := range ptrs {
			require.NotNil(t, p, "%s màu #%d chưa đặt", name, i)
			found++
			require.True(t, allowed[strings.ToLower(*p)], "%s màu #%d ngoài bảng màu: %s", name, i, *p)
		}
	}
	require.Positive(t, found)

	// Mọi mục trong ChromaTheme là "màu [bg:màu] [italic|bold|underline]".
	for tok, entry := range s.ChromaTheme() {
		for _, field := range strings.Fields(entry) {
			field = strings.TrimPrefix(field, "bg:")
			if !strings.HasPrefix(field, "#") {
				continue
			}
			require.True(t, allowed[strings.ToLower(field)], "chroma %v dùng màu ngoài bảng màu: %s", tok, field)
		}
	}
}

// Độ tương phản WCAG 2.x. Chữ đọc được phải >= 4.5:1. Các cặp được kiểm là các
// cặp thật trong UI, không chỉ vài cặp mẫu.
func TestEverforestDarkV2_Contrast(t *testing.T) {
	const minAA = 4.5
	p := everforestV2

	t.Run("bang_yeu_cau", func(t *testing.T) {
		for _, fgName := range []struct {
			name string
			c    color.Color
		}{
			{"fg.base", p.fgBase}, {"fg.muted", p.fgMuted}, {"fg.subtle", p.fgSubtle},
		} {
			for _, bgName := range []struct {
				name string
				c    color.Color
			}{
				{"bg.base", p.bgBase}, {"bg.sunken", p.bgSunken},
			} {
				r := contrastRatio(hexOf(fgName.c), hexOf(bgName.c))
				t.Logf("%-10s trên %-10s = %5.2f:1", fgName.name, bgName.name, r)
				require.GreaterOrEqual(t, r, minAA, "%s trên %s", fgName.name, bgName.name)
			}
		}
		for _, fgName := range []struct {
			name string
			c    color.Color
		}{
			{"accent", p.accent}, {"success", p.success}, {"warning", p.warning}, {"error", p.error},
		} {
			r := contrastRatio(hexOf(fgName.c), hexOf(p.bgBase))
			t.Logf("%-10s trên %-10s = %5.2f:1", fgName.name, "bg.base", r)
			require.GreaterOrEqual(t, r, minAA, "%s trên bg.base", fgName.name)
		}
	})

	s := loadEverforestV2(t)

	// Mọi style có cả chữ lẫn nền: kiểm tương phản thật giữa hai màu đó.
	t.Run("moi_style_co_ca_fg_va_bg", func(t *testing.T) {
		type pair struct{ fg, bg string }
		fgs, bgs := map[string]string{}, map[string]string{}
		for _, u := range collectColorUses(s) {
			switch u.role {
			case "fg":
				fgs[u.path] = u.hex
			case "bg":
				bgs[u.path] = u.hex
			}
		}
		var checked int
		for path, f := range fgs {
			b, ok := bgs[path]
			if !ok {
				continue
			}
			checked++
			require.GreaterOrEqual(t, contrastRatio(f, b), minAA, "%s: chữ %s trên nền %s", path, f, b)
		}
		require.Greater(t, checked, 40, "số style có cả fg và bg phải đủ lớn để phép kiểm có nghĩa")
	})

	// Style chỉ có chữ (không đặt nền) nằm trên nền bg.base của cửa sổ, trừ vài
	// thứ chỉ là hoạ tiết (đường kẻ, dải chéo, dấu phân cách) nên không cần đọc
	// được, và các style được vẽ chồng lên một nền do style cha cấp.
	t.Run("chu_tren_nen_base", func(t *testing.T) {
		decorative := map[string]string{
			"Header.Diagonals":           "dải chéo trang trí",
			"Help.Ellipsis":              "dấu phân cách",
			"Help.ShortSeparator":        "dấu phân cách",
			"Help.FullSeparator":         "dấu phân cách",
			"Dialog.Help.Ellipsis":       "dấu phân cách",
			"Dialog.Help.ShortSeparator": "dấu phân cách",
			"Dialog.Help.FullSeparator":  "dấu phân cách",
			"Dialog.ScrollbarTrack":      "đường ray thanh cuộn",
			"Section.Line":               "đường kẻ mục sidebar",
			"Logo.SmallDiagonals":        "dải chéo trang trí",
		}
		// Chữ của cột thông tin trên dòng đang chọn được vẽ lên nền màu nhấn của dòng.
		drawnOn := map[string]color.Color{
			"Dialog.ListItem.InfoFocused": p.accent,
			"Dialog.Sessions.InfoFocused": p.accent,
		}
		hasBg := map[string]bool{}
		for _, u := range collectColorUses(s) {
			if u.role == "bg" {
				hasBg[u.path] = true
			}
		}
		var checked int
		for _, u := range collectColorUses(s) {
			if u.role != "fg" || hasBg[u.path] {
				continue
			}
			if _, ok := decorative[u.path]; ok {
				// Hoạ tiết phải còn nhìn thấy mờ nhưng không cần 4.5: nếu lỡ đủ
				// tương phản thì bỏ khỏi danh sách để danh sách không phình ra.
				require.Less(t, contrastRatio(u.hex, hexOf(p.bgBase)), minAA, "%s đủ tương phản: bỏ khỏi danh sách hoạ tiết", u.path)
				continue
			}
			bgc := p.bgBase
			if c, ok := drawnOn[u.path]; ok {
				bgc = c
			}
			checked++
			require.GreaterOrEqual(t, contrastRatio(u.hex, hexOf(bgc)), minAA, "%s: %s trên %s", u.path, u.hex, hexOf(bgc))
		}
		require.Greater(t, checked, 100)
	})

	// Màu "trần" (không nằm trong lipgloss.Style) dùng làm chữ.
	t.Run("mau_tran_dung_lam_chu", func(t *testing.T) {
		for name, c := range map[string]color.Color{
			"Working label": s.WorkingLabelColor, "Working timer": s.WorkingTimerColor,
			"Working gradient from": s.WorkingGradFromColor, "Working gradient to": s.WorkingGradToColor,
			"Resource.DefaultTitleFg": s.Resource.DefaultTitleFg, "Resource.DefaultDescFg": s.Resource.DefaultDescFg,
			"Logo.TitleColorA": s.Logo.TitleColorA, "Logo.TitleColorB": s.Logo.TitleColorB,
			"Logo.CharmColor": s.Logo.CharmColor, "Logo.VersionColor": s.Logo.VersionColor,
			"Logo.SmallGradFromColor": s.Logo.SmallGradFromColor, "Logo.SmallGradToColor": s.Logo.SmallGradToColor,
			"Header.LogoGradFromColor": s.Header.LogoGradFromColor, "Header.LogoGradToColor": s.Header.LogoGradToColor,
			"Pills.QueueGradFromColor": s.Pills.QueueGradFromColor, "Pills.QueueGradToColor": s.Pills.QueueGradToColor,
			"Cursor": s.Editor.Textarea.Cursor.Color,
		} {
			require.GreaterOrEqual(t, contrastRatio(hexOf(c), hexOf(p.bgBase)), minAA, name)
		}
	})

	// Các cặp chữ/nền nằm trong cấu hình markdown (glamour) và chroma.
	t.Run("markdown_va_cu_phap", func(t *testing.T) {
		md := s.Markdown
		require.GreaterOrEqual(t, contrastRatio(*md.Code.Color, *md.Code.BackgroundColor), minAA, "inline code")
		require.GreaterOrEqual(t, contrastRatio(*md.H1.Color, *md.H1.BackgroundColor), minAA, "H1")
		require.GreaterOrEqual(t, contrastRatio(*md.Document.Color, hexOf(p.bgBase)), minAA, "thân")
		q := s.QuietMarkdown
		require.GreaterOrEqual(t, contrastRatio(*q.Document.Color, *q.Document.BackgroundColor), minAA, "thinking")
		require.GreaterOrEqual(t, contrastRatio(*q.Code.Color, *q.Code.BackgroundColor), minAA, "inline code trong thinking")
		for tok, entry := range s.ChromaTheme() {
			fields := strings.Fields(entry)
			if len(fields) == 0 || !strings.HasPrefix(fields[0], "#") {
				continue
			}
			for _, bgName := range []string{hexOf(p.bgSunken), hexOf(p.bgBase)} {
				if tok == chroma.Error {
					continue // chữ tối trên nền lỗi, có bg riêng
				}
				require.GreaterOrEqual(t, contrastRatio(fields[0], bgName), minAA, "chroma %v trên %s", tok, bgName)
			}
		}
	})
}

// Mỗi style tường minh chỉ định "nền = bg.sunken" cho khối bung ra phải thật sự
// tối hơn nền chính, để khối code chìm xuống thay vì nổi lên.
func TestEverforestDarkV2_SunkenIsDarkerThanBase(t *testing.T) {
	p := everforestV2
	require.Less(t, relativeLuminance(hexOf(p.bgSunken)), relativeLuminance(hexOf(p.bgBase)))
	require.Less(t, relativeLuminance(hexOf(p.bgBase)), relativeLuminance(hexOf(p.bgRaised)))
	require.Less(t, relativeLuminance(hexOf(p.bgRaised)), relativeLuminance(hexOf(p.bgOverlay)))
}
