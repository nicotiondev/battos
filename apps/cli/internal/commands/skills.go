package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/spf13/cobra"
)

type skillItem struct {
	ID             string    `json:"id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Category       string    `json:"category"`
	RiskLevel      string    `json:"risk_level"`
	Version        string    `json:"version"`
	Status         string    `json:"status"`
	Lifecycle      string    `json:"lifecycle"`
	PromptTemplate string    `json:"prompt_template"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// NewSkillCmd builds the top-level `battos skills` command and its subcommands.
func NewSkillCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Skills del registry de BattOS",
		Long: `Administra el registry de skills de BattOS.

Un skill es un prompt template con metadatos que puede asignarse a un run.
Usá 'skills list' para ver los skills disponibles y 'skills ingest' para
cargar un SKILL.md local al registry.`,
	}
	cmd.AddCommand(newSkillListCmd(getClient), newSkillIngestCmd(getClient))
	return cmd
}

func newSkillListCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listar skills del registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []skillItem
			if err := workGet(cmd, getClient(), "/skills", &items); err != nil {
				return err
			}
			PrintBanner("SKILLS REGISTRY")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin skills; ingresá uno con battos skills ingest <path>)"))
				return nil
			}
			printWorkHeader("ID", "NAME", "VERSION", "STATUS")
			for _, item := range items {
				fmt.Printf("%s  %s  %s  %s\n",
					styleOK.Render(item.ID),
					item.Name,
					styleSubtle.Render(emptyDash(item.Version)),
					styleSubtle.Render(item.Status),
				)
			}
			return nil
		},
	}
}

func newSkillIngestCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "ingest <path>",
		Short: "Cargar un SKILL.md al registry (upsert por nombre)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("leyendo archivo %q: %w", args[0], err)
			}
			body := map[string]any{
				"content": string(raw),
			}
			var item skillItem
			if err := workPost(cmd, getClient(), "/skills/ingest", body, &item); err != nil {
				return err
			}
			PrintBanner("SKILLS REGISTRY")
			fmt.Printf("%s Skill %s guardado (id: %s, version: %s)\n",
				styleOK.Render("OK"),
				item.Name,
				styleSubtle.Render(item.ID),
				styleSubtle.Render(emptyDash(item.Version)),
			)
			return nil
		},
	}
}
