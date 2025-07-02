package client

import "encoding/json"

type RPCResponse struct {
	Error    *string
	Response *json.RawMessage
}
