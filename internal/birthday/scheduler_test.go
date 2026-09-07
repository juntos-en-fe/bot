package birthday

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jef/bot/internal/database"
)

func TestSchedulerDeliversOnceAtLocalNine(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewStore(db)
	if err := store.SetNotificationChannel(ctx, "guild", "channel"); err != nil {
		t.Fatalf("set notification channel: %v", err)
	}
	date, _ := ParseDate("07/09")
	if _, err := store.Register(ctx, "guild", "user", date); err != nil {
		t.Fatalf("register birthday: %v", err)
	}

	var messages []string
	scheduler := NewScheduler(
		store,
		func(_, _ string) (bool, error) { return true, nil },
		func(_ string, notifications []Notification) error {
			for _, notification := range notifications {
				messages = append(messages, notification.Content)
			}
			return nil
		},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	now := time.Date(2026, time.September, 7, 9, 0, 0, 0, time.UTC)
	scheduler.deliver(ctx, now)
	scheduler.deliver(ctx, now)

	if len(messages) != 1 {
		t.Fatalf("sent %d messages, want 1", len(messages))
	}
	if messages[0] != "<@user>" {
		t.Fatalf("message = %q", messages[0])
	}
}

func TestBirthdayNotificationsMentionEveryoneInOrder(t *testing.T) {
	notifications := birthdayNotifications("¡Feliz cumpleaños, {usuarios}!", []string{"first", "second", "third"})
	if len(notifications) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifications))
	}
	if notifications[0].Content != "¡Feliz cumpleaños, <@first>, <@second>, <@third>!" {
		t.Fatalf("content = %q", notifications[0].Content)
	}
	if got := notifications[0].MentionUserIDs; len(got) != 3 || got[0] != "first" || got[2] != "third" {
		t.Fatalf("mention IDs = %v", got)
	}
}

func TestBirthdayNotificationsDefaultTemplateAndTenUsers(t *testing.T) {
	userIDs := []string{
		"10000000000000000001", "10000000000000000002", "10000000000000000003", "10000000000000000004", "10000000000000000005",
		"10000000000000000006", "10000000000000000007", "10000000000000000008", "10000000000000000009", "10000000000000000010",
	}
	notifications := birthdayNotifications("", userIDs)
	if len(notifications) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifications))
	}
	want := "<@10000000000000000001>, <@10000000000000000002>, <@10000000000000000003>, <@10000000000000000004>, <@10000000000000000005>, <@10000000000000000006>, <@10000000000000000007>, <@10000000000000000008>, <@10000000000000000009>, <@10000000000000000010>"
	if notifications[0].Content != want {
		t.Fatalf("content = %q, want %q", notifications[0].Content, want)
	}
	if got := notifications[0].MentionUserIDs; len(got) != len(userIDs) || got[9] != userIDs[9] {
		t.Fatalf("mention IDs = %v", got)
	}
}

func TestMaximumTemplateLeavesRoomForTenMentions(t *testing.T) {
	template := strings.Repeat("a", 1_490) + NotificationUserPlaceholder
	if err := ValidateNotificationTemplate(template); err != nil {
		t.Fatalf("ValidateNotificationTemplate(): %v", err)
	}
	userIDs := []string{
		"10000000000000000001", "10000000000000000002", "10000000000000000003", "10000000000000000004", "10000000000000000005",
		"10000000000000000006", "10000000000000000007", "10000000000000000008", "10000000000000000009", "10000000000000000010",
	}
	notifications := birthdayNotifications(template, userIDs)
	if len(notifications) != 1 || len(notifications[0].Content) > 2_000 {
		t.Fatalf("notifications = %#v, want one message within Discord's limit", notifications)
	}
}
