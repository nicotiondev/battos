// Package main es el entrypoint del binario `battos`.
//
// Root cobra command que registra todos los subcomandos del CLI:
//   - status: estado general del OS (Fase 1)
//   - project, agent, skill, runtime, cli, mcp, memory, usage: Fase 3
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/nicotion/battos/apps/cli/internal/commands"
	"github.com/spf13/cobra"
)

// Version info — inyectada via ldflags en build:
//
//	go build -ldflags "-X main.version=v0.1.0 -X main.commit=..." ...
var (
	version = "v0.1.0-alpha"
	commit  = "dev"
)

func main() {
	if err := newRootCmd().ExecuteContext(context.Background()); err != nil {
		// cobra ya imprime el error; salimos con código distinto a 0.
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var apiURL string
	var apiToken string

	root := &cobra.Command{
		Use:           "battos",
		Short:         "BattOS — AI Operating System self-hosted",
		Long:          longDescription,
		Version:       fmt.Sprintf("%s (commit %s)", version, commit),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			commands.PrintBanner("CLI")
			return cmd.Help()
		},
	}

	// Flag global: URL del API. Por defecto localhost, override con --api o BATTOS_API_URL.
	root.PersistentFlags().StringVar(&apiURL, "api", defaultAPIURL(), "URL del API BattOS")
	root.PersistentFlags().StringVar(&apiToken, "token", defaultAPIToken(), "Token de acceso BattOS")

	// Factory que devuelve un cliente con el apiURL resuelto en tiempo de ejecución.
	getClient := func() *client.Client {
		return client.New(apiURL, apiToken)
	}

	// Subcomandos
	root.AddCommand(commands.NewStatusCmd(getClient))
	root.AddCommand(commands.NewMemoryCmd(getClient))
	root.AddCommand(commands.NewDomainCmd(getClient))
	root.AddCommand(commands.NewProjectCmd(getClient))
	root.AddCommand(commands.NewGoalCmd(getClient))
	root.AddCommand(commands.NewTaskCmd(getClient))

	return root
}

// defaultAPIURL resuelve la URL por defecto del API:
//  1. BATTOS_API_URL si está seteado
//  2. http://localhost:8000 (default dev)
func defaultAPIURL() string {
	if v := os.Getenv("BATTOS_API_URL"); v != "" {
		return v
	}
	return "http://localhost:8000"
}

func defaultAPIToken() string {
	return os.Getenv("BATTOS_API_TOKEN")
}

const longDescription = `BattOS es una capa agentic self-hosted para orquestar proyectos,
agentes, skills, modelos, memoria, MCP, herramientas CLI, workflows y logs
desde un único panel.

Este binario es el CLI cliente. Habla con el API de BattOS por HTTP.
Asegurate de que el API esté corriendo (docker compose up -d) antes
de usar los comandos.`
