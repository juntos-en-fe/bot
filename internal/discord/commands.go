package discord

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// commandDescriptor is the single source of truth for a top-level command,
// including its Discord definition, help text, and interaction handler.
type commandDescriptor struct {
	Definition     *discordgo.ApplicationCommand
	Summary        string
	Usage          string
	PermissionNote string
	Help           string
	Handler        func(*Bot, *discordgo.Session, *discordgo.InteractionCreate)
	Subcommands    []subcommandDescriptor
}

// subcommandDescriptor describes a /cumpleanos subcommand.
type subcommandDescriptor struct {
	Definition     *discordgo.ApplicationCommandOption
	Summary        string
	Usage          string
	PermissionNote string
	Help           string
	Handler        func(*Bot, *discordgo.Session, *discordgo.InteractionCreate, *discordgo.ApplicationCommandInteractionDataOption) string
}

var (
	commandRegistry []commandDescriptor
	commands        []*discordgo.ApplicationCommand
)

func init() {
	birthdayCommands := birthdaySubcommands()
	commandRegistry = []commandDescriptor{
		{
			Definition:     &discordgo.ApplicationCommand{Name: "ping", Description: "Comprueba si el bot está respondiendo."},
			Summary:        "Comprueba si el bot está respondiendo.",
			Usage:          "/ping",
			PermissionNote: "Disponible para todos.",
			Help:           "Responde con `Pong!`.",
			Handler:        handlePingInteraction,
		},
		{
			Definition:     helpCommandDefinition(),
			Summary:        "Muestra ayuda sobre los comandos.",
			Usage:          "/ayuda [comando] [subcomando]",
			PermissionNote: "Disponible para todos. La respuesta solo la verás tú.",
			Help:           "Consulta el uso, comportamiento y permisos de cada comando.",
			Handler:        handleHelpInteraction,
		},
		{
			Definition:     birthdayCommandDefinition(birthdayCommands),
			Summary:        "Registra y administra cumpleaños.",
			Usage:          "/cumpleanos <subcomando>",
			PermissionNote: "Las acciones de administración requieren el rol de staff configurado; el propietario del servidor conserva acceso de recuperación.",
			Help:           "Guarda únicamente el día y el mes del cumpleaños para este servidor.",
			Handler:        (*Bot).handleBirthdayInteraction,
			Subcommands:    birthdayCommands,
		},
	}
	commands = commandDefinitions(commandRegistry)
}

func commandDefinitions(registry []commandDescriptor) []*discordgo.ApplicationCommand {
	definitions := make([]*discordgo.ApplicationCommand, 0, len(registry))
	for _, command := range registry {
		definitions = append(definitions, command.Definition)
	}
	return definitions
}

func helpCommandDefinition() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name: "ayuda", Description: "Muestra ayuda sobre los comandos.",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "comando", Description: "Comando sobre el que quieres ayuda."},
			{Type: discordgo.ApplicationCommandOptionString, Name: "subcomando", Description: "Subcomando sobre el que quieres ayuda."},
		},
	}
}

func birthdayCommandDefinition(subcommands []subcommandDescriptor) *discordgo.ApplicationCommand {
	disableDMs := false
	definition := &discordgo.ApplicationCommand{
		Name: "cumpleanos", Description: "Registra y administra cumpleaños.", DMPermission: &disableDMs,
	}
	for _, subcommand := range subcommands {
		definition.Options = append(definition.Options, subcommand.Definition)
	}
	return definition
}

