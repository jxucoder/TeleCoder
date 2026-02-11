// Package telegram provides a Telegram bot integration for OpenTL.
//
// Uses long polling -- no public URL or webhook needed.
// Send a message to the bot, get a PR back with full terminal output.
package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/jxucoder/opentl/internal/session"
)

// SessionCreator is the interface used to create and run sessions.
type SessionCreator interface {
	CreateAndRunSession(repo, prompt string) (*session.Session, error)
}

// Bot is the Telegram bot for OpenTL.
type Bot struct {
	api         *tgbotapi.BotAPI
	store       *session.Store
	bus         *session.EventBus
	sessions    SessionCreator
	defaultRepo string
}

// NewBot creates a new Telegram bot.
func NewBot(token, defaultRepo string, store *session.Store, bus *session.EventBus, creator SessionCreator) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating Telegram bot: %w", err)
	}

	log.Printf("Telegram bot authorized as @%s", api.Self.UserName)

	return &Bot{
		api:         api,
		store:       store,
		bus:         bus,
		sessions:    creator,
		defaultRepo: defaultRepo,
	}, nil
}

// Run starts the long-polling loop. Blocks until ctx is canceled.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := b.api.GetUpdatesChan(u)

	log.Println("Telegram bot listening for messages...")

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if update.Message != nil {
				go b.handleMessage(update.Message)
			}
		}
	}
}

// handleMessage processes an incoming message.
func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	text := strings.TrimSpace(msg.Text)
	chatID := msg.Chat.ID
	replyTo := msg.MessageID

	// Handle /start command.
	if text == "/start" || text == "/help" {
		b.sendReply(chatID, replyTo, ""+
			"*OpenTL* \\- Send a task, get a PR\\.\n\n"+
			"*Usage:*\n"+
			"`add rate limiting to users API --repo owner/repo`\n\n"+
			"Or set `TELEGRAM_DEFAULT_REPO` and just send:\n"+
			"`fix the broken login redirect`\n\n"+
			"I'll create a session, run a coding agent, and send you the PR link with full terminal output\\.")
		return
	}

	if text == "" {
		return
	}

	// Extract --repo flag from text if present.
	prompt := text
	repo := b.defaultRepo
	if idx := strings.Index(prompt, "--repo "); idx >= 0 {
		parts := strings.Fields(prompt[idx+7:])
		if len(parts) > 0 {
			repo = parts[0]
			prompt = strings.TrimSpace(prompt[:idx])
		}
	}

	if repo == "" {
		b.sendReply(chatID, replyTo,
			"Please specify a repo:\n`add rate limiting --repo owner/repo`")
		return
	}

	if prompt == "" {
		b.sendReply(chatID, replyTo, "Please provide a task description\\.")
		return
	}

	// Acknowledge the task.
	b.sendReply(chatID, replyTo,
		fmt.Sprintf("Starting task in `%s`\\.\\.\\.\n> %s",
			escapeMarkdown(repo), escapeMarkdown(prompt)))

	// Create and start the session.
	sess, err := b.sessions.CreateAndRunSession(repo, prompt)
	if err != nil {
		b.sendReply(chatID, replyTo,
			fmt.Sprintf("Failed to start session: %s", escapeMarkdown(err.Error())))
		return
	}

	b.sendReply(chatID, replyTo,
		fmt.Sprintf("Session `%s` created\\. I'll update you as it progresses\\.", sess.ID))

	// Monitor the session in the background.
	go b.monitorSession(sess, chatID, replyTo)
}

// monitorSession subscribes to session events and sends key updates.
func (b *Bot) monitorSession(sess *session.Session, chatID int64, replyTo int) {
	ch := b.bus.Subscribe(sess.ID)
	defer b.bus.Unsubscribe(sess.ID, ch)

	for event := range ch {
		switch event.Type {
		case "status":
			b.sendReply(chatID, replyTo,
				fmt.Sprintf("⚙ %s", escapeMarkdown(event.Data)))

		case "error":
			b.sendReply(chatID, replyTo,
				fmt.Sprintf("❌ *Error:* %s", escapeMarkdown(event.Data)))

		case "done":
			// Upload the full terminal log.
			b.uploadSessionLog(chatID, replyTo, sess.ID)

			// Refresh session to get PR info.
			updated, err := b.store.GetSession(sess.ID)
			if err != nil {
				b.sendReply(chatID, replyTo,
					fmt.Sprintf("✅ Session complete\\.\n%s", escapeMarkdown(event.Data)))
				return
			}

			b.sendPRMessage(chatID, replyTo, updated)
			return
		}
	}
}

