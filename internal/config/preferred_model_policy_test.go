package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// Model ngoài ranh giới không được ghi xuống đĩa (lần mở sau sẽ dừng lúc dựng
// agent mà không sửa được từ TUI).
func TestUpdatePreferredModelRefusesDeniedModel(t *testing.T) {
	// Không parallel: đổi biến cấp gói.
	prev := checkPreferredModel
	t.Cleanup(func() { checkPreferredModel = prev })
	denied := errors.New("MODEL_NOT_ALLOWED")
	checkPreferredModel = func(m SelectedModel) error {
		if m.Model == "bad" {
			return denied
		}
		return nil
	}
	s := &ConfigStore{}
	err := s.UpdatePreferredModel(ScopeWorkspace, SelectedModelTypeLarge, SelectedModel{Provider: "zai", Model: "bad"})
	require.ErrorIs(t, err, denied)
}
