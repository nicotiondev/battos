package commands

import (
	"fmt"
	"os"
	"strings"

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

func PixelBatMascot() string {
	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color("#FACC15")).Render
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#A16207")).Render
	eye := lipgloss.NewStyle().Foreground(lipgloss.Color("#0F172A")).Background(lipgloss.Color("#FACC15")).Render
	rows := []string{
		"    " + yellow("██") + "          " + yellow("██"),
		"  " + yellow("██████") + "    " + yellow("██████"),
		yellow("████████████████████"),
		yellow("████") + eye(" ●") + yellow("████") + eye("● ") + yellow("████"),
		"  " + yellow("████████████████"),
		"    " + yellow("████████████"),
		"      " + dim("████████"),
	}
	return strings.Join(rows, "\n")
}

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
	if os.Getenv("BATTOS_NO_BANNER") == "1" {
		return
	}
	fmt.Println()
	fmt.Println(BrandHeader(section))
	fmt.Println()
}
