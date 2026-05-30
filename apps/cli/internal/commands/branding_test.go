package commands

import (
	"strings"
	"testing"
)

func TestBrandHeaderUsesOriginalBattOSMark(t *testing.T) {
	got := BrandHeader("TERMINAL UI")
	for _, want := range []string{"BATT-OS", "MISSION CONTROL", "Nicotion.dev"} {
		if !strings.Contains(got, want) {
			t.Fatalf("BrandHeader missing %q in %q", want, got)
		}
	}
	if !strings.Contains(batMascot, "o.o") {
		t.Fatalf("batMascot should keep the original mascot face, got %q", batMascot)
	}
	if strings.Contains(strings.ToLower(got), "batman") {
		t.Fatalf("BrandHeader should use BattOS branding, got %q", got)
	}
}
