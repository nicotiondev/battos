package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

type ShellConfig struct {
	APIURL string
	Token  string
	In     io.Reader
	Out    io.Writer
}

type shellOption struct {
	Key         string
	Description string
	Args        []string
	NeedsInput  string
}

var (
	stylePrompt  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10B981"))
	styleCommand = lipgloss.NewStyle().Foreground(lipgloss.Color("#A7F3D0"))
)

func NewShellCmd(config func() ShellConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "Modo interactivo de BattOS con comandos slash",
		Long: `Abre una sesion interactiva estilo Mission Control.

Puedes escribir comandos slash como /status, /projects, /tasks <project>,
o usar comandos normales como dentro de la terminal: status, project list, etc.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunShell(cmd.Context(), config())
		},
	}
}

func RunShell(ctx context.Context, cfg ShellConfig) error {
	in := cfg.In
	if in == nil {
		in = os.Stdin
	}
	out := cfg.Out
	if out == nil {
		out = os.Stdout
	}

	PrintBanner("INTERACTIVE SHELL")
	fmt.Fprintln(out, styleSubtle.Render("Escribe / para ver acciones, /help para ayuda, /exit para salir."))
	fmt.Fprintln(out)

	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, stylePrompt.Render("battos > "))
		if !scanner.Scan() {
			fmt.Fprintln(out)
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" || line == "exit" || line == "quit" {
			fmt.Fprintln(out, styleSubtle.Render("Cerrando BattOS shell."))
			return nil
		}
		if line == "/" || line == "/menu" {
			if err := runShellPalette(ctx, cfg, scanner, out); err != nil {
				fmt.Fprintln(out, styleDown.Render("error:"), err)
			}
			continue
		}
		args, err := shellArgs(line)
		if err != nil {
			fmt.Fprintln(out, styleDown.Render("error:"), err)
			continue
		}
		if len(args) == 0 {
			continue
		}
		if err := runBattOSCommand(ctx, cfg, args, out); err != nil {
			fmt.Fprintln(out, styleDown.Render("error:"), err)
		}
	}
}

func runShellPalette(ctx context.Context, cfg ShellConfig, scanner *bufio.Scanner, out io.Writer) error {
	options := shellOptions()
	fmt.Fprintln(out, styleHeader.Render("Acciones disponibles"))
	for i, option := range options {
		fmt.Fprintf(out, "  %d. %-12s %s\n", i+1, styleCommand.Render(option.Key), styleSubtle.Render(option.Description))
	}
	fmt.Fprintln(out)
	fmt.Fprint(out, stylePrompt.Render("elige > "))
	if !scanner.Scan() {
		return scanner.Err()
	}
	choice := strings.TrimSpace(scanner.Text())
	if choice == "" {
		return nil
	}
	for i, option := range options {
		if choice == fmt.Sprintf("%d", i+1) || choice == option.Key || choice == strings.TrimPrefix(option.Key, "/") {
			args := append([]string(nil), option.Args...)
			if option.NeedsInput != "" {
				fmt.Fprint(out, stylePrompt.Render(option.NeedsInput+" > "))
				if !scanner.Scan() {
					return scanner.Err()
				}
				value := strings.TrimSpace(scanner.Text())
				if value == "" {
					return nil
				}
				args = append(args, value)
			}
			return runBattOSCommand(ctx, cfg, args, out)
		}
	}
	args, err := shellArgs(choice)
	if err != nil {
		return err
	}
	return runBattOSCommand(ctx, cfg, args, out)
}

func shellOptions() []shellOption {
	return []shellOption{
		{Key: "/status", Description: "Estado general del OS", Args: []string{"status"}},
		{Key: "/domains", Description: "Listar dominios", Args: []string{"domain", "list"}},
		{Key: "/projects", Description: "Listar proyectos", Args: []string{"project", "list"}},
		{Key: "/goals", Description: "Listar objetivos por proyecto", Args: []string{"goal", "list", "--project"}, NeedsInput: "project id"},
		{Key: "/tasks", Description: "Listar tareas por proyecto", Args: []string{"task", "list", "--project"}, NeedsInput: "project id"},
		{Key: "/memory", Description: "Ver estadisticas de memoria", Args: []string{"memory", "stats"}},
		{Key: "/help", Description: "Ayuda del CLI", Args: []string{"--help"}},
	}
}

func shellArgs(line string) ([]string, error) {
	if strings.HasPrefix(line, "/") {
		fields := strings.Fields(strings.TrimPrefix(line, "/"))
		if len(fields) == 0 {
			return nil, nil
		}
		switch fields[0] {
		case "help":
			return []string{"--help"}, nil
		case "status":
			return []string{"status"}, nil
		case "domains":
			return []string{"domain", "list"}, nil
		case "projects":
			return []string{"project", "list"}, nil
		case "memory":
			return append([]string{"memory"}, defaultSubcommand(fields[1:], "stats")...), nil
		case "goals":
			if len(fields) < 2 {
				return nil, fmt.Errorf("uso: /goals <project_id>")
			}
			return []string{"goal", "list", "--project", fields[1]}, nil
		case "tasks":
			if len(fields) < 2 {
				return nil, fmt.Errorf("uso: /tasks <project_id>")
			}
			return []string{"task", "list", "--project", fields[1]}, nil
		default:
			return fields, nil
		}
	}
	return strings.Fields(line), nil
}

func defaultSubcommand(args []string, fallback string) []string {
	if len(args) == 0 {
		return []string{fallback}
	}
	return args
}

func runBattOSCommand(ctx context.Context, cfg ShellConfig, args []string, out io.Writer) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolviendo ejecutable: %w", err)
	}
	fullArgs := []string{}
	if cfg.APIURL != "" {
		fullArgs = append(fullArgs, "--api", cfg.APIURL)
	}
	if cfg.Token != "" {
		fullArgs = append(fullArgs, "--token", cfg.Token)
	}
	fullArgs = append(fullArgs, args...)

	fmt.Fprintln(out, styleSubtle.Render("$ battos "+strings.Join(args, " ")))
	cmd := exec.CommandContext(ctx, exe, fullArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}