func birthdaySubcommands() []subcommandDescriptor {
	return []subcommandDescriptor{
		{birthdayDateCommand("registrar", "Registra tu cumpleaños."), "Registra tu cumpleaños.", "/cumpleanos registrar fecha:DD/MM", "Disponible para todos los miembros.", "Guarda tu día y mes de cumpleaños en este servidor.", (*Bot).registerBirthday},
		{birthdayUserDateCommand(), "Registra el cumpleaños de otro miembro.", "/cumpleanos registrar-usuario usuario:@miembro fecha:DD/MM", "Solo el rol de staff configurado.", "Registra el día y mes de otro miembro.", (*Bot).registerMemberBirthday},
		{birthdayDateCommand("editar", "Edita tu cumpleaños."), "Edita tu cumpleaños.", "/cumpleanos editar fecha:DD/MM", "Disponible para todos los miembros.", "Cambia tu día y mes de cumpleaños ya registrado.", (*Bot).editBirthday},
		{birthdayUserDateCommand("editar-usuario", "Edita el cumpleaños de otro miembro."), "Edita el cumpleaños de otro miembro.", "/cumpleanos editar-usuario usuario:@miembro fecha:DD/MM", "Solo el rol de staff configurado.", "Corrige el día y mes registrado de otro miembro.", (*Bot).editMemberBirthday},
		{&discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "ver", Description: "Muestra tu cumpleaños registrado."}, "Muestra tu cumpleaños registrado.", "/cumpleanos ver", "Disponible para todos los miembros.", "Muestra el día y mes que tienes guardados.", (*Bot).viewBirthday},
		{&discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "eliminar", Description: "Elimina tu cumpleaños."}, "Elimina tu cumpleaños.", "/cumpleanos eliminar", "Disponible para todos los miembros.", "Elimina tu cumpleaños de este servidor.", (*Bot).deleteBirthday},
		{&discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "configurar-rol", Description: "Configura el rol de staff.", Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionRole, Name: "rol", Description: "Rol autorizado para administrar cumpleaños.", Required: true}}}, "Configura el rol de staff.", "/cumpleanos configurar-rol rol:@rol", "Solo el propietario del servidor.", "Define el rol que podrá administrar los avisos y la limpieza.", (*Bot).setBirthdayStaffRole},
		{&discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "configurar-canal", Description: "Configura el canal de avisos.", Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionChannel, Name: "canal", Description: "Canal de texto para los avisos diarios.", ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText, discordgo.ChannelTypeGuildNews}, Required: true}}}, "Configura el canal de avisos.", "/cumpleanos configurar-canal canal:#canal", "Rol de staff configurado o propietario del servidor.", "Selecciona el canal de texto donde se enviarán los avisos diarios.", (*Bot).setBirthdayNotificationChannel},
		{&discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "configurar-mensaje", Description: "Configura el texto de los avisos.", Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "mensaje", Description: "Texto con exactamente un {usuarios}.", MaxLength: 1500, Required: true}}}, "Configura el texto de los avisos.", "/cumpleanos configurar-mensaje mensaje:🎂 ¡Feliz cumpleaños, {usuarios}!", "Rol de staff configurado o propietario del servidor.", "Usa exactamente una vez `{usuarios}` para insertar las menciones. Déjalo en blanco para enviar solo las menciones.", (*Bot).setBirthdayNotificationText},
		{&discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "configurar-zona", Description: "Configura la zona horaria.", Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "zona", Description: "Zona IANA, por ejemplo America/Argentina/Buenos_Aires.", Required: true}}}, "Configura la zona horaria.", "/cumpleanos configurar-zona zona:America/Argentina/Buenos_Aires", "Rol de staff configurado o propietario del servidor.", "Configura la zona IANA usada para enviar el aviso diario a las 09:00.", (*Bot).setBirthdayTimezone},
		{&discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "limpiar-salidos", Description: "Elimina registros de usuarios que ya no están."}, "Elimina registros de usuarios que ya no están.", "/cumpleanos limpiar-salidos", "Rol de staff configurado o propietario del servidor.", "Comprueba todos los registros y elimina los de miembros que ya salieron.", (*Bot).cleanBirthdayDepartedMembers},
		{&discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "eliminar-id", Description: "Elimina el registro de un usuario por su ID.", Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "usuario_id", Description: "ID de Discord del usuario ausente.", Required: true}}}, "Elimina el registro de un usuario ausente.", "/cumpleanos eliminar-id usuario_id:ID", "Rol de staff configurado o propietario del servidor.", "Elimina el registro solo si ese ID ya no corresponde a un miembro del servidor.", (*Bot).deleteBirthdayByID},
	}
}

func birthdayDateCommand(name, description string) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionSubCommand, Name: name, Description: description, Options: []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: "fecha", Description: "Día y mes con formato DD/MM.", Required: true},
	}}
}

