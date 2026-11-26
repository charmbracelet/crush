// Package httpx holds HTTP transports shared by every provider build.
package httpx

import (
	"crypto/tls"
	"net/http"
	"sync"
)

// HTTP1Transport returns the process-wide HTTP/1.1 pool; do not modify it.
// It is shared so rebuilding a provider never leaves an idle pool behind.
var HTTP1Transport = sync.OnceValue(func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Protocols = new(http.Protocols)
	t.Protocols.SetHTTP1(true)
	t.ForceAttemptHTTP2 = false
	delete(t.TLSNextProto, "h2")
	if t.TLSClientConfig == nil {
		t.TLSClientConfig = &tls.Config{}
	}
	t.TLSClientConfig.NextProtos = []string{"http/1.1"}
	return t
})
