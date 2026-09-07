package discord

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestRegistrarUsuarioRequiresAStaffRole(t *testing.T) {
	member := &discordgo.Member{Roles: []string{"staff-role"}}
	if !hasStaffRole(member, "staff-role") {
		t.Fatal("configured staff role was not accepted")
	}
	if hasStaffRole(member, "other-role") {
		t.Fatal("unconfigured role was accepted")
	}
	if hasStaffRole(member, "") {
		t.Fatal("empty configured role was accepted")
	}
}

func TestCommandRegistryDefinitionsAndDispatch(t *testing.T) {
	if len(commands) != 3 {
		t.Fatalf("registered commands = %d, want 3", len(commands))
	}
	for _, name := range []string{"ping", "cumpleanos", "ayuda"} {
		if command := findCommand(name); command == nil || command.Handler == nil {
			t.Fatalf("command %q is not dispatchable", name)
		}
	}
	birthdayCommand := findCommand("cumpleanos")
	for _, option := range birthdayCommand.Definition.Options {
		if option.Name == "registrar-usuario" {
			if len(option.Options) != 2 || option.Options[0].Type != discordgo.ApplicationCommandOptionUser {
				t.Fatal("registrar-usuario has an invalid command shape")
			}
			return
		}
	}
	t.Fatal("registrar-usuario command is missing")
}

func TestHelpMessages(t *testing.T) {
	overview := helpMessage(discordgo.ApplicationCommandInteractionData{})
	for _, command := range []string{"/ping", "/cumpleanos", "/ayuda"} {
		if !strings.Contains(overview, command) {
			t.Fatalf("overview does not contain %q: %s", command, overview)
		}
	}

	birthdayHelp := helpMessage(discordgo.ApplicationCommandInteractionData{Options: stringOptions("comando", "cumpleanos")})
	if !strings.Contains(birthdayHelp, "/cumpleanos registrar") || !strings.Contains(birthdayHelp, "configurar-mensaje") {
		t.Fatalf("birthday help is incomplete: %s", birthdayHelp)
	}

	subcommandHelp := helpMessage(discordgo.ApplicationCommandInteractionData{Options: append(stringOptions("comando", "cumpleanos"), stringOptions("subcomando", "ver")...)})
	if !strings.Contains(subcommandHelp, "`/cumpleanos ver`") || !strings.Contains(subcommandHelp, "Permisos:") {
		t.Fatalf("subcommand help is incomplete: %s", subcommandHelp)
	}

	invalid := helpMessage(discordgo.ApplicationCommandInteractionData{Options: stringOptions("comando", "cumpleanos")})
	if !strings.Contains(invalid, "Subcomandos:") {
		t.Fatalf("root command help should list valid subcommands: %s", invalid)
	}
	invalid = helpMessage(discordgo.ApplicationCommandInteractionData{Options: append(stringOptions("comando", "cumpleanos"), stringOptions("subcomando", "invalido")...)})
	if !strings.Contains(invalid, "No existe el subcomando") || !strings.Contains(invalid, "Subcomandos:") {
		t.Fatalf("invalid subcommand feedback is incomplete: %s", invalid)
	}
	invalid = helpMessage(discordgo.ApplicationCommandInteractionData{Options: stringOptions("comando", "invalido")})
	if !strings.Contains(invalid, "No existe el comando") || !strings.Contains(invalid, "/ayuda") {
		t.Fatalf("invalid command feedback is incomplete: %s", invalid)
	}
}

func stringOptions(name, value string) []*discordgo.ApplicationCommandInteractionDataOption {
	return []*discordgo.ApplicationCommandInteractionDataOption{{
		Name: name, Type: discordgo.ApplicationCommandOptionString, Value: value,
	}}
}
