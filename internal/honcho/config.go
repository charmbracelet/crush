package honcho

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// HostKey identifies Crush's section inside the shared Honcho config
// file. Sibling integrations use their own key under the same object,
// so one `honcho_setup` serves every harness on the machine.
const HostKey = "crush"

// Defaults applied when neither the shared file nor Crush config says
// otherwise.
const (
	DefaultWorkspace = "crush"
	DefaultAgentPeer = "crush"
	DefaultPeerName  = "user"
)

// RecallMode selects which channels carry memory into the model.
type RecallMode string

// Recall modes.
const (
	// RecallHybrid seals a session-stable snapshot into the system
	// prompt and refreshes volatile recall at the tail each turn.
	RecallHybrid RecallMode = "hybrid"
	// RecallContext is identical to hybrid today and exists so the
	// shared config file round-trips values written by sibling
	// integrations without loss.
	RecallContext RecallMode = "context"
	// RecallTools injects nothing automatically. The model reaches
	// memory only through the honcho_* tools.
	RecallTools RecallMode = "tools"
)

// ObservationMode selects whose collection conclusions are written to
// and queried from.
type ObservationMode string

// Observation modes.
const (
	// ObserveUnified uses the user's own self-collection, so several
	// agents sharing a workspace see each other's conclusions.
	ObserveUnified ObservationMode = "unified"
	// ObserveDirectional uses this agent's view of the user, keeping
	// each agent's memory isolated.
	ObserveDirectional ObservationMode = "directional"
)

// SessionStrategy selects what maps onto a Honcho session, which is
// the boundary Honcho summarizes and scopes recall to.
type SessionStrategy string

// Session strategies.
const (
	// StrategyPerDirectory gives each working directory its own
	// session, so memory accumulates per project. The default.
	StrategyPerDirectory SessionStrategy = "per-directory"
	// StrategyPerRepo scopes to the repository root, uniting several
	// entry directories in one repo.
	StrategyPerRepo SessionStrategy = "per-repo"
	// StrategyGitBranch scopes to the checked-out branch.
	StrategyGitBranch SessionStrategy = "git-branch"
	// StrategyPerSession scopes to a single Crush session.
	StrategyPerSession SessionStrategy = "per-session"
	// StrategyGlobal puts all work in one session.
	StrategyGlobal SessionStrategy = "global"
)

// Config is the resolved Honcho configuration, merged from the shared
// file, Crush's own config, and the environment.
type Config struct {
	// Enabled reports whether the integration should run at all.
	Enabled bool `json:"enabled,omitempty"`
	// APIKey authenticates to the deployment.
	APIKey string `json:"api_key,omitempty"`
	// BaseURL is the deployment. Empty means Honcho Cloud.
	BaseURL string `json:"base_url,omitempty"`
	// Workspace isolates this application's peers and sessions.
	Workspace string `json:"workspace,omitempty"`
	// PeerName identifies the human. It is the peer Honcho builds a
	// representation of.
	PeerName string `json:"peer_name,omitempty"`
	// AgentPeer identifies Crush itself.
	AgentPeer string `json:"agent_peer,omitempty"`
	// RecallMode selects the injection channels.
	RecallMode RecallMode `json:"recall_mode,omitempty"`
	// ObservationMode selects the conclusion collection.
	ObservationMode ObservationMode `json:"observation_mode,omitempty"`
	// SessionStrategy selects what maps onto a Honcho session.
	SessionStrategy SessionStrategy `json:"session_strategy,omitempty"`
	// AgentObserveMe asks Honcho to model Crush itself, not just the
	// user. Off by default: the point is to remember the human.
	AgentObserveMe bool `json:"agent_observe_me,omitempty"`
	// CaptureTools records a one-line summary of meaningful tool
	// calls so memory reflects what was done, not only what was said.
	CaptureTools bool `json:"capture_tools,omitempty"`
	// MaxConclusions caps how many conclusions enter an injected
	// context block.
	MaxConclusions int `json:"max_conclusions,omitempty"`
	// ContextTokens caps the assembled per-turn recall block.
	ContextTokens int `json:"context_tokens,omitempty"`
}

// sharedFile mirrors ~/.honcho/config.json. Sibling integrations own
// their own entries in Hosts, so unknown keys must survive a
// read-modify-write untouched.
type sharedFile struct {
	APIKey   string                     `json:"apiKey,omitempty"`
	BaseURL  string                     `json:"baseUrl,omitempty"`
	PeerName string                     `json:"peerName,omitempty"`
	Hosts    map[string]json.RawMessage `json:"hosts,omitempty"`

	// rest preserves any top-level keys this version does not know
	// about, so writing the file back never drops a sibling's data.
	rest map[string]json.RawMessage
}

