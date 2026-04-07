package proto

// ServerControl represents a server control request.
type ServerControl struct {
	Command string `json:"command"`
}
