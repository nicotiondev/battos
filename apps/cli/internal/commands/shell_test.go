package commands

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestShellArgsMapsSlashAliases(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{name: "status", line: "/status", want: []string{"status"}},
		{name: "projects", line: "/projects", want: []string{"project", "list"}},
		{name: "tasks global", line: "/tasks", want: []string{"task", "list"}},
		{name: "tasks", line: "/tasks demo", want: []string{"task", "list", "--project", "demo"}},
		{name: "agents", line: "/agents", want: []string{"agent", "list"}},
		{name: "agent new", line: "/agent-new builder-web --name Builder --runtime codex", want: []string{"agent", "create", "builder-web", "--name", "Builder", "--runtime", "codex"}},
		{name: "task board global", line: "/task-board", want: []string{"task", "board"}},
		{name: "task board filtered", line: "/task-board demo", want: []string{"task", "board", "--project", "demo"}},
		{name: "memory default", line: "/memory", want: []string{"memory", "stats"}},
		{name: "runs global", line: "/runs", want: []string{"run", "list"}},
		{name: "runs filtered", line: "/runs demo", want: []string{"run", "list", "--project", "demo"}},
		{name: "run approve", line: "/run-approve 11111111-1111-1111-1111-111111111111", want: []string{"run", "approve", "11111111-1111-1111-1111-111111111111"}},
		{name: "run logs", line: "/run-logs 11111111-1111-1111-1111-111111111111", want: []string{"run", "logs", "11111111-1111-1111-1111-111111111111"}},
		{name: "plain command", line: "project list", want: []string{"project", "list"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := shellArgs(tt.line)
			if err != nil {
				t.Fatalf("shellArgs returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("shellArgs(%q) = %#v, want %#v", tt.line, got, tt.want)
			}
		})
	}
}

func TestFilteredOptionsNarrowsPalette(t *testing.T) {
	got := filteredOptions("proj", tuiLanguageES)
	if len(got) != 2 || got[0].Key != "/projects" || got[1].Key != "/project-new" {
		t.Fatalf("filteredOptions(proj) = %#v, want /projects and /project-new", got)
	}
}

func TestShellOptionsLocalizeLanguage(t *testing.T) {
	got := shellOptions(tuiLanguageEN)
	if len(got) == 0 {
		t.Fatal("shellOptions(en) returned no options")
	}
	if got[0].Description != "OS status overview" {
		t.Fatalf("shellOptions(en)[0].Description = %q, want English copy", got[0].Description)
	}
	foundLanguage := false
	for _, option := range got {
		if option.Key == "/language" && option.Action == shellActionLanguage {
			foundLanguage = true
		}
	}
	if !foundLanguage {
		t.Fatalf("shellOptions(en) missing /language action: %#v", got)
	}
}

func TestReadKeyParsesArrowDown(t *testing.T) {
	pendingKeyBytes = nil
	got, err := readKey(strings.NewReader("\x1b[B"))
	if err != nil {
		t.Fatalf("readKey returned error: %v", err)
	}
	if got.Key != keyDown {
		t.Fatalf("readKey = %#v, want keyDown", got)
	}
}

func TestReadKeyParsesArrowDownWithoutStealingNextKey(t *testing.T) {
	pendingKeyBytes = nil
	in := strings.NewReader("\x1b\x1b[B")
	got, err := readKey(in)
	if err != nil {
		t.Fatalf("readKey escape returned error: %v", err)
	}
	if got.Key != keyEscape {
		t.Fatalf("first readKey = %#v, want keyEscape", got)
	}
	got, err = readKey(in)
	if err != nil {
		t.Fatalf("readKey arrow returned error: %v", err)
	}
	if got.Key != keyDown {
		t.Fatalf("second readKey = %#v, want keyDown", got)
	}
}

func TestReadKeyParsesArrowUp(t *testing.T) {
	pendingKeyBytes = nil
	got, err := readKey(strings.NewReader("\x1b[A"))
	if err != nil {
		t.Fatalf("readKey returned error: %v", err)
	}
	if got.Key != keyUp {
		t.Fatalf("readKey = %#v, want keyUp", got)
	}
}

func TestReadKeyIgnoresFunctionKeys(t *testing.T) {
	pendingKeyBytes = nil
	tests := []struct {
		name  string
		input string
	}{
		{name: "F1 application mode", input: "\x1bOP"},
		{name: "F2 application mode", input: "\x1bOQ"},
		{name: "F3 application mode", input: "\x1bOR"},
		{name: "F1 bracket mode", input: "\x1b[11~"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readKey(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("readKey returned error: %v", err)
			}
			if got.Key != keyUnknown {
				t.Fatalf("readKey(%q) = %#v, want keyUnknown", tt.name, got)
			}
		})
	}
}

func TestReadKeyParsesEscape(t *testing.T) {
	pendingKeyBytes = nil
	got, err := readKey(strings.NewReader("\x1b"))
	if err != nil {
		t.Fatalf("readKey returned error: %v", err)
	}
	if got.Key != keyEscape {
		t.Fatalf("readKey = %#v, want keyEscape", got)
	}
}

func TestFriendlyCommandErrorExplainsOfflineAPI(t *testing.T) {
	got := friendlyCommandError(fmt.Errorf("dial tcp [::1]:8000: connectex: No connection could be made because the target machine actively refused it"), "http://localhost:8000", tuiLanguageES)
	if !strings.Contains(got, "BattOS API no esta corriendo") || !strings.Contains(got, "http://localhost:8000") {
		t.Fatalf("friendlyCommandError = %q, want offline API guidance", got)
	}
}

func TestWaitTUIReturnMapsEscapeToBack(t *testing.T) {
	got := waitTUIReturn(strings.NewReader("\x1b"))
	if got != commandBack {
		t.Fatalf("waitTUIReturn(Esc) = %v, want commandBack", got)
	}
}

func TestWaitTUIReturnMapsCtrlCToExit(t *testing.T) {
	got := waitTUIReturn(strings.NewReader("\x03"))
	if got != commandExit {
		t.Fatalf("waitTUIReturn(Ctrl+C) = %v, want commandExit", got)
	}
}

func TestFitTextClampsHeight(t *testing.T) {
	got := fitText("one\ntwo\nthree", 10, 2)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("fitText lines = %d, want 2: %q", len(lines), got)
	}
}
