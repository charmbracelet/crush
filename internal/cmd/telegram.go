package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/charmbracelet/crush/internal/config"
	crushlog "github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/telegram"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var (
	telegramToken  string
	telegramChatID int64
)

func init() {
	telegramCmd.Flags().StringVar(&telegramToken, "token", "", "Telegram bot token (or $CRUSH_TELEGRAM_BOT_TOKEN)")
	telegramCmd.Flags().Int64Var(&telegramChatID, "chat-id", 0, "Authorized Telegram chat ID (or $CRUSH_TELEGRAM_CHAT_ID)")
	rootCmd.AddCommand(telegramCmd)
}

var telegramCmd = &cobra.Command{
	Use:   "telegram",
	Short: "Drive Crush from a Telegram chat",
	Long: `Start a Telegram bridge that lets one authorized chat send prompts,
approve permissions, and manage sessions against a Crush server.

Requires a bot token from @BotFather and your numeric chat ID.
See docs/notes/telegram-bridge.md for setup.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		token := telegramToken
		if token == "" {
			token = os.Getenv("CRUSH_TELEGRAM_BOT_TOKEN")
		}
		if token == "" {
			return fmt.Errorf("telegram bot token required: pass --token or set CRUSH_TELEGRAM_BOT_TOKEN")
		}

		chatID := telegramChatID
		if chatID == 0 {
			if env := os.Getenv("CRUSH_TELEGRAM_CHAT_ID"); env != "" {
				n, err := strconv.ParseInt(env, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid CRUSH_TELEGRAM_CHAT_ID: %w", err)
				}
				chatID = n
			}
		}
		if chatID == 0 {
			return fmt.Errorf("telegram chat id required: pass --chat-id or set CRUSH_TELEGRAM_CHAT_ID (message @userinfobot, or start the bridge with a wrong id and read the logged rejected chat id)")
		}

		debug, err := cmd.Flags().GetBool("debug")
		if err != nil {
			return fmt.Errorf("failed to get debug flag: %w", err)
		}

		logFile := filepath.Join(config.GlobalCacheDir(), "telegram", "crush.log")
		if term.IsTerminal(os.Stderr.Fd()) {
			crushlog.Setup(logFile, debug, os.Stderr)
		} else {
			crushlog.Setup(logFile, debug)
		}

		c, ws, cleanup, err := connectToServer(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		if !ws.Config.IsConfigured() {
			return fmt.Errorf("crush is not configured; run 'crush' once to set up a provider")
		}

		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		return telegram.Run(ctx, telegram.Options{
			Client:    c,
			Workspace: *ws,
			Token:     token,
			ChatID:    chatID,
		})
	},
}
