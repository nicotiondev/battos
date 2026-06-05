package commands

import (
	"fmt"

	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/spf13/cobra"
)

type runtimeAdapterItem struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Status               string `json:"status"`
	Version              string `json:"version"`
	Executable           string `json:"executable"`
	Command              string `json:"command"`
	ApprovalRequired     bool   `json:"approval_required"`
	ApprovedForExecution bool   `json:"approved_for_execution"`
	RequiresAuth         bool   `json:"requires_auth"`
}

type cliToolItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Command      string `json:"command"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	DetectedPath string `json:"detected_path"`
	Version      string `json:"version"`
	RuntimeID    string `json:"runtime_id"`
}

type providerItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	EnvKey string `json:"env_key"`
	Status string `json:"status"`
}

func NewRuntimeCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "runtime", Short: "Runtime adapters aprobados y detectados"}
	cmd.AddCommand(newRuntimeListCmd(getClient), newRuntimeDetectCmd(getClient))
	return cmd
}

func NewProviderCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "provider", Short: "Providers LLM configurados por entorno"}
	cmd.AddCommand(newProviderListCmd(getClient), newProviderDetectCmd(getClient))
	return cmd
}

func NewCLIToolCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "cli-tool", Short: "CLIs detectadas en el host"}
	cmd.AddCommand(newCLIToolListCmd(getClient))
	return cmd
}

func newRuntimeListCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listar runtime adapters",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []runtimeAdapterItem
			if err := workGet(cmd, getClient(), "/runtime-adapters", &items); err != nil {
				return err
			}
			printRuntimeAdapters(items)
			return nil
		},
	}
}

func newRuntimeDetectCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "detect",
		Short: "Detectar Claude Code y Codex sin autorizar ejecucion",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []runtimeAdapterItem
			if err := workPost(cmd, getClient(), "/runtime-adapters/detect", map[string]any{}, &items); err != nil {
				return err
			}
			printRuntimeAdapters(items)
			fmt.Println(styleSubtle.Render("Nota: detectar una CLI no concede permiso de ejecucion."))
			return nil
		},
	}
}

func newProviderListCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listar providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []providerItem
			if err := workGet(cmd, getClient(), "/providers", &items); err != nil {
				return err
			}
			printProviders(items)
			return nil
		},
	}
}

func newProviderDetectCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "detect",
		Short: "Validar presencia de API keys sin exponerlas",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []providerItem
			if err := workPost(cmd, getClient(), "/providers/detect", map[string]any{}, &items); err != nil {
				return err
			}
			printProviders(items)
			return nil
		},
	}
}

func newCLIToolListCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listar CLIs detectadas",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []cliToolItem
			if err := workGet(cmd, getClient(), "/cli-tools", &items); err != nil {
				return err
			}
			PrintBanner("RUNTIME DETECTION")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin CLIs detectadas; ejecuta battos runtime detect)"))
				return nil
			}
			printWorkHeader("ID", "COMMAND", "STATUS", "RUNTIME", "PATH")
			for _, item := range items {
				fmt.Printf("%s  %s  %s  %s  %s\n", styleOK.Render(item.ID), item.Command, styleSubtle.Render(item.Status), styleSubtle.Render(emptyDash(item.RuntimeID)), styleSubtle.Render(emptyDash(item.DetectedPath)))
			}
			return nil
		},
	}
}

func printRuntimeAdapters(items []runtimeAdapterItem) {
	PrintBanner("RUNTIME DETECTION")
	if len(items) == 0 {
		fmt.Println(styleSubtle.Render("(sin runtimes registrados)"))
		return
	}
	printWorkHeader("ID", "STATUS", "APPROVED", "COMMAND", "VERSION", "EXECUTABLE")
	for _, item := range items {
		approved := "no"
		if item.ApprovedForExecution {
			approved = "yes"
		}
		fmt.Printf("%s  %s  %s  %s  %s  %s\n", styleOK.Render(item.ID), styleSubtle.Render(item.Status), styleSubtle.Render(approved), emptyDash(item.Command), styleSubtle.Render(emptyDash(item.Version)), styleSubtle.Render(emptyDash(item.Executable)))
	}
	fmt.Println(styleSubtle.Render("Aprobacion: detectar o configurar no autoriza ejecucion; la aprobacion se pedira por run."))
}

func printProviders(items []providerItem) {
	PrintBanner("PROVIDER STATUS")
	if len(items) == 0 {
		fmt.Println(styleSubtle.Render("(sin providers registrados)"))
		return
	}
	printWorkHeader("ID", "STATUS", "ENV KEY")
	for _, item := range items {
		fmt.Printf("%s  %s  %s\n", styleOK.Render(item.ID), styleSubtle.Render(item.Status), styleSubtle.Render(item.EnvKey))
	}
	fmt.Println(styleSubtle.Render("Las API keys solo se leen como presencia/ausencia; BattOS no imprime secretos."))
}
