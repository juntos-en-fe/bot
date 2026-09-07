package birthday

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// NotificationUserPlaceholder is replaced inline with birthday-member mentions.
	NotificationUserPlaceholder   = "{usuarios}"
	maxNotificationTemplateLength = 1_500
	maxDiscordMessageLength       = 2_000
)

// MemberLookup reports whether a user is still a member of a server.
type MemberLookup func(guildID, userID string) (found bool, err error)

// Notification is one Discord message and the only user IDs it may mention.
type Notification struct {
	Content        string
	MentionUserIDs []string
}

// MessageSender sends birthday notifications to a channel.
type MessageSender func(channelID string, notifications []Notification) error

// Scheduler delivers birthday messages at 09:00 in each server's configured timezone.
type Scheduler struct {
	store        *Store
	memberExists MemberLookup
	send         MessageSender
	logger       *slog.Logger
	now          func() time.Time
}

func NewScheduler(store *Store, memberExists MemberLookup, send MessageSender, logger *slog.Logger) *Scheduler {
	return &Scheduler{store: store, memberExists: memberExists, send: send, logger: logger, now: time.Now}
}

// Run checks immediately, then once per minute until the context is canceled.
func (s *Scheduler) Run(ctx context.Context) {
	s.deliver(ctx, s.now())
	for {
		now := s.now()
		nextMinute := now.Truncate(time.Minute).Add(time.Minute)
		timer := time.NewTimer(time.Until(nextMinute))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.deliver(ctx, s.now())
		}
	}
}

func (s *Scheduler) deliver(ctx context.Context, now time.Time) {
	settings, err := s.store.NotificationSettings(ctx)
	if err != nil {
		s.logger.Error("list birthday notification settings", "error", err)
		return
	}

	for _, setting := range settings {
		location, err := time.LoadLocation(setting.Timezone)
		if err != nil {
			s.logger.Error("invalid stored birthday timezone", "guild_id", setting.GuildID, "timezone", setting.Timezone, "error", err)
			continue
		}
		localNow := now.In(location)
		if localNow.Hour() != 9 || localNow.Minute() != 0 {
			continue
		}

		userIDs, err := s.store.UserIDsOn(ctx, setting.GuildID, Date{Month: localNow.Month(), Day: localNow.Day()})
		if err != nil {
			s.logger.Error("list today's birthdays", "guild_id", setting.GuildID, "error", err)
			continue
		}
		if len(userIDs) == 0 {
			continue
		}

		claimed, err := s.store.MarkNotified(ctx, setting.GuildID, localNow.Format("2006-01-02"))
		if err != nil {
			s.logger.Error("mark birthday notification", "guild_id", setting.GuildID, "error", err)
			continue
		}
		if !claimed {
			continue
		}

		activeUserIDs := s.activeMembers(setting.GuildID, userIDs)
		if len(activeUserIDs) == 0 {
			continue
		}
		if err := s.send(setting.NotificationChannelID, birthdayNotifications(setting.NotificationText, activeUserIDs)); err != nil {
			s.logger.Error("send birthday notification", "guild_id", setting.GuildID, "channel_id", setting.NotificationChannelID, "error", err)
		}
	}
}

func (s *Scheduler) activeMembers(guildID string, userIDs []string) []string {
	active := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		found, err := s.memberExists(guildID, userID)
		if err != nil {
			s.logger.Error("look up birthday member", "guild_id", guildID, "user_id", userID, "error", err)
			continue
		}
		if found {
			active = append(active, userID)
		}
	}
	return active
}

func birthdayNotifications(text string, userIDs []string) []Notification {
	text = strings.TrimSpace(text)
	if len(userIDs) == 0 {
		return nil
	}
	if text == "" {
		return notificationChunks("", "", userIDs)
	}
	if strings.Count(text, NotificationUserPlaceholder) != 1 {
		// Settings saved before templates were introduced remain safe: do not send
		// arbitrary legacy text, but still deliver the birthday mentions.
		return notificationChunks("", "", userIDs)
	}
	prefix, suffix, _ := strings.Cut(text, NotificationUserPlaceholder)
	return notificationChunks(prefix, suffix, userIDs)
}

// ValidateNotificationTemplate accepts a blank template (mentions only) or a
// template containing exactly one placeholder for birthday-member mentions.
func ValidateNotificationTemplate(text string) error {
	if utf8.RuneCountInString(text) > maxNotificationTemplateLength {
		return fmt.Errorf("template exceeds %d characters", maxNotificationTemplateLength)
	}
	if text != "" && strings.Count(text, NotificationUserPlaceholder) != 1 {
		return fmt.Errorf("template must contain exactly one %s", NotificationUserPlaceholder)
	}
	return nil
}

func notificationChunks(prefix, suffix string, userIDs []string) []Notification {
	var notifications []Notification
	current := Notification{}
	for _, userID := range userIDs {
		mention := "<@" + userID + ">"
		separator := ""
		if len(current.MentionUserIDs) > 0 {
			separator = ", "
		}
		content := prefix + current.Content + separator + mention + suffix
		if utf8.RuneCountInString(content) > maxDiscordMessageLength && len(current.MentionUserIDs) > 0 {
			current.Content = prefix + current.Content + suffix
			notifications = append(notifications, current)
			current = Notification{Content: mention, MentionUserIDs: []string{userID}}
			continue
		}
		current.Content += separator + mention
		current.MentionUserIDs = append(current.MentionUserIDs, userID)
	}
	if len(current.MentionUserIDs) > 0 {
		current.Content = prefix + current.Content + suffix
		notifications = append(notifications, current)
	}
	return notifications
}
