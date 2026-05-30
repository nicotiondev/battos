package commands

import (
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
	got := filteredOptions("proj")
	if len(got) != 1 || got[0].Key != "/projects" {
		t.Fatalf("filteredOptions(proj) = %#v, want only /projects", got)
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
