package server

import (
	"context"
	"net/http"
)

// HTTPContextFunc is a function that takes an existing context and the current
// request and returns a potentially modified context based on the request
// content. This can be used to inject context values from headers, for example.
type HTTPContextFunc func(ctx context.Context, r *http.Request) context.Context
