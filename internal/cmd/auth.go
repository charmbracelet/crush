package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/spf13/cobra"
)

const (
	copilotClientID = "Iv1.b507a08c87ecfe98"

	githubDeviceCodeURL   = "https://github.com/login/device/code"
	githubAccessTokenURL  = "https://github.com/login/oauth/access_token"
	githubCopilotTokenURL = "https://api.github.com/copilot_internal/v2/token"

	copilotUserAgent           = "GitHubCopilotChat/0.32.4"
	copilotEditorVersion       = "vscode/1.105.1"
	copilotEditorPluginVersion = "copilot-chat/0.32.4"
	copilotIntegrationID       = "vscode-chat"
)

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

type CopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type CopilotAuthData struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// authLoginModel is the Bubble Tea model for the login UI
type authLoginModel struct {
	spinner         *anim.Anim
	state           authState
	deviceCode      *DeviceCodeResponse
	errorMsg        string
	ctx             context.Context
	cancel          context.CancelFunc
	accessToken     string
	copilotToken    *CopilotTokenResponse
	cmd             *cobra.Command
	width           int
	height          int
	cancelRequested bool
}

type authState int

const (
	stateRequesting authState = iota
	stateWaiting
	stateFetchingToken
	stateSuccess
	stateError
	stateCancelled
)

type deviceCodeMsg *DeviceCodeResponse
type accessTokenMsg string
type copilotTokenMsg *CopilotTokenResponse
type authErrorMsg error

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication for providers",
	Long:  `Manage authentication for various AI providers that require OAuth or special authentication flows.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login <provider>",
	Short: "Authenticate with a provider",
	Long: `Authenticate with a provider using OAuth device flow.

Supported providers:
  - github-copilot: GitHub Copilot (requires active subscription)`,
	Example: `
# Login to GitHub Copilot
crush auth login github-copilot

