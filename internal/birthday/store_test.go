package birthday

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jef/bot/internal/database"
)

func TestStoreBirthdayLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewStore(db)
	date, _ := ParseDate("29/02")
	created, err := store.Register(ctx, "guild", "user", date)
	if err != nil || !created {
		t.Fatalf("Register() = (%t, %v), want (true, nil)", created, err)
	}
	created, err = store.Register(ctx, "guild", "user", date)
	if err != nil || created {
		t.Fatalf("second Register() = (%t, %v), want (false, nil)", created, err)
	}

	updatedDate, _ := ParseDate("01/03")
	updated, err := store.Update(ctx, "guild", "user", updatedDate)
	if err != nil || !updated {
		t.Fatalf("Update() = (%t, %v), want (true, nil)", updated, err)
	}
	userIDs, err := store.UserIDsOn(ctx, "guild", updatedDate)
	if err != nil || len(userIDs) != 1 || userIDs[0] != "user" {
		t.Fatalf("UserIDsOn() = (%v, %v), want ([user], nil)", userIDs, err)
	}

	deleted, err := store.Delete(ctx, "guild", "user")
	if err != nil || !deleted {
		t.Fatalf("Delete() = (%t, %v), want (true, nil)", deleted, err)
	}
}

func TestNotificationTemplateValidationDoesNotOverwriteSavedText(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewStore(db)
	valid := "🎂 ¡Feliz cumpleaños, {usuarios}!"
	if err := store.SetNotificationText(ctx, "guild", valid); err != nil {
		t.Fatalf("set valid template: %v", err)
	}
	for _, invalid := range []string{"¡Feliz cumpleaños!", "{usuarios} y otra vez {usuarios}"} {
		if err := store.SetNotificationText(ctx, "guild", invalid); err == nil {
			t.Fatalf("SetNotificationText(%q) succeeded", invalid)
		}
		settings, err := store.Settings(ctx, "guild")
		if err != nil {
			t.Fatalf("Settings(): %v", err)
		}
		if settings.NotificationText != valid {
			t.Fatalf("saved text = %q, want %q", settings.NotificationText, valid)
		}
	}
}
