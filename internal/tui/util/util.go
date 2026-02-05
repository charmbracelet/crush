package util

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/uiutil"
)

type Cursor = uiutil.Cursor

type Model interface {
	Init() tea.Cmd
	Update(tea.Msg) (Model, tea.Cmd)
	View() string
}

func CmdHandler(msg tea.Msg) tea.Cmd {
	return uiutil.CmdHandler(msg)
}

func ReportError(err error) tea.Cmd {
	return uiutil.ReportError(err)
}

type InfoType = uiutil.InfoType

const (
	InfoTypeInfo    = uiutil.InfoTypeInfo
	InfoTypeSuccess = uiutil.InfoTypeSuccess
	InfoTypeWarn    = uiutil.InfoTypeWarn
	InfoTypeError   = uiutil.InfoTypeError
	InfoTypeUpdate  = uiutil.InfoTypeUpdate
)

func ReportInfo(info string) tea.Cmd {
	return uiutil.ReportInfo(info)
}

func ReportWarn(warn string) tea.Cmd {
	return uiutil.ReportWarn(warn)
}

type (
	InfoMsg        = uiutil.InfoMsg
	ClearStatusMsg = uiutil.ClearStatusMsg
)