// uploadSessionLog fetches all events, formats them, and uploads as a document.
func (b *Bot) uploadSessionLog(chatID int64, replyTo int, sessionID string) {
	events, err := b.store.GetEvents(sessionID, 0)
	if err != nil {
		log.Printf("Telegram: failed to get events for session %s: %v", sessionID, err)
		return
	}

	if len(events) == 0 {
		return
	}

	// Format as a readable log.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("OpenTL Session Log: %s\n", sessionID))
	sb.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(strings.Repeat("=", 60))
	sb.WriteString("\n\n")

	for _, e := range events {
		ts := e.CreatedAt.Format("15:04:05")
		tag := strings.ToUpper(e.Type)
		sb.WriteString(fmt.Sprintf("[%s] [%s] %s\n", ts, tag, e.Data))
	}

	content := sb.String()
	filename := fmt.Sprintf("opentl-session-%s.log", sessionID)

	doc := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{
		Name:  filename,
		Bytes: []byte(content),
	})
	doc.ReplyToMessageID = replyTo
	doc.Caption = fmt.Sprintf("Terminal output for session %s", sessionID)

	if _, err := b.api.Send(doc); err != nil {
		log.Printf("Telegram: failed to upload log file: %v", err)
		// Fall back to a truncated text message.
		truncated := content
		if len(truncated) > 3500 {
			truncated = "...(truncated)...\n" + truncated[len(truncated)-3500:]
		}
		b.sendReply(chatID, replyTo,
			fmt.Sprintf("*Terminal Output \\(truncated\\):*\n```\n%s\n```", escapeMarkdown(truncated)))
	}
}

// sendPRMessage sends a formatted message with the PR link.
func (b *Bot) sendPRMessage(chatID int64, replyTo int, sess *session.Session) {
	if sess.PRUrl == "" {
		b.sendReply(chatID, replyTo, "✅ Session complete \\(no PR created\\)\\.")
		return
	}

	text := fmt.Sprintf(
		"✅ *PR Ready\\!*\n\n"+
			"[PR \\#%d: %s](%s)\n\n"+
			"Session `%s` \\| Repo `%s` \\| Branch `%s`",
		sess.PRNumber,
		escapeMarkdown(truncate(sess.Prompt, 60)),
		escapeMarkdown(sess.PRUrl),
		sess.ID,
		escapeMarkdown(sess.Repo),
		escapeMarkdown(sess.Branch),
	)

	b.sendReply(chatID, replyTo, text)
}

// sendReply sends a MarkdownV2 message as a reply.
func (b *Bot) sendReply(chatID int64, replyTo int, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyTo
	msg.ParseMode = "MarkdownV2"

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Telegram: failed to send message: %v", err)
		// Retry without markdown in case of parse errors.
		msg.ParseMode = ""
		msg.Text = stripMarkdown(text)
		b.api.Send(msg)
	}
}

// escapeMarkdown escapes special characters for Telegram MarkdownV2.
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(s)
}

// stripMarkdown removes MarkdownV2 escape sequences for plain text fallback.
func stripMarkdown(s string) string {
	r := strings.NewReplacer(
		"\\*", "*",
		"\\_", "_",
		"\\[", "[",
		"\\]", "]",
		"\\(", "(",
		"\\)", ")",
		"\\~", "~",
		"\\`", "`",
		"\\>", ">",
		"\\#", "#",
		"\\+", "+",
		"\\-", "-",
		"\\=", "=",
		"\\|", "|",
		"\\{", "{",
		"\\}", "}",
		"\\.", ".",
		"\\!", "!",
	)
	return r.Replace(s)
}

// truncate shortens a string to maxLen.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