# Login with custom data directory
crush auth login github-copilot -D /path/to/data
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := args[0]

		switch provider {
		case "github-copilot", "copilot":
			return loginGitHubCopilot(cmd)
		default:
			return fmt.Errorf("unsupported provider: %s\n\nSupported providers:\n  - github-copilot", provider)
		}
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status [provider]",
	Short: "Check authentication status",
	Long:  `Check the authentication status for providers.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir, _ := cmd.Flags().GetString("data-dir")
		cwd, err := ResolveCwd(cmd)
		if err != nil {
			return err
		}

		cfg, err := config.Load(cwd, dataDir, false)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %v", err)
		}

		if len(args) == 0 || args[0] == "github-copilot" || args[0] == "copilot" {
			return checkCopilotStatus(cfg)
		}

		return fmt.Errorf("unsupported provider: %s", args[0])
	},
}

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
}

func (m authLoginModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Init(),
		m.requestDeviceCode,
	)
}

func (m authLoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.cancelRequested = true
			m.cancel()
			return m, tea.Quit
		}

	case deviceCodeMsg:
		m.deviceCode = msg
		m.state = stateWaiting
		m.spinner.SetLabel("Waiting for authorization")
		return m, m.pollForAccessToken

	case accessTokenMsg:
		m.accessToken = string(msg)
		m.state = stateFetchingToken
		m.spinner.SetLabel("Fetching Copilot token")
		return m, m.getCopilotTokenCmd

	case copilotTokenMsg:
		m.copilotToken = msg
		m.state = stateSuccess
		return m, tea.Sequence(
			tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
				return tea.QuitMsg{}
			}),
		)

	case authErrorMsg:
		if m.ctx.Err() == context.Canceled && m.cancelRequested {
			m.state = stateCancelled
		} else {
			m.state = stateError
			m.errorMsg = msg.Error()
		}
		return m, tea.Quit

	case anim.StepMsg:
		mm, cmd := m.spinner.Update(msg)
		m.spinner = mm.(*anim.Anim)
		return m, cmd
	}

	return m, nil
}

func (m authLoginModel) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}

	var s strings.Builder

	// Show spinner and current state
	if m.state != stateSuccess && m.state != stateError && m.state != stateCancelled {
		s.WriteString(m.spinner.View())
		s.WriteString("\n\n")
	}

	switch m.state {
	case stateRequesting:
		// Spinner already shows the message

	case stateWaiting:
		if m.deviceCode != nil {
			urlStyle := lipgloss.NewStyle().
				Foreground(charmtone.Malibu).
				Underline(true)

			s.WriteString("Visit: ")
			s.WriteString(urlStyle.Render(m.deviceCode.VerificationURI))
			s.WriteString("\n\n")

			codeStyle := lipgloss.NewStyle().
				Foreground(charmtone.Guac).
				Bold(true)

			s.WriteString("Code:  ")
			s.WriteString(codeStyle.Render(m.deviceCode.UserCode))
			s.WriteString("\n")
		}

	case stateFetchingToken:
		// Spinner already shows the message

	case stateSuccess:
		successStyle := lipgloss.NewStyle().
			Foreground(charmtone.Guac)

		s.WriteString(successStyle.Render("✓ Authentication successful"))
		s.WriteString("\n\n")

		if m.copilotToken != nil {
			expiresAt := time.Unix(m.copilotToken.ExpiresAt, 0)
			dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
			s.WriteString(dimStyle.Render(fmt.Sprintf("Token expires in %s", time.Until(expiresAt).Round(time.Minute))))
			s.WriteString("\n")
		}

	case stateError:
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff5555"))

		s.WriteString(errorStyle.Render("✗ " + m.errorMsg))
		s.WriteString("\n")

	case stateCancelled:
		cancelStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

		s.WriteString(cancelStyle.Render("Cancelled"))
		s.WriteString("\n")
	}

	// Help text
	if m.state == stateWaiting || m.state == stateRequesting {
		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Italic(true)
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("Press Ctrl+C to cancel"))
	}

	return tea.NewView(s.String())
}

// Commands for async operations
func (m authLoginModel) requestDeviceCode() tea.Msg {
	deviceCode, err := requestDeviceCode()
	if err != nil {
		return authErrorMsg(fmt.Errorf("failed to request device code: %w", err))
	}
	return deviceCodeMsg(deviceCode)
}

func (m authLoginModel) pollForAccessToken() tea.Msg {
	token, err := pollForAccessToken(m.ctx, m.deviceCode)
	if err != nil {
		return authErrorMsg(fmt.Errorf("authorization failed: %w", err))
	}
	return accessTokenMsg(token)
}

func (m authLoginModel) getCopilotTokenCmd() tea.Msg {
	token, err := getCopilotToken(m.accessToken)
	if err != nil {
		return authErrorMsg(fmt.Errorf("failed to get Copilot token: %w", err))
	}
	return copilotTokenMsg(token)
}

func loginGitHubCopilot(cmd *cobra.Command) error {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get current theme colors
	t := styles.CurrentTheme()

	// Create animated spinner with theme gradient colors
	spinner := anim.New(anim.Settings{
		Size:        10,
		Label:       "Requesting device code",
		LabelColor:  t.FgBase,
		GradColorA:  t.Primary,
		GradColorB:  t.Secondary,
		CycleColors: true,
	})

	// Create the model
	model := authLoginModel{
		spinner: spinner,
		state:   stateRequesting,
		ctx:     ctx,
		cancel:  cancel,
		cmd:     cmd,
	}

	// Handle Ctrl+C signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		model.cancelRequested = true
		cancel()
	}()

	// Run the Bubble Tea program
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running auth UI: %w", err)
	}

	m, ok := finalModel.(authLoginModel)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	// Check final state
	if m.state == stateCancelled {
		return fmt.Errorf("authentication cancelled")
	}

	if m.state == stateError {
		return fmt.Errorf("%s", m.errorMsg)
	}

	if m.state != stateSuccess {
		return fmt.Errorf("authentication failed")
	}

	// Save the authentication
	dataDir, _ := cmd.Flags().GetString("data-dir")
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return err
	}

	cfg, err := config.Load(cwd, dataDir, false)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}

	if err := saveCopilotAuth(cfg, m.accessToken, m.copilotToken); err != nil {
		return fmt.Errorf("failed to save authentication: %w", err)
	}

	return nil
}

func checkCopilotStatus(cfg *config.Config) error {
	// Check global directory first, then local
	authFile := filepath.Join(home.Dir(), ".crush", "copilot-auth.json")

	data, err := os.ReadFile(authFile)
	if os.IsNotExist(err) {
		// Try local data directory as fallback
		authFile = filepath.Join(cfg.Options.DataDirectory, "copilot-auth.json")
		data, err = os.ReadFile(authFile)
	}
	if os.IsNotExist(err) {
		fmt.Println("GitHub Copilot: Not authenticated")
		fmt.Println()
		fmt.Println("Run 'crush auth login github-copilot' to authenticate.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read auth file: %w", err)
	}

	var authData CopilotAuthData
	if err := json.Unmarshal(data, &authData); err != nil {
		return fmt.Errorf("failed to parse auth file: %w", err)
	}

	expiresAt := time.Unix(authData.ExpiresAt, 0)
	if time.Now().After(expiresAt) {
		fmt.Println("GitHub Copilot: Token expired")
		fmt.Printf("   Expired at: %s\n", expiresAt.Format(time.RFC1123))
		fmt.Println()
		fmt.Println("Run 'crush auth login github-copilot' to re-authenticate.")
	} else {
		fmt.Println("GitHub Copilot: Authenticated ✅")
		fmt.Printf("   Expires at: %s\n", expiresAt.Format(time.RFC1123))
		fmt.Printf("   (in %s)\n", time.Until(expiresAt).Round(time.Minute))
	}

	return nil
}

func saveCopilotAuth(cfg *config.Config, refreshToken string, copilotToken *CopilotTokenResponse) error {
	authData := CopilotAuthData{
		RefreshToken: refreshToken,
		AccessToken:  copilotToken.Token,
		ExpiresAt:    copilotToken.ExpiresAt,
	}

	data, err := json.MarshalIndent(authData, "", "  ")
	if err != nil {
		return err
	}

	// Save to global data directory so it's shared across all projects
	globalDir := filepath.Join(home.Dir(), ".crush")
	if err := os.MkdirAll(globalDir, 0o700); err != nil {
		return fmt.Errorf("failed to create global data directory: %w", err)
	}

	authFile := filepath.Join(globalDir, "copilot-auth.json")
	return os.WriteFile(authFile, data, 0o600)
}

func requestDeviceCode() (*DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", copilotClientID)
	data.Set("scope", "read:user")

	req, err := http.NewRequest("POST", githubDeviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", copilotUserAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed: %s - %s", resp.Status, string(body))
	}

	var deviceCode DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
		return nil, err
	}

	return &deviceCode, nil
}

func pollForAccessToken(ctx context.Context, deviceCode *DeviceCodeResponse) (string, error) {
	interval := max(deviceCode.Interval, 5)

	deadline := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		data := url.Values{}
		data.Set("client_id", copilotClientID)
		data.Set("device_code", deviceCode.DeviceCode)
		data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		req, err := http.NewRequestWithContext(ctx, "POST", githubAccessTokenURL, strings.NewReader(data.Encode()))
		if err != nil {
			return "", err
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", copilotUserAgent)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", err
		}

		var tokenResp AccessTokenResponse
		if err := json.Unmarshal(body, &tokenResp); err != nil {
			return "", err
		}

		if tokenResp.AccessToken != "" {
			return tokenResp.AccessToken, nil
		}

		if tokenResp.Error == "slow_down" {
			interval += 5
			ticker.Reset(time.Duration(interval) * time.Second)
		}

		if tokenResp.Error != "" && tokenResp.Error != "authorization_pending" && tokenResp.Error != "slow_down" {
			return "", fmt.Errorf("authorization failed: %s", tokenResp.Error)
		}

		// Wait for next tick or cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			continue
		}
	}

	return "", fmt.Errorf("authorization timed out")
}

func getCopilotToken(accessToken string) (*CopilotTokenResponse, error) {
	req, err := http.NewRequest("GET", githubCopilotTokenURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", copilotUserAgent)
	req.Header.Set("Editor-Version", copilotEditorVersion)
	req.Header.Set("Editor-Plugin-Version", copilotEditorPluginVersion)
	req.Header.Set("Copilot-Integration-Id", copilotIntegrationID)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("copilot token request failed: %s\n\n"+
				"This usually means:\n"+
				"  1. Your GitHub account doesn't have an active Copilot subscription\n"+
				"  2. GitHub Copilot Chat is not enabled for your account\n"+
				"  3. You're using a GitHub organization that doesn't have Copilot access\n\n"+
				"Please ensure you have GitHub Copilot enabled at: https://github.com/settings/copilot\n\n"+
				"Response: %s", resp.Status, string(body))
		}
		return nil, fmt.Errorf("copilot token request failed: %s - %s", resp.Status, string(body))
	}

	var copilotToken CopilotTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&copilotToken); err != nil {
		return nil, err
	}

	return &copilotToken, nil
}
