package discord

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/bwmarrin/discordgo"
	"github.com/jef/bot/internal/birthday"
	"github.com/jef/bot/internal/config"
)

// Bot owns the Discord gateway session and command handlers.
type Bot struct {
	applicationID string
	guildID       string
	logger        *slog.Logger
	session       *discordgo.Session
	birthdays     *birthday.Store
	scheduler     *birthday.Scheduler
	stopScheduler context.CancelFunc
}

// New configures a Discord session but does not connect it.
func New(cfg config.Config, logger *slog.Logger, db *sql.DB) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	session.Identify.Intents = discordgo.IntentsGuilds

	store := birthday.NewStore(db)
	bot := &Bot{applicationID: cfg.DiscordApplicationID, guildID: cfg.DiscordGuildID, logger: logger, session: session, birthdays: store}
	bot.scheduler = birthday.NewScheduler(store, bot.memberExists, bot.sendBirthdayMessage, logger)
	session.AddHandler(bot.onReady)
	session.AddHandler(bot.onInteractionCreate)
	return bot, nil
}

// Open connects to Discord, registers commands, and starts birthday notifications.
func (b *Bot) Open() error {
	if err := b.session.Open(); err != nil {
		return err
	}
	if _, err := b.session.ApplicationCommandBulkOverwrite(b.applicationID, b.guildID, commands); err != nil {
		b.session.Close()
		return fmt.Errorf("register application commands: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.stopScheduler = cancel
	go b.scheduler.Run(ctx)
	b.logger.Info("commands registered", "scope", commandScope(b.guildID), "count", len(commands))
	return nil
}

// Close stops background work and disconnects from Discord.
func (b *Bot) Close() error {
	if b.stopScheduler != nil {
		b.stopScheduler()
	}
	return b.session.Close()
}

func (b *Bot) onReady(_ *discordgo.Session, ready *discordgo.Ready) {
	b.logger.Info("connected to Discord", "user", ready.User.String())
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if interaction.Type != discordgo.InteractionApplicationCommand {
		return
	}
	command := findCommand(interaction.ApplicationCommandData().Name)
	if command == nil {
		return
	}
	command.Handler(b, s, interaction)
}

func (b *Bot) handleBirthdayInteraction(s *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if interaction.GuildID == "" || interaction.Member == nil || interaction.Member.User == nil {
		if err := respond(s, interaction.Interaction, "Este comando solo puede usarse dentro de un servidor.", true); err != nil {
			b.logger.Error("respond to direct message birthday command", "error", err)
		}
		return
	}
	if err := deferEphemeral(s, interaction.Interaction); err != nil {
		b.logger.Error("defer birthday interaction", "error", err)
		return
	}
	message := b.handleBirthdayCommand(s, interaction)
	if err := editResponse(s, interaction.Interaction, message); err != nil {
		b.logger.Error("edit birthday interaction response", "error", err)
	}
}

func (b *Bot) handleBirthdayCommand(s *discordgo.Session, interaction *discordgo.InteractionCreate) string {
	data := interaction.ApplicationCommandData()
	if len(data.Options) != 1 {
		return "No se pudo determinar el subcomando."
	}
	subcommand := data.Options[0]
	command := findCommand("cumpleanos")
	descriptor := findSubcommand(command, subcommand.Name)
	if descriptor == nil {
		return "No se pudo determinar el subcomando."
	}
	return descriptor.Handler(b, s, interaction, subcommand)
}

func (b *Bot) registerBirthday(_ *discordgo.Session, interaction *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) string {
	date, err := birthday.ParseDate(stringOption(option, "fecha"))
	if err != nil {
		return "No se pudo registrar tu cumpleaños: " + err.Error() + "."
	}
	created, err := b.birthdays.Register(context.Background(), interaction.GuildID, interaction.Member.User.ID, date)
	if err != nil {
		b.logger.Error("register birthday", "error", err)
		return "No se pudo guardar tu cumpleaños. Inténtalo de nuevo."
	}
	if !created {
		return "Ya tienes un cumpleaños registrado. Usa `/cumpleanos editar` para cambiarlo."
	}
	return fmt.Sprintf("Tu cumpleaños quedó registrado para el %s.", date)
}

func (b *Bot) editBirthday(_ *discordgo.Session, interaction *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) string {
	date, err := birthday.ParseDate(stringOption(option, "fecha"))
	if err != nil {
		return "No se pudo editar tu cumpleaños: " + err.Error() + "."
	}
	updated, err := b.birthdays.Update(context.Background(), interaction.GuildID, interaction.Member.User.ID, date)
	if err != nil {
		b.logger.Error("update birthday", "error", err)
		return "No se pudo actualizar tu cumpleaños. Inténtalo de nuevo."
	}
	if !updated {
		return "No tienes un cumpleaños registrado. Usa `/cumpleanos registrar` primero."
	}
	return fmt.Sprintf("Tu cumpleaños se actualizó al %s.", date)
}

func (b *Bot) viewBirthday(_ *discordgo.Session, interaction *discordgo.InteractionCreate, _ *discordgo.ApplicationCommandInteractionDataOption) string {
	date, found, err := b.birthdays.Birthday(context.Background(), interaction.GuildID, interaction.Member.User.ID)
	if err != nil {
		b.logger.Error("view own birthday", "error", err)
		return "No se pudo consultar tu cumpleaños. Inténtalo de nuevo."
	}
	if !found {
		return "No tienes un cumpleaños registrado. Usa `/cumpleanos registrar` primero."
	}
	return fmt.Sprintf("Tu cumpleaños está registrado para el %s.", date)
}

func (b *Bot) deleteBirthday(_ *discordgo.Session, interaction *discordgo.InteractionCreate, _ *discordgo.ApplicationCommandInteractionDataOption) string {
	deleted, err := b.birthdays.Delete(context.Background(), interaction.GuildID, interaction.Member.User.ID)
	if err != nil {
		b.logger.Error("delete own birthday", "error", err)
		return "No se pudo eliminar tu cumpleaños. Inténtalo de nuevo."
	}
	if !deleted {
		return "No tenías un cumpleaños registrado."
	}
	return "Tu cumpleaños fue eliminado."
}

func (b *Bot) registerMemberBirthday(_ *discordgo.Session, interaction *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) string {
	if message := b.requireConfiguredStaff(interaction); message != "" {
		return message
	}
	targetUserID := idOption(option, "usuario")
	if targetUserID == "" {
		return "Elige un miembro válido del servidor."
	}
	date, err := birthday.ParseDate(stringOption(option, "fecha"))
	if err != nil {
		return "No se pudo registrar el cumpleaños: " + err.Error() + "."
	}
	created, err := b.birthdays.Register(context.Background(), interaction.GuildID, targetUserID, date)
	if err != nil {
		b.logger.Error("register birthday for member", "error", err)
		return "No se pudo guardar el cumpleaños. Inténtalo de nuevo."
	}
	if !created {
		return "Ese usuario ya tiene un cumpleaños registrado."
	}
	return fmt.Sprintf("El cumpleaños del usuario quedó registrado para el %s.", date)
}

func (b *Bot) editMemberBirthday(_ *discordgo.Session, interaction *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) string {
	if message := b.requireConfiguredStaff(interaction); message != "" {
		return message
	}
	targetUserID := idOption(option, "usuario")
	if targetUserID == "" {
		return "Elige un miembro válido del servidor."
	}
	date, err := birthday.ParseDate(stringOption(option, "fecha"))
	if err != nil {
		return "No se pudo editar el cumpleaños: " + err.Error() + "."
	}
	updated, err := b.birthdays.Update(context.Background(), interaction.GuildID, targetUserID, date)
	if err != nil {
		b.logger.Error("update birthday for member", "error", err)
		return "No se pudo actualizar el cumpleaños. Inténtalo de nuevo."
	}
	if !updated {
		return "Ese usuario no tiene un cumpleaños registrado."
	}
	return fmt.Sprintf("El cumpleaños del usuario se actualizó al %s.", date)
}

func (b *Bot) setBirthdayStaffRole(s *discordgo.Session, interaction *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) string {
	owner, err := b.isGuildOwner(s, interaction.GuildID, interaction.Member.User.ID)
	if err != nil {
		b.logger.Error("check guild owner", "error", err)
		return "No se pudo comprobar el propietario del servidor. Inténtalo de nuevo."
	}
	if !owner {
		return "Solo el propietario del servidor puede configurar el rol de staff."
	}
	roleID := idOption(option, "rol")
	if roleID == "" || roleID == interaction.GuildID {
		return "Elige un rol de staff válido distinto de `@everyone`."
	}
	if err := b.birthdays.SetStaffRole(context.Background(), interaction.GuildID, roleID); err != nil {
		b.logger.Error("set birthday staff role", "error", err)
		return "No se pudo guardar el rol de staff. Inténtalo de nuevo."
	}
	return "El rol de staff para cumpleaños fue configurado."
}

func (b *Bot) setBirthdayNotificationChannel(s *discordgo.Session, interaction *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) string {
	if message := b.requireBirthdayAdmin(s, interaction); message != "" {
		return message
	}
	channelID := idOption(option, "canal")
	if channelID == "" {
		return "Elige un canal de texto válido."
	}
	if err := b.birthdays.SetNotificationChannel(context.Background(), interaction.GuildID, channelID); err != nil {
		b.logger.Error("set birthday notification channel", "error", err)
		return "No se pudo guardar el canal de avisos. Inténtalo de nuevo."
	}
	return "El canal de avisos de cumpleaños fue configurado."
}

func (b *Bot) setBirthdayNotificationText(s *discordgo.Session, interaction *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) string {
	if message := b.requireBirthdayAdmin(s, interaction); message != "" {
		return message
	}
	message := strings.TrimSpace(stringOption(option, "mensaje"))
	if err := birthday.ValidateNotificationTemplate(message); err != nil {
		return "El mensaje debe incluir exactamente una vez `{usuarios}`. Ejemplo: `🎂 ¡Feliz cumpleaños, {usuarios}!`"
	}
	if err := b.birthdays.SetNotificationText(context.Background(), interaction.GuildID, message); err != nil {
		b.logger.Error("set birthday notification text", "error", err)
		return "No se pudo guardar el mensaje de aviso. Inténtalo de nuevo."
	}
	if message == "" {
		return "El mensaje personalizado fue eliminado; el aviso solo mencionará a los cumpleañeros."
	}
	return "El mensaje de aviso de cumpleaños fue configurado."
}

func (b *Bot) setBirthdayTimezone(s *discordgo.Session, interaction *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) string {
	if message := b.requireBirthdayAdmin(s, interaction); message != "" {
		return message
	}
	zone := strings.TrimSpace(stringOption(option, "zona"))
	if _, err := time.LoadLocation(zone); err != nil {
		return "La zona horaria no es válida. Usa un nombre IANA, por ejemplo `America/Argentina/Buenos_Aires`."
	}
	if err := b.birthdays.SetTimezone(context.Background(), interaction.GuildID, zone); err != nil {
		b.logger.Error("set birthday timezone", "error", err)
		return "No se pudo guardar la zona horaria. Inténtalo de nuevo."
	}
	return fmt.Sprintf("La zona horaria de cumpleaños quedó configurada como `%s`.", zone)
}

func (b *Bot) deleteBirthdayByID(s *discordgo.Session, interaction *discordgo.InteractionCreate, option *discordgo.ApplicationCommandInteractionDataOption) string {
	if message := b.requireBirthdayAdmin(s, interaction); message != "" {
		return message
	}
	absentUserID := strings.TrimSpace(stringOption(option, "usuario_id"))
	if !validDiscordID(absentUserID) {
		return "El ID de usuario no es válido. Copia el ID numérico de Discord."
	}
	_, found, err := b.memberName(interaction.GuildID, absentUserID)
	if err != nil {
		b.logger.Error("check member before deleting birthday by user id", "error", err)
		return "No se pudo comprobar si el usuario sigue en el servidor. Inténtalo de nuevo."
	}
	if found {
		return "Ese usuario sigue en el servidor y debe administrar su propio cumpleaños."
	}
	deleted, err := b.birthdays.Delete(context.Background(), interaction.GuildID, absentUserID)
	if err != nil {
		b.logger.Error("delete birthday by user id", "error", err)
		return "No se pudo eliminar el registro. Inténtalo de nuevo."
	}
	if !deleted {
		return "No hay un cumpleaños registrado para ese usuario."
	}
	return "El registro de cumpleaños fue eliminado."
}

func (b *Bot) cleanBirthdayDepartedMembers(s *discordgo.Session, interaction *discordgo.InteractionCreate, _ *discordgo.ApplicationCommandInteractionDataOption) string {
	if message := b.requireBirthdayAdmin(s, interaction); message != "" {
		return message
	}
	return b.cleanDepartedMembers(context.Background(), interaction.GuildID)
}

func (b *Bot) requireConfiguredStaff(interaction *discordgo.InteractionCreate) string {
	settings, err := b.birthdays.Settings(context.Background(), interaction.GuildID)
	if err != nil {
		b.logger.Error("get birthday settings", "error", err)
		return "No se pudo consultar la configuración de cumpleaños."
	}
	if !hasStaffRole(interaction.Member, settings.StaffRoleID) {
		return "Solo el rol de staff configurado puede administrar cumpleaños de otros usuarios."
	}
	return ""
}

func (b *Bot) requireBirthdayAdmin(s *discordgo.Session, interaction *discordgo.InteractionCreate) string {
	settings, err := b.birthdays.Settings(context.Background(), interaction.GuildID)
	if err != nil {
		b.logger.Error("get birthday settings", "error", err)
		return "No se pudo consultar la configuración de cumpleaños."
	}
	allowed, err := b.isBirthdayAdmin(s, interaction.GuildID, interaction.Member, interaction.Member.User.ID, settings)
	if err != nil {
		b.logger.Error("check birthday administration permission", "error", err)
		return "No se pudo comprobar tus permisos. Inténtalo de nuevo."
	}
	if !allowed {
		return "No tienes permisos para administrar cumpleaños."
	}
	return ""
}

func (b *Bot) cleanDepartedMembers(ctx context.Context, guildID string) string {
	userIDs, err := b.birthdays.UserIDs(ctx, guildID)
	if err != nil {
		b.logger.Error("list birthday users for cleanup", "error", err)
		return "No se pudieron consultar los registros de cumpleaños."
	}
	var departed []string
	var unavailable int
	for _, userID := range userIDs {
		_, found, err := b.memberName(guildID, userID)
		if err != nil {
			b.logger.Warn("check member for birthday cleanup", "guild_id", guildID, "user_id", userID, "error", err)
			unavailable++
			continue
		}
		if !found {
			departed = append(departed, userID)
		}
	}
	deleted, err := b.birthdays.DeleteUsers(ctx, guildID, departed)
	if err != nil {
		b.logger.Error("delete departed birthday users", "error", err)
		return "No se pudieron eliminar los registros encontrados."
	}
	message := fmt.Sprintf("Limpieza completada: se eliminaron %d registro(s).", deleted)
	if unavailable > 0 {
		message += fmt.Sprintf(" No se pudieron comprobar %d usuario(s), así que sus registros se conservaron.", unavailable)
	}
	return message
}

func (b *Bot) isBirthdayAdmin(s *discordgo.Session, guildID string, member *discordgo.Member, userID string, settings birthday.Settings) (bool, error) {
	owner, err := b.isGuildOwner(s, guildID, userID)
	if err != nil || owner {
		return owner, err
	}
	return hasStaffRole(member, settings.StaffRoleID), nil
}

func hasStaffRole(member *discordgo.Member, staffRoleID string) bool {
	if staffRoleID == "" {
		return false
	}
	for _, roleID := range member.Roles {
		if roleID == staffRoleID {
			return true
		}
	}
	return false
}

func (b *Bot) isGuildOwner(s *discordgo.Session, guildID, userID string) (bool, error) {
	guild, err := s.Guild(guildID)
	if err != nil {
		return false, err
	}
	return guild.OwnerID == userID, nil
}

func (b *Bot) memberName(guildID, userID string) (string, bool, error) {
	member, err := b.session.GuildMember(guildID, userID)
	if err != nil {
		if isNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if member.Nick != "" {
		return safeDisplayName(member.Nick), true, nil
	}
	if member.User == nil {
		return userID, true, nil
	}
	if member.User.GlobalName != "" {
		return safeDisplayName(member.User.GlobalName), true, nil
	}
	return safeDisplayName(member.User.Username), true, nil
}

func (b *Bot) memberExists(guildID, userID string) (bool, error) {
	_, found, err := b.memberName(guildID, userID)
	return found, err
}

func (b *Bot) sendBirthdayMessage(channelID string, notifications []birthday.Notification) error {
	for _, notification := range notifications {
		_, err := b.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Content: notification.Content,
			AllowedMentions: &discordgo.MessageAllowedMentions{
				Users: notification.MentionUserIDs,
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func respond(s *discordgo.Session, interaction *discordgo.Interaction, content string, ephemeral bool) error {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	return s.InteractionRespond(interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Content: content, Flags: flags, AllowedMentions: &discordgo.MessageAllowedMentions{}}})
}

func deferEphemeral(s *discordgo.Session, interaction *discordgo.Interaction) error {
	return s.InteractionRespond(interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseDeferredChannelMessageWithSource, Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral}})
}

func editResponse(s *discordgo.Session, interaction *discordgo.Interaction, content string) error {
	_, err := s.InteractionResponseEdit(interaction, &discordgo.WebhookEdit{Content: &content, AllowedMentions: &discordgo.MessageAllowedMentions{}})
	return err
}

func stringOption(option *discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, child := range option.Options {
		if child.Name == name && child.Type == discordgo.ApplicationCommandOptionString {
			if value, ok := child.Value.(string); ok {
				return value
			}
		}
	}
	return ""
}

func idOption(option *discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, child := range option.Options {
		if child.Name == name {
			if value, ok := child.Value.(string); ok {
				return value
			}
		}
	}
	return ""
}

func validDiscordID(value string) bool {
	if len(value) < 16 || len(value) > 20 {
		return false
	}
	for _, character := range value {
		if !unicode.IsDigit(character) {
			return false
		}
	}
	return true
}

func safeDisplayName(name string) string {
	return strings.NewReplacer("\n", " ", "\r", " ").Replace(name)
}

func isNotFound(err error) bool {
	var restErr *discordgo.RESTError
	return errors.As(err, &restErr) && restErr.Response != nil && restErr.Response.StatusCode == http.StatusNotFound
}

func commandScope(guildID string) string {
	if guildID == "" {
		return "global"
	}
	return "guild"
}
