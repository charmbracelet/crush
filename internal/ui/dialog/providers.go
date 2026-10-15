package dialog

import (
	"cmp"
	"log/slog"
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/sahilm/fuzzy"
)

const (
	// ProvidersID is the identifier for the provider picker dialog.
	ProvidersID              = "providers"
	providersDialogMaxWidth  = 60
	providersDialogMinHeight = 8
	providersDialogMaxHeight = 16
)

// Providers là hộp thoại chọn provider (alt+p). Chọn xong, UI mở bảng model
// chỉ gồm model của provider đó.
type Providers struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

// ProviderItem là một dòng của hộp thoại: tên provider và trạng thái của nó.
type ProviderItem struct {
	*list.Versioned
	id      string
	name    string
	status  string
	t       *styles.Styles
	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var (
	_ Dialog   = (*Providers)(nil)
	_ ListItem = (*ProviderItem)(nil)
)

// NewProviders tạo hộp thoại chọn provider.
func NewProviders(com *common.Common) (*Providers, error) {
	known, err := config.Providers(com.Config())
	if err != nil {
		slog.Warn("Listing the previously known providers", "error", err)
	}
	return newProviders(com, known), nil
}

// newProviders nhận catalog từ ngoài: config.Providers nhớ kết quả lần gọi
// đầu (sync.Once) nên test phải truyền catalog của riêng nó.
func newProviders(com *common.Common, known []catwalk.Provider) *Providers {
	p := &Providers{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	p.help = h

	p.list = list.NewFilterableList()
	p.list.Focus()

	p.input = textinput.New()
	p.input.SetVirtualCursor(false)
	p.input.Placeholder = "Type to filter"
	p.input.SetStyles(com.Styles.TextInput)
	p.input.Focus()

	p.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "choose models"),
	)
	p.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	p.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	p.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	p.keyMap.Close = CloseKey

	p.setItems(known)
	return p
}

// ID implements Dialog.
func (p *Providers) ID() string {
	return ProvidersID
}

