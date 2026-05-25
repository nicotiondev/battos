// Package commands contiene los comandos cobra del CLI battos.
//
// Convención: un archivo por subcomando. Cada uno expone una func
// New<Cmd>Cmd() *cobra.Command que el root usa para registrarse.
package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/spf13/cobra"
)

// Estilos lipgloss compartidos por todos los comandos.
// Definidos en un solo lugar para mantener identidad visual consistente.
var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#10B981")). // verde BattOS
			Padding(0, 1)

	styleSubtle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")) // gris

	styleOK = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)

	styleDegraded = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true)

	styleDown = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)

	styleUnknown = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	styleKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Width(18)
)

// NewStatusCmd construye el comando `battos status`.
//
// Llama a /status del API y formatea la respuesta con colores.
// Es el "latido" del OS — la primera prueba de que todo está vivo.
func NewStatusCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Estado general del OS — versión, salud de subsistemas y métricas en vivo",
		Long: `Muestra el latido completo de BattOS:

  - Versión del binario, commit y build date
  - Estado de cada subsistema (config, sysmetrics, db, memory, ...)
  - Snapshot en vivo de CPU, memoria y red

Es el comando para validar que el API está corriendo y respondiendo.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			c := getClient()
			s, err := c.Status(ctx)
			if err != nil {
				return err
			}

			printStatus(s)
			return nil
		},
	}
}

func printStatus(s *client.StatusResponse) {
	fmt.Println()
	fmt.Println(styleHeader.Render("BattOS Command Center"))
	fmt.Println()

	// --- Versión ---
	fmt.Println(styleSubtle.Render("VERSION"))
	printKV("Version", s.Version.Version)
	printKV("Commit", s.Version.Commit)
	printKV("Build date", s.Version.BuildDate)
	printKV("Go", s.Version.GoVersion)
	fmt.Println()

	// --- Salud global ---
	fmt.Println(styleSubtle.Render("HEALTH"))
	printKV("Overall", colorStatus(s.Overall))
	for _, sub := range s.Subsystems {
		// Mostramos status en la columna principal y el detail (si hay)
		// en una segunda línea con estilo subtle, para no romper alineación.
		printKV(sub.Name, colorStatus(sub.Status))
		if sub.Detail != "" {
			fmt.Printf("  %s %s\n", styleKey.Render(""), styleSubtle.Render("↳ "+sub.Detail))
		}
	}
	fmt.Println()

	// --- Métricas en vivo ---
	fmt.Println(styleSubtle.Render("METRICS"))
	printKV("CPU", fmt.Sprintf("%.1f%%", s.Metrics.CPUPercent))
	printKV("Memory", fmt.Sprintf("%.1f%% (%d / %d MB)",
		s.Metrics.MemPercent, s.Metrics.MemUsedMB, s.Metrics.MemTotalMB))
	printKV("Net up", fmt.Sprintf("%.2f KB/s", s.Metrics.NetUploadKBps))
	printKV("Net down", fmt.Sprintf("%.2f KB/s", s.Metrics.NetDownloadKBps))
	fmt.Println()

	fmt.Println(styleSubtle.Render(
		fmt.Sprintf("Snapshot: %s", s.Timestamp.Format(time.RFC3339)),
	))
	fmt.Println()
}

// printKV imprime una línea "label: value" con la label gris a ancho fijo.
func printKV(key, value string) {
	fmt.Printf("  %s %s\n", styleKey.Render(key+":"), value)
}

// colorStatus pinta el estado según ok/degraded/down/unknown.
func colorStatus(status string) string {
	switch strings.ToLower(status) {
	case "ok":
		return styleOK.Render("● OK")
	case "degraded":
		return styleDegraded.Render("● DEGRADED")
	case "down":
		return styleDown.Render("● DOWN")
	default:
		return styleUnknown.Render("○ UNKNOWN")
	}
}
