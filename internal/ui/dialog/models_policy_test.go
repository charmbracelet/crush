package dialog

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/execpolicy"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// Hộp thoại chọn model chỉ hiện model được phép; nhóm không còn model nào bị
// bỏ (provider không có model được phép coi như tắt).
func TestAllowedModelGroups(t *testing.T) {
	t.Parallel()
	st := styles.DefaultTheme()
	zai := catwalk.Provider{ID: "zai", Name: "Z.ai"}
	openai := catwalk.Provider{ID: "openai", Name: "OpenAI"}
	mk := func(p catwalk.Provider, ids ...string) ModelGroup {
		g := NewModelGroup(&st, p.Name, true)
		for _, id := range ids {
			g.AppendItems(NewModelItem(&st, p, catwalk.Model{ID: id, Name: id}, ModelTypeLarge, false))
		}
		return g
	}
	groups := []ModelGroup{mk(zai, "glm-5.3", "glm-5.3-flash", "glm-4.6", "glm-5.4"), mk(openai, "gpt-6.1-sol")}
	pol, err := execpolicy.Parse("zai:glm-5.3 zai:glm-5.3-flash")
	require.NoError(t, err)

	got := allowedModelGroups(&st, groups, pol)
	require.Len(t, got, 1, "OpenAI has no allowed model, so its group goes")
	var ids []string
	for _, item := range got[0].Items {
		ids = append(ids, item.model.ID)
	}
	require.Equal(t, []string{"glm-5.3", "glm-5.3-flash"}, ids)
	require.True(t, modelGroupsContain(got, modelKey("zai", "glm-5.3")))
	require.False(t, modelGroupsContain(got, modelKey("zai", "glm-4.6")))
}
