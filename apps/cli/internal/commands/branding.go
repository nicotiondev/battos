package commands

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

const wordmark = `   ____        __  __  ____  _____
  / __ )____ _/ /_/ /_/ __ \/ ___/
 / __  / __ '/ __/ __/ / / /\__ \
/ /_/ / /_/ / /_/ /_/ /_/ /___/ /
/_____/\__,_/\__/\__/\____//____/`

var (
	styleBrand = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981"))

	styleBrandMeta = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	styleStudioMark = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F9FAFB"))

	styleStudioName = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#D1D5DB"))
)

// PrintBanner renders BattOS identity before interactive command output.
func PrintBanner(section string) {
	fmt.Println()
	fmt.Println(styleBrand.Render(wordmark))
	if section != "" {
		fmt.Println(styleBrandMeta.Render("  MISSION CONTROL  //  " + section))
	}
	fmt.Printf("  %s  %s %s\n",
		styleBrandMeta.Render("DESARROLLADO POR"),
		styleStudioMark.Render("[ N ]"),
		styleStudioName.Render("Nicotion.dev"),
	)
	fmt.Println()
}
