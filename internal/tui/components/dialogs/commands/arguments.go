package commands

// ShowArgumentsDialogMsg is a message that is sent to show the arguments dialog.
type ShowArgumentsDialogMsg struct {
	CommandID string
	Content   string
	ArgNames  []string
}

// CloseArgumentsDialogMsg is a message that is sent when the arguments dialog is closed.
type CloseArgumentsDialogMsg struct {
	Submit    bool
	CommandID string
	Content   string
	Args      map[string]string
}
