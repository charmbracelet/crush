// Package hyper provides a fantasy.Provider that proxies requests to Hyper.
package hyper

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/object"
	"github.com/charmbracelet/crush/internal/event"
)

//go:generate wget -O provider.json https://console.charm.land/api/v1/provider

//go:embed provider.json
var embedded []byte

// Enabled returns true if hyper is enabled.
var Enabled = sync.OnceValue(func() bool {
	b, _ := strconv.ParseBool(
		cmp.Or(
			os.Getenv("HYPER"),
			os.Getenv("HYPERCRUSH"),
			os.Getenv("HYPER_ENABLE"),
			os.Getenv("HYPER_ENABLED"),
		),
	)
	return b
})

// Embedded returns the embedded Hyper provider.
var Embedded = sync.OnceValue(func() catwalk.Provider {
	var provider catwalk.Provider
	if err := json.Unmarshal(embedded, &provider); err != nil {
		slog.Error("Could not use embedded provider data", "err", err)
	}
	if e := os.Getenv("HYPER_URL"); e != "" {
		provider.APIEndpoint = e + "/api/v1/fantasy"
	}
	return provider
})

const (
	// Name is the default name of this meta provider.
	Name = "hyper"
	// defaultBaseURL is the default proxy URL.
	// TODO: change this to production URL when ready.
	defaultBaseURL = "https://console.charm.land"
)

// BaseURL returns the base URL, which is either $HYPER_URL or the default.
var BaseURL = sync.OnceValue(func() string {
	return cmp.Or(os.Getenv("HYPER_URL"), defaultBaseURL)
})

var ErrNoCredits = errors.New("you're out of credits")

type options struct {
	baseURL string
	apiKey  string
	name    string
	headers map[string]string
	client  *http.Client
}

// Option configures the proxy provider.
type Option = func(*options)

// New creates a new proxy provider.
func New(opts ...Option) (fantasy.Provider, error) {
	o := options{
		baseURL: BaseURL() + "/api/v1/fantasy",
		name:    Name,
		headers: map[string]string{
			"x-crush-id": event.GetID(),
		},
		client: &http.Client{Timeout: 0}, // stream-safe
	}
	for _, opt := range opts {
		opt(&o)
	}
	return &provider{options: o}, nil
}

// WithBaseURL sets the proxy base URL (e.g. http://localhost:8080).
func WithBaseURL(url string) Option { return func(o *options) { o.baseURL = url } }

// WithName sets the provider name.
func WithName(name string) Option { return func(o *options) { o.name = name } }

// WithHeaders sets extra headers sent to the proxy.
func WithHeaders(headers map[string]string) Option {
	return func(o *options) {
		maps.Copy(o.headers, headers)
	}
}

// WithHTTPClient sets custom HTTP client.
func WithHTTPClient(c *http.Client) Option { return func(o *options) { o.client = c } }

// WithAPIKey sets the API key.
func WithAPIKey(key string) Option {
	return func(o *options) {
		o.apiKey = key
	}
}

type provider struct{ options options }

func (p *provider) Name() string { return p.options.name }

// LanguageModel implements fantasy.Provider.
func (p *provider) LanguageModel(_ context.Context, modelID string) (fantasy.LanguageModel, error) {
	if modelID == "" {
		return nil, errors.New("missing model id")
	}
	return &languageModel{modelID: modelID, provider: p.options.name, opts: p.options}, nil
}

type languageModel struct {
	provider string
	modelID  string
	opts     options
}

// GenerateObject implements fantasy.LanguageModel.
func (m *languageModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return object.GenerateWithTool(ctx, m, call)
}

// StreamObject implements fantasy.LanguageModel.
func (m *languageModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return object.StreamWithTool(ctx, m, call)
}

func (m *languageModel) Provider() string { return m.provider }
func (m *languageModel) Model() string    { return m.modelID }

