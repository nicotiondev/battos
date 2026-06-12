// Package commands — `battos serve`
//
// Levanta el servidor HTTP de BattOS (API + worker + dashboard) en un solo
// proceso, relanzando el binario battos-api que debe estar compilado y
// accesible en PATH o junto al binario battos.
package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewServeCmd construye el subcomando `battos serve`.
//
// Busca el binario battos-api en:
//  1. PATH del sistema.
//  2. Directorio del binario battos actual (deployment side-by-side).
//
// Si no lo encuentra, devuelve un error con instrucciones de compilación.
func NewServeCmd() *cobra.Command {
	var port int
	var dataDir string
	var open bool

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Arranca BattOS (API + worker + dashboard) en un solo comando",
		Long: `Levanta el servidor HTTP de BattOS con el dashboard embebido.

El dashboard queda disponible en http://localhost:<port>.

Requiere que el binario battos-api esté compilado y accesible:

  go build -o battos-api ./apps/api/cmd/api

O junto al binario battos en el mismo directorio.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			apiPath, err := findAPIBinary()
			if err != nil {
				return fmt.Errorf("battos-api no encontrado: %w\nCompilá con: go build -o battos-api ./apps/api/cmd/api", err)
			}

			env := os.Environ()
			if port != 8000 {
				env = append(env, fmt.Sprintf("BATTOS_API_PORT=%d", port))
			}
			if dataDir != "" {
				env = append(env, fmt.Sprintf("BATTOS_DATABASE_PATH=%s", filepath.Join(dataDir, "battos.db")))
			}

			fmt.Printf("BattOS UI → http://localhost:%d\n", port)
			fmt.Println("Ctrl+C para detener.")

			if open {
				openBrowser(fmt.Sprintf("http://localhost:%d", port))
			}

			proc := exec.CommandContext(cmd.Context(), apiPath)
			proc.Env = env
			proc.Stdout = os.Stdout
			proc.Stderr = os.Stderr
			return proc.Run()
		},
	}

	cmd.Flags().IntVar(&port, "port", 8000, "Puerto del servidor")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Directorio de datos (default: data/)")
	cmd.Flags().BoolVar(&open, "open", false, "Abrir el dashboard en el browser al arrancar")

	return cmd
}

// findAPIBinary localiza el binario battos-api en dos lugares:
//  1. PATH del sistema (instalación global).
//  2. Mismo directorio que el binario battos actual (deployment side-by-side).
func findAPIBinary() (string, error) {
	// 1. PATH
	if p, err := exec.LookPath("battos-api"); err == nil {
		return p, nil
	}

	// 2. Side-by-side con el binario actual
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("no se pudo resolver el path del ejecutable: %w", err)
	}
	candidate := filepath.Join(filepath.Dir(self), "battos-api")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	// Windows: probar también con extensión .exe
	candidateExe := candidate + ".exe"
	if _, err := os.Stat(candidateExe); err == nil {
		return candidateExe, nil
	}

	return "", fmt.Errorf("no se encontró battos-api en PATH ni junto a battos")
}

// openBrowser abre la URL en el browser por defecto del sistema.
// El error se ignora deliberadamente: si el browser no abre, el servidor
// sigue corriendo igual.
func openBrowser(url string) {
	// Windows
	_ = exec.Command("cmd", "/c", "start", url).Start()
}
