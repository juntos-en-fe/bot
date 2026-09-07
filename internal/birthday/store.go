package birthday

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Settings contains birthday configuration for one Discord server.
type Settings struct {
	GuildID               string
	StaffRoleID           string
	NotificationChannelID string
	NotificationText      string
	Timezone              string
	LastNotifiedDate      string
}

// Store persists birthdays and server-specific settings.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Register(ctx context.Context, guildID, userID string, date Date) (bool, error) {
	result, err := s.db.ExecContext(ctx, `INSERT INTO birthdays (guild_id, user_id, month, day)
		VALUES (?, ?, ?, ?) ON CONFLICT(guild_id, user_id) DO NOTHING`, guildID, userID, date.Month, date.Day)
	if err != nil {
		return false, fmt.Errorf("register birthday: %w", err)
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (s *Store) Update(ctx context.Context, guildID, userID string, date Date) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE birthdays
		SET month = ?, day = ?, updated_at = CURRENT_TIMESTAMP
		WHERE guild_id = ? AND user_id = ?`, date.Month, date.Day, guildID, userID)
	if err != nil {
		return false, fmt.Errorf("update birthday: %w", err)
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (s *Store) Delete(ctx context.Context, guildID, userID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM birthdays WHERE guild_id = ? AND user_id = ?", guildID, userID)
	if err != nil {
		return false, fmt.Errorf("delete birthday: %w", err)
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (s *Store) Settings(ctx context.Context, guildID string) (Settings, error) {
	settings := Settings{GuildID: guildID, Timezone: "UTC"}
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(staff_role_id, ''),
		COALESCE(notification_channel_id, ''), COALESCE(notification_text, ''), timezone, COALESCE(last_notified_date, '')
		FROM birthday_settings WHERE guild_id = ?`, guildID).Scan(
		&settings.StaffRoleID, &settings.NotificationChannelID, &settings.NotificationText, &settings.Timezone, &settings.LastNotifiedDate,
	)
	if err == sql.ErrNoRows {
		return settings, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("get birthday settings: %w", err)
	}
	return settings, nil
}

func (s *Store) SetStaffRole(ctx context.Context, guildID, roleID string) error {
	return s.upsertSetting(ctx, guildID, "staff_role_id", roleID)
}

func (s *Store) SetNotificationChannel(ctx context.Context, guildID, channelID string) error {
	return s.upsertSetting(ctx, guildID, "notification_channel_id", channelID)
}

func (s *Store) SetNotificationText(ctx context.Context, guildID, text string) error {
	if err := ValidateNotificationTemplate(text); err != nil {
		return err
	}
	return s.upsertSetting(ctx, guildID, "notification_text", text)
}

func (s *Store) SetTimezone(ctx context.Context, guildID, timezone string) error {
	return s.upsertSetting(ctx, guildID, "timezone", timezone)
}

func (s *Store) upsertSetting(ctx context.Context, guildID, column, value string) error {
	allowedColumns := map[string]bool{
		"staff_role_id":           true,
		"notification_channel_id": true,
		"notification_text":       true,
		"timezone":                true,
	}
	if !allowedColumns[column] {
		return fmt.Errorf("unsupported birthday setting %q", column)
	}
	query := fmt.Sprintf(`INSERT INTO birthday_settings (guild_id, %s) VALUES (?, ?)
		ON CONFLICT(guild_id) DO UPDATE SET %s = excluded.%s`, column, column, column)
	if _, err := s.db.ExecContext(ctx, query, guildID, value); err != nil {
		return fmt.Errorf("set birthday setting: %w", err)
	}
	return nil
}

func (s *Store) NotificationSettings(ctx context.Context) ([]Settings, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT guild_id, COALESCE(staff_role_id, ''), notification_channel_id,
		COALESCE(notification_text, ''), timezone, COALESCE(last_notified_date, '') FROM birthday_settings
		WHERE notification_channel_id IS NOT NULL AND notification_channel_id <> ''`)
	if err != nil {
		return nil, fmt.Errorf("list birthday notification settings: %w", err)
	}
	defer rows.Close()

	var settings []Settings
	for rows.Next() {
		var item Settings
		if err := rows.Scan(&item.GuildID, &item.StaffRoleID, &item.NotificationChannelID, &item.NotificationText, &item.Timezone, &item.LastNotifiedDate); err != nil {
			return nil, fmt.Errorf("scan birthday notification settings: %w", err)
		}
		settings = append(settings, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate birthday notification settings: %w", err)
	}
	return settings, nil
}

func (s *Store) UserIDsOn(ctx context.Context, guildID string, date Date) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT user_id FROM birthdays WHERE guild_id = ? AND month = ? AND day = ? ORDER BY user_id", guildID, date.Month, date.Day)
	if err != nil {
		return nil, fmt.Errorf("list birthdays by date: %w", err)
	}
	defer rows.Close()

	return collectUserIDs(rows)
}

func (s *Store) UserIDs(ctx context.Context, guildID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT user_id FROM birthdays WHERE guild_id = ? ORDER BY user_id", guildID)
	if err != nil {
		return nil, fmt.Errorf("list birthday users: %w", err)
	}
	defer rows.Close()

	return collectUserIDs(rows)
}

func (s *Store) Birthday(ctx context.Context, guildID, userID string) (Date, bool, error) {
	var month, day int
	err := s.db.QueryRowContext(ctx, "SELECT month, day FROM birthdays WHERE guild_id = ? AND user_id = ?", guildID, userID).Scan(&month, &day)
	if err == sql.ErrNoRows {
		return Date{}, false, nil
	}
	if err != nil {
		return Date{}, false, fmt.Errorf("get birthday: %w", err)
	}
	return Date{Month: time.Month(month), Day: day}, true, nil
}

func collectUserIDs(rows *sql.Rows) ([]string, error) {
	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan birthday user: %w", err)
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate birthday users: %w", err)
	}
	return userIDs, nil
}

func (s *Store) DeleteUsers(ctx context.Context, guildID string, userIDs []string) (int64, error) {
	if len(userIDs) == 0 {
		return 0, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(userIDs)), ",")
	args := make([]any, 0, len(userIDs)+1)
	args = append(args, guildID)
	for _, userID := range userIDs {
		args = append(args, userID)
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM birthdays WHERE guild_id = ? AND user_id IN ("+placeholders+")", args...)
	if err != nil {
		return 0, fmt.Errorf("delete birthday users: %w", err)
	}
	return result.RowsAffected()
}

// MarkNotified records a delivery once per server-local calendar date.
func (s *Store) MarkNotified(ctx context.Context, guildID, localDate string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE birthday_settings SET last_notified_date = ?
		WHERE guild_id = ? AND (last_notified_date IS NULL OR last_notified_date <> ?)`, localDate, guildID, localDate)
	if err != nil {
		return false, fmt.Errorf("mark birthday notification: %w", err)
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}
