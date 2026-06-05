package commands

import (
	"fmt"

	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/spf13/cobra"
)

type agentItem struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	RuntimeID    string `json:"runtime_id"`
	SystemPrompt string `json:"system_prompt"`
	RiskLevel    string `json:"risk_level"`
	IsLead       bool   `json:"is_lead"`
	IsMeta       bool   `json:"is_meta"`
	Status       string `json:"status"`
}

func NewAgentCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Agentes de BattOS y sus runtimes",
		Long: `Administra identidades de agentes dentro de BattOS.

Un agente guarda identidad, rol, prompt base y runtime. Ejecutar un agente
requiere crear un run y aprobarlo; crear el agente no ejecuta nada.`,
	}
	cmd.AddCommand(newAgentListCmd(getClient), newAgentCreateCmd(getClient), newAgentShowCmd(getClient))
	return cmd
}

func newAgentListCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listar agentes",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []agentItem
			if err := workGet(cmd, getClient(), "/agents", &items); err != nil {
				return err
			}
			PrintBanner("AGENTS REGISTRY")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin agentes; crea uno con battos agent create)"))
				return nil
			}
			printWorkHeader("ID", "NAME", "RUNTIME", "STATUS", "ROLE")
			for _, item := range items {
				fmt.Printf("%s  %s  %s  %s  %s\n",
					styleOK.Render(item.ID),
					item.Name,
					styleSubtle.Render(emptyDash(item.RuntimeID)),
					styleSubtle.Render(item.Status),
					styleSubtle.Render(emptyDash(item.Role)),
				)
			}
			return nil
		},
	}
}

func newAgentCreateCmd(getClient func() *client.Client) *cobra.Command {
	var name, role, description, runtimeID, systemPrompt, riskLevel, status string
	cmd := &cobra.Command{
		Use:   "create <slug>",
		Short: "Crear un agente sin ejecutar nada",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" || runtimeID == "" {
				return fmt.Errorf("--name y --runtime son obligatorios")
			}
			body := map[string]any{
				"slug":          args[0],
				"name":          name,
				"role":          role,
				"description":   description,
				"runtime_id":    runtimeID,
				"system_prompt": systemPrompt,
				"risk_level":    riskLevel,
				"status":        status,
			}
			var item agentItem
			if err := workPost(cmd, getClient(), "/agents", body, &item); err != nil {
				return err
			}
			PrintBanner("AGENTS REGISTRY")
			fmt.Printf("%s Agente %s creado con runtime %s\n", styleOK.Render("OK"), item.ID, styleSubtle.Render(item.RuntimeID))
			fmt.Println(styleSubtle.Render("Nota: crear un agente no ejecuta nada; los runs requieren aprobacion separada."))
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "nombre visible del agente (obligatorio)")
	cmd.Flags().StringVar(&role, "role", "", "rol del agente, ej: web_builder")
	cmd.Flags().StringVar(&description, "description", "", "descripcion")
	cmd.Flags().StringVar(&runtimeID, "runtime", "", "runtime asociado, ej: codex o claude-code (obligatorio)")
	cmd.Flags().StringVar(&systemPrompt, "system-prompt", "", "prompt base del agente")
	cmd.Flags().StringVar(&riskLevel, "risk-level", "medium", "low|medium|high")
	cmd.Flags().StringVar(&status, "status", "active", "active|paused|archived")
	return cmd
}

func newAgentShowCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "show <agent_id>",
		Short: "Ver detalle de un agente",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []agentItem
			if err := workGet(cmd, getClient(), "/agents", &items); err != nil {
				return err
			}
			id := args[0]
			for _, item := range items {
				if item.ID != id && item.Slug != id {
					continue
				}
				PrintBanner("AGENTS REGISTRY")
				printAgentDetail(item)
				return nil
			}
			return fmt.Errorf("agente %q no encontrado", id)
		},
	}
}

func printAgentDetail(item agentItem) {
	printKV("ID", item.ID)
	printKV("Slug", item.Slug)
	printKV("Name", item.Name)
	printKV("Role", emptyDash(item.Role))
	printKV("Runtime", emptyDash(item.RuntimeID))
	printKV("Risk", emptyDash(item.RiskLevel))
	printKV("Status", item.Status)
	printKV("Lead", fmt.Sprintf("%t", item.IsLead))
	printKV("Meta", fmt.Sprintf("%t", item.IsMeta))
	printKV("Description", emptyDash(item.Description))
	printKV("System prompt", emptyDash(item.SystemPrompt))
}
