package commands

import (
	"bufio"
	"bytes"
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
	APIURL   string
	Token    string
	Language string
	In       io.Reader
	Out      io.Writer
}

type shellOption struct {
	Key         string
	Description string
	Args        []string
	NeedsInput  string
	Action      string
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

type tuiLanguage string

const (
	tuiLanguageES tuiLanguage = "es"
	tuiLanguageEN tuiLanguage = "en"
)

const shellActionLanguage = "language"

type tuiCopy struct {
	welcome           string
	missionControl    string
	commandPalette    string
	filter            string
	noActions         string
	tip               string
	footer            string
	resultFooter      string
	inputSection      string
	resultTitle       string
	noOutput          string
	apiOffline        string
	apiOfflineDetail  string
	commandError      string
	languageTitle     string
	languageHelp      string
	languageUpdated   string
	tipsTitle         string
	tipProjects       string
	tipTasks          string
	tipBack           string
	whatsNewTitle     string
	whatsNewLine      string
	brandingLine      string
	missionName       string
	lineShellHelp     string
	lineShellClose    string
	availableActions  string
	choose            string
	projectID         string
	statusDescription string
	domainsDesc       string
	projectsDesc      string
	goalsDesc         string
	tasksDesc         string
	memoryDesc        string
	helpDesc          string
	languageDesc      string
	terminalUI        string
}

var tuiCopies = map[tuiLanguage]tuiCopy{
	tuiLanguageES: {
		welcome:           "Bienvenido.",
		missionControl:    "Mission Control",
		commandPalette:    "Paleta de Comandos",
		filter:            "Filtro",
		noActions:         "Sin acciones para ese filtro.",
		tip:               "Tip: /tasks <project> tambien funciona desde el modo shell simple.",
		footer:            "↑/↓ navegar   Enter ejecutar   / palette   Esc volver   l idioma   Ctrl+C salir",
		resultFooter:      "Esc/Enter volver   Ctrl+C salir",
		inputSection:      "TERMINAL UI",
		resultTitle:       "BattOS // Resultado",
		noOutput:          "(sin salida)",
		apiOffline:        "BattOS API no esta corriendo.",
		apiOfflineDetail:  "El comando intento conectarse a %s. Inicia el API y vuelve a ejecutar la accion.",
		commandError:      "El comando termino con error: ",
		languageTitle:     "Idioma",
		languageHelp:      "↑/↓ seleccionar   Enter aplicar   Esc volver",
		languageUpdated:   "Idioma actualizado.",
		tipsTitle:         "Tips para empezar",
		tipProjects:       "Ejecuta /projects para revisar tu tablero",
		tipTasks:          "Ejecuta /tasks <project> para revisar tareas activas",
		tipBack:           "Usa Esc para volver y Ctrl+C para salir de la TUI",
		whatsNewTitle:     "Novedades",
		whatsNewLine:      "TUI v1 usa deck amplio, paleta de comandos, idioma y footer fijo",
		brandingLine:      "Branding con mascota BattOS original, sin logos externos",
		missionName:       "BattOS Mission Control",
		lineShellHelp:     "Escribe / para ver acciones, /help para ayuda, /exit para salir.",
		lineShellClose:    "Cerrando BattOS shell.",
		availableActions:  "Acciones disponibles",
		choose:            "elige",
		projectID:         "project id",
		statusDescription: "Estado general del OS",
		domainsDesc:       "Listar dominios",
		projectsDesc:      "Listar proyectos",
		goalsDesc:         "Listar objetivos por proyecto",
		tasksDesc:         "Listar tareas por proyecto",
		memoryDesc:        "Ver estadisticas de memoria",
		helpDesc:          "Ayuda del CLI",
		languageDesc:      "Cambiar idioma",
		terminalUI:        "TERMINAL UI",
	},
	tuiLanguageEN: {
		welcome:           "Welcome back.",
		missionControl:    "Mission Control",
		commandPalette:    "Command Palette",
		filter:            "Filter",
		noActions:         "No actions for that filter.",
		tip:               "Tip: /tasks <project> also works from the simple shell mode.",
		footer:            "↑/↓ navigate   Enter run   / palette   Esc back   l language   Ctrl+C quit",
		resultFooter:      "Esc/Enter back   Ctrl+C quit",
		inputSection:      "TERMINAL UI",
		resultTitle:       "BattOS // Command Result",
		noOutput:          "(no output)",
		apiOffline:        "BattOS API is not running.",
		apiOfflineDetail:  "The command tried to connect to %s. Start the API and run the action again.",
		commandError:      "The command ended with an error: ",
		languageTitle:     "Language",
		languageHelp:      "↑/↓ select   Enter apply   Esc back",
		languageUpdated:   "Language updated.",
		tipsTitle:         "Tips for getting started",
		tipProjects:       "Run /projects to review your work board",
		tipTasks:          "Run /tasks <project> to inspect active tasks",
		tipBack:           "Use Esc to go back and Ctrl+C to leave the terminal UI",
		whatsNewTitle:     "What's new",
		whatsNewLine:      "TUI v1 has a wide deck, command palette, language and fixed footer",
		brandingLine:      "Branding uses an original BattOS mascot, not any external logo",
		missionName:       "BattOS Mission Control",
		lineShellHelp:     "Type / to see actions, /help for help, /exit to quit.",
		lineShellClose:    "Closing BattOS shell.",
		availableActions:  "Available actions",
		choose:            "choose",
		projectID:         "project id",
		statusDescription: "OS status overview",
		domainsDesc:       "List domains",
		projectsDesc:      "List projects",
		goalsDesc:         "List goals by project",
		tasksDesc:         "List tasks by project",
		memoryDesc:        "View memory statistics",
		helpDesc:          "CLI help",
		languageDesc:      "Change language",
		terminalUI:        "TERMINAL UI",
	},
}

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
	copy := copyForLanguage(normalizeTUILanguage(cfg.Language))
	in := cfg.In
	if in == nil {
		in = os.Stdin
	}
	out := cfg.Out
	if out == nil {
		out = os.Stdout
	}

	PrintBanner("INTERACTIVE SHELL")
	fmt.Fprintln(out, styleSubtle.Render(copy.lineShellHelp))
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
			fmt.Fprintln(out, styleSubtle.Render(copy.lineShellClose))
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

	app := tuiState{selected: 0, language: normalizeTUILanguage(cfg.Language)}
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
			if app.selected < len(shellOptions(app.language))-1 {
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
				case 'l':
					language, action := selectTUILanguage(app.language, out)
					if action == commandExit {
						return nil
					}
					app.language = language
					app.selected = 0
				case 'j':
					if app.selected < len(shellOptions(app.language))-1 {
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
			options := filteredOptions(app.filter, app.language)
			if len(options) == 0 {
				continue
			}
			if app.selected >= len(options) {
				app.selected = len(options) - 1
			}
			option := options[app.selected]
			if option.Action == shellActionLanguage {
				language, action := selectTUILanguage(app.language, out)
				if action == commandExit {
					return nil
				}
				app.language = language
				app.palette = false
				app.filter = ""
				app.selected = 0
				continue
			}
			cfg.Language = string(app.language)
			action, err := runTUIOption(ctx, cfg, option, state, out, app.language)
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
	language tuiLanguage
}

func renderTUI(out io.Writer, app tuiState) {
	clearTUIScreen(out)
	copy := copyForLanguage(app.language)
	options := filteredOptions(app.filter, app.language)
	fmt.Fprintln(out, renderWelcomeDeck(app.language))
	fmt.Fprintln(out)
	if app.palette {
		fmt.Fprintln(out, styleHeader.Render(copy.commandPalette))
		fmt.Fprintln(out, styleSubtle.Render(copy.filter+": /"+app.filter))
	} else {
		fmt.Fprintln(out, styleHeader.Render(copy.missionControl))
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
		lines = append(lines, styleSubtle.Render(copy.noActions))
	}
	fmt.Fprintln(out, stylePanel.Render(strings.Join(lines, "\n")))
	fmt.Fprintln(out)
	fmt.Fprintln(out, styleSubtle.Render(copy.tip))
	renderFooter(out, copy.footer)
}

func renderWelcomeDeck(language tuiLanguage) string {
	width, _, err := xterm.GetSize(os.Stdout.Fd())
	copy := copyForLanguage(language)
	if err != nil || width < 92 {
		return renderCompactWelcomeDeck(copy, width)
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
			styleBrand.Render(copy.welcome),
			"",
			PixelBatMascot(),
			"",
			styleStudioName.Render(copy.missionName),
			styleBrandMeta.Render(cwd),
		}, "\n"))

	rightBody := strings.Join([]string{
		styleBrand.Render(copy.tipsTitle),
		styleStudioName.Render(copy.tipProjects),
		styleStudioName.Render(copy.tipTasks),
		styleBrandMeta.Render(copy.tipBack),
		strings.Repeat("─", maxInt(20, rightWidth-6)),
		styleBrand.Render(copy.whatsNewTitle),
		styleStudioName.Render(copy.whatsNewLine),
		styleBrandMeta.Render(copy.brandingLine),
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

func renderCompactWelcomeDeck(copy tuiCopy, width int) string {
	panelWidth := width - 4
	if panelWidth < 38 {
		panelWidth = 38
	}
	if panelWidth > 58 {
		panelWidth = 58
	}
	return lipgloss.NewStyle().
		Width(panelWidth).
		Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FACC15")).
		Padding(1, 2).
		Render(strings.Join([]string{
			styleBrand.Render(copy.welcome),
			"",
			PixelBatMascot(),
			"",
			styleStudioName.Render(copy.missionName),
		}, "\n"))
}

func filteredOptions(filter string, language tuiLanguage) []shellOption {
	all := shellOptions(language)
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

func normalizeTUILanguage(value string) tuiLanguage {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "en", "eng", "english":
		return tuiLanguageEN
	default:
		return tuiLanguageES
	}
}

func copyForLanguage(language tuiLanguage) tuiCopy {
	copy, ok := tuiCopies[language]
	if !ok {
		return tuiCopies[tuiLanguageES]
	}
	return copy
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func selectTUILanguage(current tuiLanguage, out io.Writer) (tuiLanguage, commandResultAction) {
	selected := 0
	if current == tuiLanguageEN {
		selected = 1
	}
	languages := []struct {
		value tuiLanguage
		label string
	}{
		{value: tuiLanguageES, label: "Español"},
		{value: tuiLanguageEN, label: "English"},
	}
	for {
		copy := copyForLanguage(current)
		clearTUIScreen(out)
		fmt.Fprintln(out, renderCompactWelcomeDeck(copy, 72))
		fmt.Fprintln(out)
		fmt.Fprintln(out, styleHeader.Render(copy.languageTitle))
		fmt.Fprintln(out)
		lines := make([]string, 0, len(languages))
		for i, language := range languages {
			line := fmt.Sprintf("%-10s %s", language.value, language.label)
			if i == selected {
				lines = append(lines, styleSelected.Render("> "+line))
			} else {
				lines = append(lines, "  "+styleCommand.Render(string(language.value))+"  "+styleSubtle.Render(language.label))
			}
		}
		fmt.Fprintln(out, stylePanel.Render(strings.Join(lines, "\n")))
		renderFooter(out, copy.languageHelp)
		event, err := readKey(os.Stdin)
		if err != nil {
			return current, commandBack
		}
		switch event.Key {
		case keyCtrlC:
			return current, commandExit
		case keyEscape:
			return current, commandBack
		case keyUp:
			if selected > 0 {
				selected--
			}
		case keyDown:
			if selected < len(languages)-1 {
				selected++
			}
		case keyEnter:
			return languages[selected].value, commandBack
		case keyRune:
			switch strings.ToLower(string(event.Ch)) {
			case "e":
				return tuiLanguageES, commandBack
			case "i":
				return tuiLanguageEN, commandBack
			}
		}
	}
}

func runTUIOption(ctx context.Context, cfg ShellConfig, option shellOption, state *xterm.State, out io.Writer, language tuiLanguage) (commandResultAction, error) {
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
	return runTUICommand(ctx, cfg, args, state, out, language)
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

func runTUICommand(ctx context.Context, cfg ShellConfig, args []string, state *xterm.State, out io.Writer, language tuiLanguage) (commandResultAction, error) {
	clearTUIScreen(out)
	fmt.Fprint(out, "\x1b[?25h")
	output, err := runBattOSCommandOutput(ctx, cfg, args)
	renderCommandResult(out, args, output, err, cfg.APIURL, language)
	renderFooter(out, copyForLanguage(language).resultFooter)
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
	renderFooter(out, copyForLanguage(tuiLanguageES).resultFooter)
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
		}
	}
}

func friendlyCommandError(err error, apiURL string, language tuiLanguage) string {
	copy := copyForLanguage(language)
	msg := err.Error()
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "No connection could be made") {
		return styleDown.Render(copy.apiOffline) + "\n" +
			styleSubtle.Render(fmt.Sprintf(copy.apiOfflineDetail, apiURL))
	}
	return styleDown.Render(copy.commandError) + styleSubtle.Render(msg)
}

func renderCommandResult(out io.Writer, args []string, output string, err error, apiURL string, language tuiLanguage) {
	copy := copyForLanguage(language)
	width, height, sizeErr := xterm.GetSize(os.Stdout.Fd())
	if sizeErr != nil || width < 40 {
		width = 100
		height = 32
	}
	if height < 16 {
		height = 16
	}
	panelWidth := width - 4
	if panelWidth < 36 {
		panelWidth = 36
	}
	panelHeight := height - 8
	if panelHeight < 8 {
		panelHeight = 8
	}

	title := styleBrand.Render(copy.resultTitle)
	commandLine := styleSubtle.Render("$ battos " + strings.Join(args, " "))
	body := strings.TrimSpace(output)
	if body == "" {
		body = copy.noOutput
	}
	if err != nil {
		body += "\n\n" + friendlyCommandError(err, apiURL, language)
	}
	body = fitText(body, panelWidth-6, panelHeight-4)
	panel := lipgloss.NewStyle().
		Width(panelWidth).
		Height(panelHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FACC15")).
		Padding(1, 2).
		Render(title + "\n" + commandLine + "\n\n" + body)
	fmt.Fprintln(out, panel)
}

func fitText(text string, width, height int) string {
	if width <= 0 || height <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, height)
	for _, line := range lines {
		for len(line) > width {
			out = append(out, line[:width])
			line = line[width:]
			if len(out) == height {
				out[height-1] = out[height-1] + "..."
				return strings.Join(out, "\n")
			}
		}
		out = append(out, line)
		if len(out) == height {
			if len(lines) > height {
				out[height-1] = out[height-1] + "..."
			}
			return strings.Join(out, "\n")
		}
	}
	return strings.Join(out, "\n")
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
		seq, _ := readBytesWithTimeout(in, 8, 25*time.Millisecond)
		if len(seq) == 0 {
			return keyEvent{Key: keyEscape}, nil
		}
		if len(seq) >= 2 && seq[0] == '[' {
			switch seq[1] {
			case 'A':
				return keyEvent{Key: keyUp}, nil
			case 'B':
				return keyEvent{Key: keyDown}, nil
			}
		}
		return keyEvent{Key: keyUnknown}, nil
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
	language := normalizeTUILanguage(cfg.Language)
	copy := copyForLanguage(language)
	options := shellOptions(language)
	fmt.Fprintln(out, styleHeader.Render(copy.availableActions))
	for i, option := range options {
		fmt.Fprintf(out, "  %d. %-12s %s\n", i+1, styleCommand.Render(option.Key), styleSubtle.Render(option.Description))
	}
	fmt.Fprintln(out)
	fmt.Fprint(out, stylePrompt.Render(copy.choose+" > "))
	if !scanner.Scan() {
		return scanner.Err()
	}
	choice := strings.TrimSpace(scanner.Text())
	if choice == "" {
		return nil
	}
	for i, option := range options {
		if choice == fmt.Sprintf("%d", i+1) || choice == option.Key || choice == strings.TrimPrefix(option.Key, "/") {
			if option.Action == shellActionLanguage {
				fmt.Fprintln(out, styleSubtle.Render("TUI: presiona l o usa /language dentro de battos."))
				return nil
			}
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

func shellOptions(language tuiLanguage) []shellOption {
	copy := copyForLanguage(language)
	return []shellOption{
		{Key: "/status", Description: copy.statusDescription, Args: []string{"status"}},
		{Key: "/domains", Description: copy.domainsDesc, Args: []string{"domain", "list"}},
		{Key: "/projects", Description: copy.projectsDesc, Args: []string{"project", "list"}},
		{Key: "/goals", Description: copy.goalsDesc, Args: []string{"goal", "list", "--project"}, NeedsInput: copy.projectID},
		{Key: "/tasks", Description: copy.tasksDesc, Args: []string{"task", "list", "--project"}, NeedsInput: copy.projectID},
		{Key: "/memory", Description: copy.memoryDesc, Args: []string{"memory", "stats"}},
		{Key: "/language", Description: copy.languageDesc, Action: shellActionLanguage},
		{Key: "/help", Description: copy.helpDesc, Args: []string{"--help"}},
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
	if cfg.Language != "" {
		fullArgs = append(fullArgs, "--lang", cfg.Language)
	}
	fullArgs = append(fullArgs, args...)

	fmt.Fprintln(out, styleSubtle.Render("$ battos "+strings.Join(args, " ")))
	cmd := exec.CommandContext(ctx, exe, fullArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

func runBattOSCommandOutput(ctx context.Context, cfg ShellConfig, args []string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolviendo ejecutable: %w", err)
	}
	fullArgs := []string{}
	if cfg.APIURL != "" {
		fullArgs = append(fullArgs, "--api", cfg.APIURL)
	}
	if cfg.Token != "" {
		fullArgs = append(fullArgs, "--token", cfg.Token)
	}
	if cfg.Language != "" {
		fullArgs = append(fullArgs, "--lang", cfg.Language)
	}
	fullArgs = append(fullArgs, args...)
	cmd := exec.CommandContext(ctx, exe, fullArgs...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err = cmd.Run()
	return output.String(), err
}
