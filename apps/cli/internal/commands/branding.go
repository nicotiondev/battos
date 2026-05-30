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

const batMascot = `  /\_/\
 < o.o >
  > ^ <
 /|___|\
   |_|`

var (
	styleBrand = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FACC15"))

	styleBrandMeta = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#71717A"))

	styleStudioMark = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FACC15"))

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