// HandleMsg implements [Dialog].
func (p *Providers) HandleMsg(msg tea.Msg) Action {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	switch {
	case key.Matches(keyMsg, p.keyMap.Close):
		return ActionClose{}
	case key.Matches(keyMsg, p.keyMap.Previous):
		p.list.Focus()
		if p.list.IsSelectedFirst() {
			p.list.SelectLast()
			p.list.ScrollToBottom()
			break
		}
		p.list.SelectPrev()
		p.list.ScrollToSelected()
	case key.Matches(keyMsg, p.keyMap.Next):
		p.list.Focus()
		if p.list.IsSelectedLast() {
			p.list.SelectFirst()
			p.list.ScrollToTop()
			break
		}
		p.list.SelectNext()
		p.list.ScrollToSelected()
	case key.Matches(keyMsg, p.keyMap.Select):
		item, ok := p.list.SelectedItem().(*ProviderItem)
		if !ok {
			break
		}
		return ActionSelectProvider{ProviderID: item.id}
	default:
		prev := p.input.Value()
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		if value := p.input.Value(); value != prev {
			p.list.SetFilter(value)
			p.list.ScrollToTop()
			p.list.SetSelected(0)
		}
		return ActionCmd{cmd}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (p *Providers) Cursor() *tea.Cursor {
	return InputCursor(p.com.Styles, p.input.Cursor())
}

// Draw implements [Dialog].
func (p *Providers) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := p.com.Styles
	width := max(0, min(providersDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	p.input.SetWidth(dialogInputTextWidth(t, p.input, innerWidth))

	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()
	desiredHeight := heightOffset + p.list.TotalHeight()
	maxAvailable := area.Dy() - t.Dialog.View.GetVerticalBorderSize()
	height := max(providersDialogMinHeight, min(providersDialogMaxHeight, desiredHeight, maxAvailable))

	listHeight, listTotalHeight, _ := sizeDialogList(t, p.list, innerWidth, height)

	rc := NewRenderContext(t, width)
	rc.Title = "Switch Provider"
	rc.AddPart(t.Dialog.InputPrompt.Render(p.input.View()))

	if p.list.Height() >= len(p.list.FilteredItems()) {
		p.list.ScrollToTop()
	} else {
		p.list.ScrollToSelected()
	}
	listView := t.Dialog.List.Height(p.list.Height()).Render(p.list.Render())
	rc.AddPart(joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, p.list.Offset()))
	rc.Help = renderDialogHelp(t, &p.help, p, innerWidth)

	view := rc.Render()
	cur := p.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (p *Providers) ShortHelp() []key.Binding {
	return []key.Binding{p.keyMap.UpDown, p.keyMap.Select, p.keyMap.Close}
}

// FullHelp implements [help.KeyMap].
func (p *Providers) FullHelp() [][]key.Binding {
	return [][]key.Binding{{p.keyMap.Select, p.keyMap.Next, p.keyMap.Previous, p.keyMap.Close}}
}

// setItems dựng danh sách provider: theo options.visible_providers nếu có
// (đúng thứ tự khai), không thì các provider đã cấu hình xếp theo tên. Provider
// đang dùng luôn có mặt và đứng đầu. ID lạ (không có trong cấu hình lẫn
// catalog) và provider đã tắt bị ẩn: chọn vào chỉ dẫn tới một bảng model rỗng.
func (p *Providers) setItems(known []catwalk.Provider) {
	cfg := p.com.Config()
	current := cfg.Models[config.SelectedModelTypeLarge].Provider

	names := map[string]string{}
	inCatalog := map[string]bool{}
	for _, k := range known {
		names[string(k.ID)] = k.Name
		inCatalog[string(k.ID)] = true
	}

	var configured []string
	for id, pc := range cfg.Providers.Seq2() {
		if pc.Name != "" {
			names[id] = pc.Name
		}
		if !pc.Disable {
			configured = append(configured, id)
		}
	}
	usable := func(id string) bool {
		if pc, ok := cfg.Providers.Get(id); ok {
			return !pc.Disable
		}
		return inCatalog[id]
	}

	ids := slices.Clone(cfg.Options.VisibleProviders)
	if len(ids) == 0 {
		slices.SortFunc(configured, func(a, b string) int {
			return cmp.Compare(strings.ToLower(cmp.Or(names[a], a)), strings.ToLower(cmp.Or(names[b], b)))
		})
		ids = configured
	}
	if current != "" {
		ids = append([]string{current}, slices.DeleteFunc(ids, func(id string) bool { return id == current })...)
	}

	items := make([]list.FilterableItem, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if id != current && !usable(id) {
			slog.Warn("Hiding unknown or disabled provider from the provider picker", "provider", id)
			continue
		}
		items = append(items, &ProviderItem{
			Versioned: list.NewVersioned(),
			id:        id,
			name:      cmp.Or(names[id], id),
			status:    providerStatus(cfg, id, current),
			t:         p.com.Styles,
		})
	}
	p.list.SetItems(items...)
	p.list.SetSelected(0)
	p.list.ScrollToTop()
}

// loginCommands là lệnh đăng nhập của các provider dùng tài khoản (OAuth)
// thay cho API key.
var loginCommands = map[catwalk.InferenceProvider]string{
	catwalk.InferenceProviderOpenAI:  "agent-cli login openai",
	catwalk.InferenceProviderXAI:     "agent-cli login grok",
	catwalk.InferenceProviderCopilot: "agent-cli login copilot",
	"hyper":                          "agent-cli login",
}

// providerStatus cho biết provider dùng được ngay hay còn thiếu gì.
func providerStatus(cfg *config.Config, id, current string) string {
	if id == current {
		return "current"
	}
	if pc, ok := cfg.Providers.Get(id); ok && !pc.Disable {
		return "ready"
	}
	if cmd, ok := loginCommands[catwalk.InferenceProvider(id)]; ok {
		return cmd
	}
	return "needs API key"
}

// Finished implements list.Item.
func (i *ProviderItem) Finished() bool { return true }

// Filter returns the filter value for the provider item.
func (i *ProviderItem) Filter() string { return i.name + " " + i.id }

// ID returns the provider ID.
func (i *ProviderItem) ID() string { return i.id }

// SetFocused sets the focus state of the provider item.
func (i *ProviderItem) SetFocused(focused bool) {
	if i.focused == focused {
		return
	}
	i.cache = nil
	i.focused = focused
	if i.Versioned != nil {
		i.Bump()
	}
}

// SetMatch sets the fuzzy match for the provider item.
func (i *ProviderItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(i.m, m) {
		return
	}
	i.cache = nil
	i.m = m
	if i.Versioned != nil {
		i.Bump()
	}
}

// Render returns the string representation of the provider item.
func (i *ProviderItem) Render(width int) string {
	st := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: i.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(st, i.name, i.status, i.focused, width, i.cache, &i.m)
}
