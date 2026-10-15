package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

// scopeWorkspace đếm số lần bảng model ghi cấu hình.
type scopeWorkspace struct {
	recentWorkspace
	configWrites int
}

func (w *scopeWorkspace) SetConfigField(config.Scope, string, any) error {
	w.configWrites++
	return nil
}

func twoProviderConfig() *config.Config {
	providers := csync.NewMap[string, config.ProviderConfig]()
	providers.Set("main", config.ProviderConfig{
		ID: "main", Name: "GLM",
		Models: []catwalk.Model{{ID: "glm-5.3"}, {ID: "glm-5.3-flash"}},
	})
	providers.Set("other", config.ProviderConfig{
		ID: "other", Models: []catwalk.Model{{ID: "deepseek"}},
	})
	current := config.SelectedModel{Provider: "main", Model: "glm-5.3"}
	return &config.Config{
		Options:   &config.Options{DisableDefaultProviders: true, DisableProviderAutoUpdate: true},
		Providers: providers,
		Models:    map[config.SelectedModelType]config.SelectedModel{config.SelectedModelTypeLarge: current},
		RecentModels: map[config.SelectedModelType][]config.SelectedModel{
			config.SelectedModelTypeLarge: {current, {Provider: "other", Model: "deepseek"}},
		},
	}
}

// alt+m chỉ hiện model của provider đang dùng, và không được coi model gần
// đây của provider khác là hỏng mà xoá khỏi cấu hình.
func TestModelsDialogScopedToProvider(t *testing.T) {
	ws := &scopeWorkspace{recentWorkspace: recentWorkspace{cfg: twoProviderConfig()}}
	m, err := NewModelsForProvider(common.DefaultCommon(ws), "main")
	require.NoError(t, err)

	var ids []string
	for _, g := range m.list.groups {
		for _, it := range g.Items {
			ids = append(ids, it.ID())
		}
	}
	require.NotEmpty(t, ids)
	for _, id := range ids {
		require.Truef(t, strings.HasPrefix(id, "main:"), "unexpected model %q in a GLM-only list", id)
	}
	require.Zero(t, ws.configWrites, "a scoped list must keep other providers' recent models")
	require.Equal(t, "GLM", m.providerName())

	// Bảng đầy đủ (không lọc) vẫn hiện mọi provider như cũ.
	full, err := NewModels(common.DefaultCommon(ws), false)
	require.NoError(t, err)
	require.Empty(t, full.providerName())
}

func providerItems(t *testing.T, p *Providers) []*ProviderItem {
	t.Helper()
	var out []*ProviderItem
	for _, it := range p.list.FilteredItems() {
		pi, ok := it.(*ProviderItem)
		require.True(t, ok)
		out = append(out, pi)
	}
	return out
}

// alt+p liệt kê đúng các provider khai trong visible_providers, theo thứ tự
// khai, kèm trạng thái cho biết dùng được ngay hay còn thiếu gì.
func TestProvidersDialogListsVisibleProviders(t *testing.T) {
	cfg := twoProviderConfig()
	cfg.Options.VisibleProviders = []string{"main", "openai", "antropic", "anthropic"}
	catalog := []catwalk.Provider{{ID: "openai", Name: "OpenAI"}, {ID: "anthropic", Name: "Anthropic"}}
	p := newProviders(common.DefaultCommon(&recentWorkspace{cfg: cfg}), catalog)

	items := providerItems(t, p)
	require.Len(t, items, 3, "providers outside the list, and unknown IDs, stay hidden")
	var got [][2]string
	for _, it := range items {
		got = append(got, [2]string{it.id, it.status})
	}
	require.Equal(t, [][2]string{
		{"main", "current"},
		{"openai", "agent-cli login openai"},
		{"anthropic", "needs API key"},
	}, got)
	require.Equal(t, []string{"GLM", "OpenAI", "Anthropic"}, []string{items[0].name, items[1].name, items[2].name})

	// Enter chọn provider đang được chọn (provider hiện dùng); ↓ rồi Enter chọn dòng kế.
	resp, ok := p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}).(ActionSelectProvider)
	require.True(t, ok)
	require.Equal(t, "main", resp.ProviderID)
	p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	resp, ok = p.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter}).(ActionSelectProvider)
	require.True(t, ok)
	require.Equal(t, "openai", resp.ProviderID)
}

// Không khai visible_providers: liệt kê các provider đã cấu hình, provider
// đang dùng đứng đầu.
func TestProvidersDialogDefaultsToConfiguredProviders(t *testing.T) {
	p, err := NewProviders(common.DefaultCommon(&recentWorkspace{cfg: twoProviderConfig()}))
	require.NoError(t, err)

	items := providerItems(t, p)
	require.Len(t, items, 2)
	require.Equal(t, "main", items[0].id)
	require.Equal(t, "current", items[0].status)
	require.Equal(t, "other", items[1].id)
	require.Equal(t, "ready", items[1].status)
}

// Trong bảng model, alt+p đổi sang hộp thoại chọn provider.
func TestModelsDialogAltPOpensProviders(t *testing.T) {
	ws := &scopeWorkspace{recentWorkspace: recentWorkspace{cfg: twoProviderConfig()}}
	m, err := NewModelsForProvider(common.DefaultCommon(ws), "main")
	require.NoError(t, err)
	_, ok := m.HandleMsg(tea.KeyPressMsg{Code: 'p', Mod: tea.ModAlt}).(ActionOpenProviders)
	require.True(t, ok)
}

// Tab sang loại model mà model hiện tại không thuộc provider đang lọc: chọn lại
// từ đầu, không giữ chỉ số cũ trỏ vào một dòng bất kỳ như thể đó là model đang dùng.
func TestModelsDialogScopedTabReselects(t *testing.T) {
	cfg := twoProviderConfig()
	cfg.Models[config.SelectedModelTypeLarge] = config.SelectedModel{Provider: "main", Model: "glm-5.3-flash"}
	cfg.Models[config.SelectedModelTypeSmall] = config.SelectedModel{Provider: "other", Model: "deepseek"}
	m, err := NewModelsForProvider(common.DefaultCommon(&scopeWorkspace{recentWorkspace: recentWorkspace{cfg: cfg}}), "main")
	require.NoError(t, err)
	require.Equal(t, "main:glm-5.3-flash", m.list.SelectedItem().(*ModelItem).ID())

	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Equal(t, "main:glm-5.3", m.list.SelectedItem().(*ModelItem).ID(), "small has no GLM model yet: start from the top")
}

// Provider đang dùng không còn model nào: báo lỗi để UI mở bảng đầy đủ thay vì
// một bảng rỗng.
func TestModelsDialogScopedToProviderWithoutModels(t *testing.T) {
	ws := &scopeWorkspace{recentWorkspace: recentWorkspace{cfg: twoProviderConfig()}}
	_, err := NewModelsForProvider(common.DefaultCommon(ws), "gone")
	require.ErrorIs(t, err, ErrNoProviderModels)
}