func birthdayUserDateCommand(names ...string) *discordgo.ApplicationCommandOption {
	name, description := "registrar-usuario", "Registra el cumpleaños de otro miembro."
	if len(names) == 2 {
		name, description = names[0], names[1]
	}
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        name,
		Description: description,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionUser, Name: "usuario", Description: "Miembro cuyo cumpleaños se registrará.", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "fecha", Description: "Día y mes con formato DD/MM.", Required: true},
		},
	}
}

func findCommand(name string) *commandDescriptor {
	for index := range commandRegistry {
		if commandRegistry[index].Definition.Name == name {
			return &commandRegistry[index]
		}
	}
	return nil
}

func findSubcommand(command *commandDescriptor, name string) *subcommandDescriptor {
	if command == nil {
		return nil
	}
	for index := range command.Subcommands {
		if command.Subcommands[index].Definition.Name == name {
			return &command.Subcommands[index]
		}
	}
	return nil
}

func handlePingInteraction(b *Bot, s *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if err := respond(s, interaction.Interaction, "Pong!", false); err != nil {
		b.logger.Error("respond to ping", "error", err)
		return
	}
}

func handleHelpInteraction(b *Bot, s *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if err := respond(s, interaction.Interaction, helpMessage(interaction.ApplicationCommandData()), true); err != nil {
		b.logger.Error("respond to help", "error", err)
		return
	}
}

func helpMessage(data discordgo.ApplicationCommandInteractionData) string {
	commandName := strings.TrimSpace(rootStringOption(data.Options, "comando"))
	subcommandName := strings.TrimSpace(rootStringOption(data.Options, "subcomando"))
	if commandName == "" {
		if subcommandName != "" {
			return "Indica primero un comando. Uso: `/ayuda comando:cumpleanos subcomando:ver`.\n\n" + helpOverview()
		}
		return helpOverview()
	}
	command := findCommand(commandName)
	if command == nil {
		return fmt.Sprintf("No existe el comando `%s`.\n\n%s", commandName, helpOverview())
	}
	if subcommandName == "" {
		return commandHelp(command)
	}
	subcommand := findSubcommand(command, subcommandName)
	if subcommand == nil {
		if len(command.Subcommands) == 0 {
			return fmt.Sprintf("`/%s` no tiene subcomandos.\n\n%s", command.Definition.Name, commandHelp(command))
		}
		return fmt.Sprintf("No existe el subcomando `%s` para `/%s`.\n\n%s", subcommandName, command.Definition.Name, commandHelp(command))
	}
	return fmt.Sprintf("`%s`\n%s\n\nUso: `%s`\nPermisos: %s", subcommand.Summary, subcommand.Help, subcommand.Usage, subcommand.PermissionNote)
}

func helpOverview() string {
	var lines []string
	for _, command := range commandRegistry {
		staff := ""
		if len(command.Subcommands) > 0 {
			staff = " Incluye acciones solo para staff."
		}
		lines = append(lines, fmt.Sprintf("`/%s` — %s%s", command.Definition.Name, command.Summary, staff))
	}
	return "Comandos disponibles:\n" + strings.Join(lines, "\n") + "\n\nUsa `/ayuda comando:cumpleanos` para ver sus subcomandos."
}

func commandHelp(command *commandDescriptor) string {
	message := fmt.Sprintf("`/%s`\n%s\n\nUso: `%s`\nPermisos: %s", command.Definition.Name, command.Help, command.Usage, command.PermissionNote)
	if len(command.Subcommands) == 0 {
		return message
	}
	lines := make([]string, 0, len(command.Subcommands))
	for _, subcommand := range command.Subcommands {
		lines = append(lines, fmt.Sprintf("`%s` — %s", subcommand.Usage, subcommand.Summary))
	}
	return message + "\n\nSubcomandos:\n" + strings.Join(lines, "\n")
}

func rootStringOption(options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, option := range options {
		if option.Name == name && option.Type == discordgo.ApplicationCommandOptionString {
			if value, ok := option.Value.(string); ok {
				return value
			}
		}
	}
	return ""
}
