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
		{name: "tasks", line: "/tasks demo", want: []string{"task", "list", "--project", "demo"}},
		{name: "memory default", line: "/memory", want: []string{"memory", "stats"}},
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

func TestShellArgsRequiresProjectForTasks(t *testing.T) {
	_, err := shellArgs("/tasks")
	if err == nil {
		t.Fatal("shellArgs(/tasks) error = nil, want usage error")
	}
}

func TestFilteredOptionsNarrowsPalette(t *testing.T) {
	got := filteredOptions("proj", tuiLanguageES)
	if len(got) != 1 || got[0].Key != "/projects" {
		t.Fatalf("filteredOptions(proj) = %#v, want only /projects", got)
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
	got, err := readKey(strings.NewReader("\x1b[B"))
	if err != nil {
		t.Fatalf("readKey returned error: %v", err)
	}
	if got.Key != keyDown {
		t.Fatalf("readKey = %#v, want keyDown", got)
	}
}

func TestReadKeyParsesEscape(t *testing.T) {
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

func TestWaitTUIReturnMapsQToExit(t *testing.T) {
	got := waitTUIReturn(strings.NewReader("q"))
	if got != commandExit {
		t.Fatalf("waitTUIReturn(q) = %v, want commandExit", got)
	}
}

func TestFitTextClampsHeight(t *testing.T) {
	got := fitText("one\ntwo\nthree", 10, 2)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("fitText lines = %d, want 2: %q", len(lines), got)
	}
}
