// Package protocol implements the JSON-RPC 2.0 framing and the ACP v1
// message types shared by the ACP server and client tracks.
package protocol

import (
	"encoding/json"
	"fmt"
)

// JSON-RPC 2.0 protocol version.
const (
	JSONRPCVersion = "2.0"
)

// Error codes defined by JSON-RPC 2.0 and ACP.
const (
	ErrCodeParse       = -32700
	ErrCodeInvalidReq  = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams = -32602
	ErrCodeInternal    = -32603
	ErrCodeCancelled   = -32800
	ErrCodeAuthRequired = -32000
	ErrCodeResourceNotFound = -32002
)

// Message is the union of all JSON-RPC 2.0 message kinds.
type Message struct {
	// JSON-RPC fields.
	JSONRPC string `json:"jsonrpc"`
	ID      *int   `json:"id,omitempty"`
	Method  string `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	if e.Data != nil {
		return fmt.Sprintf("jsonrpc error %d: %s (%v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

// NewRequest builds a JSON-RPC 2.0 request message.
func NewRequest(id int, method string, params any) (*Message, error) {
	var p json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		p = b
	}
	return &Message{
		JSONRPC: JSONRPCVersion,
		ID:      &id,
		Method:  method,
		Params:  p,
	}, nil
}

// NewNotification builds a JSON-RPC 2.0 notification message.
func NewNotification(method string, params any) (*Message, error) {
	var p json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		p = b
	}
	return &Message{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  p,
	}, nil
}

// NewResponse builds a JSON-RPC 2.0 response message with a result.
func NewResponse(id int, result any) (*Message, error) {
	var r json.RawMessage
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal result: %w", err)
		}
		r = b
	}
	return &Message{
		JSONRPC: JSONRPCVersion,
		ID:      &id,
		Result:  r,
	}, nil
}

// NewErrorResponse builds a JSON-RPC 2.0 response message with an error.
func NewErrorResponse(id int, err *RPCError) *Message {
	return &Message{
		JSONRPC: JSONRPCVersion,
		ID:      &id,
		Error:   err,
	}
}

// IsRequest reports whether the message is a request (has an id and method).
func (m *Message) IsRequest() bool {
	return m != nil && m.ID != nil && m.Method != ""
}

// IsNotification reports whether the message is a notification (no id, has method).
func (m *Message) IsNotification() bool {
	return m != nil && m.ID == nil && m.Method != ""
}

// IsResponse reports whether the message is a response (has an id, no method).
func (m *Message) IsResponse() bool {
	return m != nil && m.ID != nil && m.Method == ""
}

// DecodeParams unmarshals the message params into v.
func (m *Message) DecodeParams(v any) error {
	if len(m.Params) == 0 {
		return nil
	}
	return json.Unmarshal(m.Params, v)
}

// DecodeResult unmarshals the message result into v.
func (m *Message) DecodeResult(v any) error {
	if len(m.Result) == 0 {
		return nil
	}
	return json.Unmarshal(m.Result, v)
}

// DecodeError unmarshals the message error.
func (m *Message) DecodeError() *RPCError {
	return m.Error
}