// hostSection is Crush's entry under hosts in the shared file. Field
// names follow the shared file's camelCase convention rather than
// Crush's snake_case, because the file is not ours alone.
type hostSection struct {
	APIKey          string `json:"apiKey,omitempty"`
	Workspace       string `json:"workspace,omitempty"`
	AIPeer          string `json:"aiPeer,omitempty"`
	RecallMode      string `json:"recallMode,omitempty"`
	ObservationMode string `json:"observationMode,omitempty"`
	SessionStrategy string `json:"sessionStrategy,omitempty"`
	AgentObserveMe  *bool  `json:"agentObserveMe,omitempty"`
	CaptureTools    *bool  `json:"captureTools,omitempty"`
}

// SharedConfigPath returns the path of the cross-harness Honcho config
// file, honouring HONCHO_CONFIG_DIR.
func SharedConfigPath() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("HONCHO_CONFIG_DIR")); dir != "" {
		return filepath.Join(dir, "config.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("honcho: locate home directory: %w", err)
	}
	return filepath.Join(home, ".honcho", "config.json"), nil
}

// loadShared reads the shared config file. A missing file is not an
// error: it simply contributes nothing to the merge.
func loadShared() (*sharedFile, error) {
	path, err := SharedConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &sharedFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("honcho: read %s: %w", path, err)
	}

	var f sharedFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("honcho: parse %s: %w", path, err)
	}
	// Capture unknown top-level keys so a later write preserves them.
	if err := json.Unmarshal(data, &f.rest); err != nil {
		return nil, fmt.Errorf("honcho: parse %s: %w", path, err)
	}
	return &f, nil
}

// Resolve merges configuration from the shared file, Crush's own
// config, and the environment, in increasing order of precedence.
//
// overrides carries the values Crush's own config supplied; zero
// fields defer to lower layers. It returns a usable Config even when
// the shared file is missing or malformed, because failing to read a
// sibling's config should disable memory rather than break startup.
func Resolve(overrides Config) Config {
	cfg := Config{
		Workspace:       DefaultWorkspace,
		PeerName:        DefaultPeerName,
		AgentPeer:       DefaultAgentPeer,
		RecallMode:      RecallHybrid,
		ObservationMode: ObserveUnified,
		SessionStrategy: StrategyPerDirectory,
		CaptureTools:    true,
		MaxConclusions:  8,
		ContextTokens:   2000,
	}

	// Layer 1: the shared cross-harness file.
	if shared, err := loadShared(); err == nil {
		applyString(&cfg.APIKey, shared.APIKey)
		applyString(&cfg.BaseURL, shared.BaseURL)
		applyString(&cfg.PeerName, shared.PeerName)

		if raw, ok := shared.Hosts[HostKey]; ok {
			var host hostSection
			if json.Unmarshal(raw, &host) == nil {
				applyString(&cfg.APIKey, host.APIKey)
				applyString(&cfg.Workspace, host.Workspace)
				applyString(&cfg.AgentPeer, host.AIPeer)
				applyString((*string)(&cfg.RecallMode), host.RecallMode)
				applyString((*string)(&cfg.ObservationMode), host.ObservationMode)
				applyString((*string)(&cfg.SessionStrategy), host.SessionStrategy)
				applyBool(&cfg.AgentObserveMe, host.AgentObserveMe)
				applyBool(&cfg.CaptureTools, host.CaptureTools)
			}
		}
	} else {
		logFailure("load shared config", err)
	}

	// Layer 2: Crush's own config.
	applyString(&cfg.APIKey, overrides.APIKey)
	applyString(&cfg.BaseURL, overrides.BaseURL)
	applyString(&cfg.Workspace, overrides.Workspace)
	applyString(&cfg.PeerName, overrides.PeerName)
	applyString(&cfg.AgentPeer, overrides.AgentPeer)
	applyString((*string)(&cfg.RecallMode), string(overrides.RecallMode))
	applyString((*string)(&cfg.ObservationMode), string(overrides.ObservationMode))
	applyString((*string)(&cfg.SessionStrategy), string(overrides.SessionStrategy))
	if overrides.MaxConclusions > 0 {
		cfg.MaxConclusions = overrides.MaxConclusions
	}
	if overrides.ContextTokens > 0 {
		cfg.ContextTokens = overrides.ContextTokens
	}
	// Booleans from Crush config are only meaningful when the section
	// exists at all; Enabled gates the whole integration, so an
	// explicit true here is what turns the others on.
	if overrides.Enabled {
		cfg.Enabled = true
		cfg.AgentObserveMe = overrides.AgentObserveMe
		cfg.CaptureTools = overrides.CaptureTools
	}

	// Layer 3: the environment.
	applyString(&cfg.APIKey, os.Getenv("HONCHO_API_KEY"))
	applyString(&cfg.BaseURL, firstNonEmpty(os.Getenv("HONCHO_URL"), os.Getenv("HONCHO_BASE_URL")))
	applyString(&cfg.Workspace, firstNonEmpty(os.Getenv("HONCHO_WORKSPACE"), os.Getenv("HONCHO_WORKSPACE_ID")))
	applyString(&cfg.PeerName, os.Getenv("HONCHO_PEER_NAME"))
	applyString(&cfg.AgentPeer, os.Getenv("HONCHO_AI_PEER"))

	cfg.Workspace = NormalizeID(cfg.Workspace)
	cfg.AgentPeer = NormalizeID(cfg.AgentPeer)

	// A key, a stored sign-in, or an explicitly chosen deployment is
	// each an unambiguous statement of intent, so any of them turns
	// the integration on.
	if cfg.APIKey != "" || SignedIn() || (cfg.BaseURL != "" && cfg.BaseURL != DefaultBaseURL) {
		cfg.Enabled = true
	}
	return cfg
}