// Generate implements fantasy.LanguageModel by calling the proxy JSON endpoint.
func (m *languageModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	resp, err := m.doRequest(ctx, false, call)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := ioReadAllLimit(resp.Body, 64*1024)
		return nil, fmt.Errorf("proxy generate error: %s", strings.TrimSpace(string(b)))
	}
	var out fantasy.Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Stream implements fantasy.LanguageModel using SSE from the proxy.
func (m *languageModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	// Prefer explicit /stream endpoint
	resp, err := m.doRequest(ctx, true, call)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		_ = resp.Body.Close()
		return nil, toProviderError(resp, retryAfter(resp))
	case http.StatusUnauthorized:
		_ = resp.Body.Close()
		return nil, toProviderError(resp, "")
	case http.StatusPaymentRequired:
		_ = resp.Body.Close()
		return nil, ErrNoCredits
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		b, _ := ioReadAllLimit(resp.Body, 64*1024)
		return nil, &fantasy.ProviderError{
			Title:      "Stream Error",
			Message:    strings.TrimSpace(string(b)),
			StatusCode: resp.StatusCode,
		}
	}

	return func(yield func(fantasy.StreamPart) bool) {
		defer func() { _ = resp.Body.Close() }()
		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 4*1024*1024)

		var (
			event     string
			dataBuf   bytes.Buffer
			sawFinish bool
			dispatch  = func() bool {
				if dataBuf.Len() == 0 || event == "" {
					dataBuf.Reset()
					event = ""
					return true
				}
				var part fantasy.StreamPart
				if err := json.Unmarshal(dataBuf.Bytes(), &part); err != nil {
					return yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: err})
				}
				if part.Type == fantasy.StreamPartTypeFinish {
					sawFinish = true
				}
				ok := yield(part)
				dataBuf.Reset()
				event = ""
				return ok
			}
		)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" { // event boundary
				if !dispatch() {
					return
				}
				continue
			}
			if strings.HasPrefix(line, ":") { // comment / ping
				continue
			}
			if strings.HasPrefix(line, "event: ") {
				event = strings.TrimSpace(line[len("event: "):])
				continue
			}
			if strings.HasPrefix(line, "data: ") {
				if dataBuf.Len() > 0 {
					dataBuf.WriteByte('\n')
				}
				dataBuf.WriteString(line[len("data: "):])
				continue
			}
		}
		if err := scanner.Err(); err != nil &&
			!errors.Is(err, context.Canceled) &&
			!errors.Is(err, context.DeadlineExceeded) {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: err})
			return
		}
		// flush any pending data
		_ = dispatch()
		if !sawFinish {
			_ = yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish})
		}
	}, nil
}

func (m *languageModel) doRequest(ctx context.Context, stream bool, call fantasy.Call) (*http.Response, error) {
	addr, err := url.Parse(m.opts.baseURL)
	if err != nil {
		return nil, err
	}
	addr = addr.JoinPath(m.modelID)
	if stream {
		addr = addr.JoinPath("stream")
	} else {
		addr = addr.JoinPath("generate")
	}

	body, err := json.Marshal(call)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, addr.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	for k, v := range m.opts.headers {
		req.Header.Set(k, v)
	}

	if m.opts.apiKey != "" {
		req.Header.Set("Authorization", m.opts.apiKey)
	}
	return m.opts.client.Do(req)
}

// ioReadAllLimit reads up to n bytes.
func ioReadAllLimit(r io.Reader, n int64) ([]byte, error) {
	var b bytes.Buffer
	if n <= 0 {
		n = 1 << 20
	}
	lr := &io.LimitedReader{R: r, N: n}
	_, err := b.ReadFrom(lr)
	return b.Bytes(), err
}

func toProviderError(resp *http.Response, message string) error {
	return &fantasy.ProviderError{
		Title:      fantasy.ErrorTitleForStatusCode(resp.StatusCode),
		Message:    message,
		StatusCode: resp.StatusCode,
	}
}

func retryAfter(resp *http.Response) string {
	after, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err == nil && after > 0 {
		d := time.Duration(after) * time.Second
		return "Try again in " + d.String()
	}
	return "Try again later"
}
