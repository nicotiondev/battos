package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	xterm "github.com/charmbracelet/x/term"
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

type shellKey int

const (
	keyUnknown shellKey = iota
	keyUp
	keyDown
	keyEnter
	keySlash
	keyEscape
	keyBackspace
	keyCtrlC
	keyRune
)

type keyEvent struct {
	Key shellKey
	Ch  rune
}

type commandResultAction int

const (
	commandBack commandResultAction = iota
	commandExit
)

var (
	stylePrompt   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FACC15"))
	styleCommand  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FDE68A"))
	stylePanel    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#FACC15")).Padding(1, 2)
	styleSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0F172A")).
			Background(lipgloss.Color("#FACC15")).
			Padding(0, 1)
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
	if cfg.In == nil && cfg.Out == nil && xterm.IsTerminal(os.Stdin.Fd()) && xterm.IsTerminal(os.Stdout.Fd()) {
		return RunTUI(ctx, cfg)
	}
	return runLineShell(ctx, cfg)
}

func runLineShell(ctx context.Context, cfg ShellConfig) error {
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

func RunTUI(ctx context.Context, cfg ShellConfig) error {
	out := os.Stdout
	state, err := xterm.MakeRaw(os.Stdin.Fd())
	if err != nil {
		return runLineShell(ctx, cfg)
	}
	defer xterm.Restore(os.Stdin.Fd(), state)
	fmt.Fprint(out, "\x1b[?1049h")
	defer fmt.Fprint(out, "\x1b[?25h\x1b[?1049l\x1b[0m")

	app := tuiState{selected: 0}
	for {
		renderTUI(out, app)
		event, err := readKey(os.Stdin)
		if err != nil {
			return err
		}
		switch event.Key {
		case keyCtrlC:
			return nil
		case keyUp:
			if app.selected > 0 {
				app.selected--
			}
		case keyDown:
			if app.selected < len(shellOptions())-1 {
				app.selected++
			}
		case keySlash:
			app.palette = true
			app.filter = ""
			app.selected = 0
		case keyEscape:
			if app.palette {
				app.palette = false
				app.filter = ""
				app.selected = 0
			} else {
				return nil
			}
		case keyBackspace:
			if app.palette && len(app.filter) > 0 {
				app.filter = app.filter[:len(app.filter)-1]
				app.selected = 0
			}
		case keyRune:
			if app.palette {
				app.filter += string(event.Ch)
				app.selected = 0
			} else {
				switch event.Ch {
				case 'q':
					return nil
				case 'j':
					if app.selected < len(shellOptions())-1 {
						app.selected++
					}
				case 'k':
					if app.selected > 0 {
						app.selected--
					}
				case '?':
					app.palette = true
					app.filter = ""
					app.selected = 0
				}
			}
		case keyEnter:
			options := filteredOptions(app.filter)
			if len(options) == 0 {
				continue
			}
			if app.selected >= len(options) {
				app.selected = len(options) - 1
			}
			option := options[app.selected]
			action, err := runTUIOption(ctx, cfg, option, state, out)
			if err != nil {
				showTUIMessage(out, state, "error: "+err.Error())
			}
			if action == commandExit {
				return nil
			}
			app.palette = false
			app.filter = ""
			app.selected = 0
		}
	}
}

type tuiState struct {
	selected int
	palette  bool
	filter   string
}

func renderTUI(out io.Writer, app tuiState) {
	clearTUIScreen(out)
	options := filteredOptions(app.filter)
	fmt.Fprintln(out, renderWelcomeDeck())
	fmt.Fprintln(out)
	if app.palette {
		fmt.Fprintln(out, styleHeader.Render("Command Palette"))
		fmt.Fprintln(out, styleSubtle.Render("Filtro: /"+app.filter))
	} else {
		fmt.Fprintln(out, styleHeader.Render("Mission Control"))
	}
	fmt.Fprintln(out)
	lines := make([]string, 0, len(options))
	for i, option := range options {
		line := fmt.Sprintf("%-12s %s", option.Key, option.Description)
		if i == app.selected {
			lines = append(lines, styleSelected.Render("> "+line))
		} else {
			lines = append(lines, "  "+styleCommand.Render(option.Key)+"  "+styleSubtle.Render(option.Description))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, styleSubtle.Render("Sin acciones para ese filtro."))
	}
	fmt.Fprintln(out, stylePanel.Render(strings.Join(lines, "\n")))
	fmt.Fprintln(out)
	fmt.Fprintln(out, styleSubtle.Render("Tip: /tasks <project> tambien funciona desde el modo shell simple."))
	renderFooter(out, "↑/↓ navegar   Enter ejecutar   / palette   Esc volver   q salir   Ctrl+C salir")
}

func renderWelcomeDeck() string {
	width, _, err := xterm.GetSize(os.Stdout.Fd())
	if err != nil || width < 92 {
		return BrandHeader("TERMINAL UI")
	}
	leftWidth := 42
	rightWidth := width - leftWidth - 6
	if rightWidth > 105 {
		rightWidth = 105
	}
	if rightWidth < 50 {
		rightWidth = 50
	}

	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	if home != "" {
		cwd = strings.Replace(cwd, home, "~", 1)
	}

	left := lipgloss.NewStyle().
		Width(leftWidth).
		Height(10).
		Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FACC15")).
		Padding(1, 2).
		Render(strings.Join([]string{
			styleBrand.Render("Welcome back."),
			"",
			styleBrand.Render(batMascot),
			"",
			styleStudioName.Render("BattOS Mission Control"),
			styleBrandMeta.Render(cwd),
		}, "\n"))

	rightBody := strings.Join([]string{
		styleBrand.Render("Tips for getting started"),
		styleStudioName.Render("Run /projects to review your work board"),
		styleStudioName.Render("Run /tasks <project> to inspect active tasks"),
		styleBrandMeta.Render("Use Esc to go back, q to leave the terminal UI"),
		strings.Repeat("─", maxInt(20, rightWidth-6)),
		styleBrand.Render("What's new"),
		styleStudioName.Render("TUI v1 now has a wide welcome deck, command palette and fixed footer"),
		styleBrandMeta.Render("Branding uses an original BattOS bat mascot, not any external logo"),
	}, "\n")

	right := lipgloss.NewStyle().
		Width(rightWidth).
		Height(10).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FACC15")).
		Padding(1, 2).
		Render(rightBody)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func filteredOptions(filter string) []shellOption {
	all := shellOptions()
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return all
	}
	var out []shellOption
	for _, option := range all {
		haystack := strings.ToLower(option.Key + " " + option.Description)
		if strings.Contains(haystack, filter) {
			out = append(out, option)
		}
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func runTUIOption(ctx context.Context, cfg ShellConfig, option shellOption, state *xterm.State, out io.Writer) (commandResultAction, error) {
	args := append([]string(nil), option.Args...)
	if option.NeedsInput != "" {
		value, err := promptTUIInput(option.NeedsInput, state, out)
		if err != nil {
			return commandBack, err
		}
		if strings.TrimSpace(value) == "" {
			return commandBack, nil
		}
		args = append(args, strings.TrimSpace(value))
	}
	return runTUICommand(ctx, cfg, args, state, out)
}

func promptTUIInput(label string, state *xterm.State, out io.Writer) (string, error) {
	if err := xterm.Restore(os.Stdin.Fd(), state); err != nil {
		return "", err
	}
	clearTUIScreen(out)
	fmt.Fprint(out, "\x1b[?25h")
	PrintBanner("TERMINAL UI")
	fmt.Fprint(out, stylePrompt.Render(label+" > "))
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	newState, rawErr := xterm.MakeRaw(os.Stdin.Fd())
	if rawErr == nil {
		*state = *newState
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func runTUICommand(ctx context.Context, cfg ShellConfig, args []string, state *xterm.State, out io.Writer) (commandResultAction, error) {
	if err := xterm.Restore(os.Stdin.Fd(), state); err != nil {
		return commandBack, err
	}
	clearTUIScreen(out)
	fmt.Fprint(out, "\x1b[?25h")
	err := runBattOSCommand(ctx, cfg, args, out)
	if err != nil {
		fmt.Fprintln(out)
		fmt.Fprintln(out, friendlyCommandError(err, cfg.APIURL))
	}
	newState, rawErr := xterm.MakeRaw(os.Stdin.Fd())
	if rawErr == nil {
		*state = *newState
	}
	renderFooter(out, "Esc/Enter volver   q salir   Ctrl+C salir")
	action := waitTUIReturn(os.Stdin)
	return action, nil
}

func showTUIMessage(out io.Writer, state *xterm.State, message string) {
	_ = xterm.Restore(os.Stdin.Fd(), state)
	clearTUIScreen(out)
	fmt.Fprint(out, "\x1b[?25h")
	fmt.Fprintln(out, styleDown.Render(message))
	newState, err := xterm.MakeRaw(os.Stdin.Fd())
	if err == nil {
		*state = *newState
	}
	renderFooter(out, "Esc/Enter volver   q salir   Ctrl+C salir")
	_ = waitTUIReturn(os.Stdin)
}

func clearTUIScreen(out io.Writer) {
	fmt.Fprint(out, "\x1b[?25l\x1b[H\x1b[2J\x1b[3J")
}

func renderFooter(out io.Writer, text string) {
	_, height, err := xterm.GetSize(os.Stdout.Fd())
	footer := styleSubtle.Render(text)
	if err != nil || height <= 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, footer)
		return
	}
	fmt.Fprintf(out, "\x1b[%d;1H\x1b[2K%s", height, footer)
}

func waitTUIReturn(in io.Reader) commandResultAction {
	for {
		event, err := readKey(in)
		if err != nil {
			return commandBack
		}
		switch event.Key {
		case keyEnter, keyEscape:
			return commandBack
		case keyCtrlC:
			return commandExit
		case keyRune:
			if event.Ch == 'q' {
				return commandExit
			}
		}
	}
}

func friendlyCommandError(err error, apiURL string) string {
	msg := err.Error()
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "No connection could be made") {
		return styleDown.Render("BattOS API no esta corriendo.") + "\n" +
			styleSubtle.Render("El comando intento conectarse a "+apiURL+". Inicia el API y vuelve a ejecutar la accion.")
	}
	return styleDown.Render("El comando termino con error: ") + styleSubtle.Render(msg)
}

func readKey(in io.Reader) (keyEvent, error) {
	buf := make([]byte, 1)
	if _, err := in.Read(buf); err != nil {
		return keyEvent{}, err
	}
	switch buf[0] {
	case 3:
		return keyEvent{Key: keyCtrlC}, nil
	case 13, 10:
		return keyEvent{Key: keyEnter}, nil
	case 27:
		seq, err := readBytesWithTimeout(in, 2, 25*time.Millisecond)
		if err == nil && len(seq) == 2 && seq[0] == '[' {
			switch seq[1] {
			case 'A':
				return keyEvent{Key: keyUp}, nil
			case 'B':
				return keyEvent{Key: keyDown}, nil
			}
		}
		return keyEvent{Key: keyEscape}, nil
	case 8, 127:
		return keyEvent{Key: keyBackspace}, nil
	case '/':
		return keyEvent{Key: keySlash, Ch: '/'}, nil
	default:
		if buf[0] >= 32 {
			return keyEvent{Key: keyRune, Ch: rune(buf[0])}, nil
		}
		return keyEvent{Key: keyUnknown}, nil
	}
}

func readBytesWithTimeout(in io.Reader, size int, timeout time.Duration) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, size)
		n, err := io.ReadFull(in, buf)
		ch <- result{data: buf[:n], err: err}
	}()
	select {
	case res := <-ch:
		return res.data, res.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout")
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