// Validate reports whether the resolved config can actually reach a
// deployment. A local deployment needs no key; Honcho Cloud does.
func (c Config) Validate() error {
	if !c.Enabled {
		return ErrDisabled
	}
	if c.Workspace == "" {
		return errors.New("honcho: workspace is required")
	}
	if c.PeerName == "" {
		return errors.New("honcho: peer name is required")
	}
	// A browser sign-in is a credential too: Client.authorization
	// falls back to the stored OAuth token whenever no key is set.
	// Counting only API keys here rejected every user who connected
	// from the command palette, which is the documented way in.
	if c.APIKey == "" && !SignedIn() && !c.IsLocal() {
		return errors.New("honcho: sign in with `crush login honcho`, or set an API key, to use Honcho Cloud")
	}
	if NormalizeID(c.PeerName) == c.AgentPeer {
		return fmt.Errorf("honcho: user peer and agent peer are both %q; set a distinct peer_name or agent_peer", c.AgentPeer)
	}
	return nil
}

// IsLocal reports whether the configured deployment is loopback, in
// which case authentication is generally unnecessary.
func (c Config) IsLocal() bool {
	if c.BaseURL == "" {
		return false
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// SaveShared writes Crush's section into the shared config file,
// leaving every other host's settings and any unknown top-level keys
// untouched. Credentials are written with owner-only permissions.
func SaveShared(cfg Config) error {
	path, err := SharedConfigPath()
	if err != nil {
		return err
	}
	shared, err := loadShared()
	if err != nil {
		// Refuse to clobber a file we could not parse: a sibling's
		// configuration is not ours to discard.
		return err
	}

	out := shared.rest
	if out == nil {
		out = map[string]json.RawMessage{}
	}

	setJSON := func(key string, value any) error {
		raw, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("honcho: encode %s: %w", key, err)
		}
		out[key] = raw
		return nil
	}

	// Credentials and identity are shared across harnesses; the rest
	// is Crush-specific and belongs under our host key.
	if cfg.APIKey != "" {
		if err := setJSON("apiKey", cfg.APIKey); err != nil {
			return err
		}
	}
	if cfg.BaseURL != "" {
		if err := setJSON("baseUrl", cfg.BaseURL); err != nil {
			return err
		}
	}
	if cfg.PeerName != "" {
		if err := setJSON("peerName", cfg.PeerName); err != nil {
			return err
		}
	}

	hosts := map[string]json.RawMessage{}
	if raw, ok := out["hosts"]; ok {
		if err := json.Unmarshal(raw, &hosts); err != nil {
			return fmt.Errorf("honcho: parse hosts in %s: %w", path, err)
		}
	}
	section := hostSection{
		Workspace:       cfg.Workspace,
		AIPeer:          cfg.AgentPeer,
		RecallMode:      string(cfg.RecallMode),
		ObservationMode: string(cfg.ObservationMode),
		SessionStrategy: string(cfg.SessionStrategy),
		AgentObserveMe:  &cfg.AgentObserveMe,
		CaptureTools:    &cfg.CaptureTools,
	}
	sectionRaw, err := json.Marshal(section)
	if err != nil {
		return fmt.Errorf("honcho: encode host section: %w", err)
	}
	hosts[HostKey] = sectionRaw
	if err := setJSON("hosts", hosts); err != nil {
		return err
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("honcho: encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("honcho: create config directory: %w", err)
	}
	// The file holds an API key, so it is owner-readable only.
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("honcho: write %s: %w", path, err)
	}
	return nil
}

// applyString overwrites dst when v is non-empty, so higher config
// layers win and empty values defer.
func applyString(dst *string, v string) {
	if s := strings.TrimSpace(v); s != "" {
		*dst = s
	}
}

// applyBool overwrites dst when v was explicitly set.
func applyBool(dst *bool, v *bool) {
	if v != nil {
		*dst = *v
	}
}

// firstNonEmpty returns the first non-blank argument.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
