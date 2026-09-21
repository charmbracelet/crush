package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/clipboard"
	"github.com/charmbracelet/crush/internal/honcho"
	"github.com/pkg/browser"
)

// loginHoncho signs Crush in to a Honcho deployment so it can keep
// memory across sessions.
//
// Honcho's authorization server advertises a device grant but rejects
// it for dynamically registered clients, so this uses authorization
// code with PKCE over a loopback redirect: the flow RFC 8252
// recommends for native applications, and the one that works.
func loginHoncho(force bool) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Honoring an existing sign-in keeps an accidental `crush login
	// honcho` from discarding a working credential.
	if !force && honcho.SignedIn() {
		fmt.Println()
		fmt.Println("Already signed in to Honcho. Use -f to sign in again.")
		return nil
	}

	baseURL := honcho.Resolve(honcho.Config{}).BaseURL
	if baseURL == "" {
		baseURL = honcho.DefaultBaseURL
	}

	fmt.Println()
	fmt.Println("Connecting to", baseURL)

	flow, err := honcho.StartAuth(ctx, baseURL)
	if err != nil {
		return err
	}
	// Wait closes the flow, but an early return before then would
	// otherwise leak the loopback listener.
	defer flow.Close()

	url := flow.URL
	clipboard.WriteText(url)
	fmt.Println()
	fmt.Println("Press enter to open this URL and connect your Honcho account:")
	fmt.Println()
	lipgloss.Println(lipgloss.NewStyle().Hyperlink(url, "id=honcho").Render(url))
	fmt.Println()
	fmt.Println("(It's on your clipboard too, in case the browser doesn't open.)")
	fmt.Println()
	// Inlined rather than shared: the equivalent helper lives on a
	// branch this one does not depend on, and a second copy of it
	// would collide when both land.
	_, _ = fmt.Scanln()
	if err := browser.OpenURL(url); err != nil {
		fmt.Println("Could not open the URL. You'll need to manually open the URL in your browser.")
	}

	fmt.Println()
	fmt.Println("Waiting for authorization...")
	token, err := flow.Wait(ctx)
	if err != nil {
		return err
	}
	if err := honcho.SaveToken(token); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Crush will now remember what you work on across sessions.")
	fmt.Println("Run `crush logout honcho` to disconnect, or `honcho status` in a session to check on it.")
	return nil
}

// logoutHoncho removes the stored Honcho sign-in.
func logoutHoncho() error {
	if !honcho.SignedIn() {
		fmt.Println("Not signed in to Honcho.")
		return nil
	}
	if err := honcho.DeleteToken(); err != nil {
		return err
	}
	fmt.Println("Signed out of Honcho. Memory already stored is untouched; sign in again to reach it.")
	return nil
}
