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

const batMark = `     /\                 /\
 ___/  \__  BATT-OS  __/  \___
 \__    _/\/\____/\/\_    __/
    \__/              \__/`

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

func BrandHeader(section string) string {
	header := styleBrand.Render(batMark) + "\n" + styleBrand.Render(wordmark)
	if section != "" {
		header += "\n" + styleBrandMeta.Render("  MISSION CONTROL  //  "+section)
	}
	header += "\n" + fmt.Sprintf("  %s  %s %s",
		styleBrandMeta.Render("DESARROLLADO POR"),
		styleStudioMark.Render("[ N ]"),
		styleStudioName.Render("Nicotion.dev"),
	)
	return header
}

// PrintBanner renders BattOS identity before interactive command output.
func PrintBanner(section string) {
	fmt.Println()
	fmt.Println(BrandHeader(section))
	fmt.Println()
}
