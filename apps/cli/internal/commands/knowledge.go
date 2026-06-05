package commands

import (
	"fmt"
	"net/url"

	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/spf13/cobra"
)

type knowledgeWorkspaceItem struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Layout    string `json:"layout"`
	Status    string `json:"status"`
}

type journalItem struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id"`
	Title       string `json:"title"`
	JournalDate string `json:"journal_date"`
}

type artifactItem struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	TaskID      string `json:"task_id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	ManagedPath string `json:"managed_path"`
	ExternalURL string `json:"external_url"`
}

func NewKnowledgeCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge",
		Short: "Knowledge Center: workspaces, journals y artifacts",
	}
	cmd.AddCommand(newKnowledgeWorkspaceCmd(getClient))
	cmd.AddCommand(newKnowledgeJournalCmd(getClient))
	cmd.AddCommand(newKnowledgeArtifactCmd(getClient))
	return cmd
}

func newKnowledgeWorkspaceCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "workspace", Short: "Workspaces de conocimiento por proyecto"}
	cmd.AddCommand(newKnowledgeWorkspaceListCmd(getClient), newKnowledgeWorkspaceCreateCmd(getClient))
	return cmd
}

func newKnowledgeJournalCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "journal", Short: "Journals del Knowledge Center"}
	cmd.AddCommand(newKnowledgeJournalListCmd(getClient), newKnowledgeJournalCreateCmd(getClient))
	return cmd
}

func newKnowledgeArtifactCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "artifact", Short: "Artifacts asociados a proyectos, tareas o runs"}
	cmd.AddCommand(newKnowledgeArtifactListCmd(getClient), newKnowledgeArtifactCreateCmd(getClient))
	return cmd
}

func newKnowledgeWorkspaceListCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listar workspaces activos",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []knowledgeWorkspaceItem
			if err := workGet(cmd, getClient(), "/knowledge/workspaces", &items); err != nil {
				return err
			}
			PrintBanner("KNOWLEDGE CENTER")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin workspaces)"))
				return nil
			}
			printWorkHeader("ID", "PROJECT", "NAME", "LAYOUT", "STATUS")
			for _, item := range items {
				fmt.Printf("%s  %s  %s  %s  %s\n", styleOK.Render(item.ID), styleSubtle.Render(item.ProjectID), item.Name, styleSubtle.Render(item.Layout), styleSubtle.Render(item.Status))
			}
			return nil
		},
	}
}

func newKnowledgeWorkspaceCreateCmd(getClient func() *client.Client) *cobra.Command {
	var projectID, name, layout, status string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Crear workspace de conocimiento",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" || name == "" {
				return fmt.Errorf("--project y --name son obligatorios")
			}
			var item knowledgeWorkspaceItem
			body := map[string]any{"project_id": projectID, "name": name, "layout": layout, "status": status}
			if err := workPost(cmd, getClient(), "/knowledge/workspaces", body, &item); err != nil {
				return err
			}
			PrintBanner("KNOWLEDGE CENTER")
			fmt.Printf("%s Workspace %s creado\n", styleOK.Render("OK"), item.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "proyecto asociado (obligatorio)")
	cmd.Flags().StringVar(&name, "name", "", "nombre del workspace (obligatorio)")
	cmd.Flags().StringVar(&layout, "layout", "raw_wiki_outputs", "layout del workspace")
	cmd.Flags().StringVar(&status, "status", "active", "active|archived")
	return cmd
}

func newKnowledgeJournalListCmd(getClient func() *client.Client) *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Listar journals por proyecto",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" {
				return fmt.Errorf("--project es obligatorio")
			}
			var items []journalItem
			path := "/journals?project_id=" + url.QueryEscape(projectID)
			if err := workGet(cmd, getClient(), path, &items); err != nil {
				return err
			}
			PrintBanner("KNOWLEDGE CENTER")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin journals)"))
				return nil
			}
			printWorkHeader("ID", "PROJECT", "DATE", "TITLE", "WORKSPACE")
			for _, item := range items {
				fmt.Printf("%s  %s  %s  %s  %s\n", styleOK.Render(item.ID), styleSubtle.Render(item.ProjectID), styleSubtle.Render(item.JournalDate), item.Title, styleSubtle.Render(item.WorkspaceID))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "proyecto asociado (obligatorio)")
	return cmd
}

func newKnowledgeJournalCreateCmd(getClient func() *client.Client) *cobra.Command {
	var workspaceID, projectID, title, content, date string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Crear journal",
		RunE: func(cmd *cobra.Command, args []string) error {
			if workspaceID == "" || title == "" || content == "" {
				return fmt.Errorf("--workspace, --title y --content son obligatorios")
			}
			var item journalItem
			body := map[string]any{"workspace_id": workspaceID, "project_id": projectID, "title": title, "content": content, "journal_date": date}
			if err := workPost(cmd, getClient(), "/journals", body, &item); err != nil {
				return err
			}
			PrintBanner("KNOWLEDGE CENTER")
			fmt.Printf("%s Journal %s creado\n", styleOK.Render("OK"), item.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "workspace UUID (obligatorio)")
	cmd.Flags().StringVar(&projectID, "project", "", "proyecto asociado; si se omite se infiere del workspace")
	cmd.Flags().StringVar(&title, "title", "", "titulo (obligatorio)")
	cmd.Flags().StringVar(&content, "content", "", "contenido markdown (obligatorio)")
	cmd.Flags().StringVar(&date, "date", "", "fecha YYYY-MM-DD; por defecto hoy")
	return cmd
}

func newKnowledgeArtifactListCmd(getClient func() *client.Client) *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Listar artifacts por proyecto",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" {
				return fmt.Errorf("--project es obligatorio")
			}
			var items []artifactItem
			path := "/artifacts?project_id=" + url.QueryEscape(projectID)
			if err := workGet(cmd, getClient(), path, &items); err != nil {
				return err
			}
			PrintBanner("KNOWLEDGE CENTER")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin artifacts)"))
				return nil
			}
			printWorkHeader("ID", "PROJECT", "TASK", "KIND", "NAME", "LOCATION")
			for _, item := range items {
				location := emptyDash(item.ManagedPath)
				if item.ExternalURL != "" {
					location = item.ExternalURL
				}
				fmt.Printf("%s  %s  %s  %s  %s  %s\n", styleOK.Render(item.ID), styleSubtle.Render(item.ProjectID), styleSubtle.Render(emptyDash(item.TaskID)), styleSubtle.Render(item.Kind), item.Name, styleSubtle.Render(location))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "proyecto asociado (obligatorio)")
	return cmd
}

func newKnowledgeArtifactCreateCmd(getClient func() *client.Client) *cobra.Command {
	var projectID, taskID, runID, name, kind, bucket, content, managedPath, externalURL string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Crear artifact",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" || name == "" || kind == "" {
				return fmt.Errorf("--project, --name y --kind son obligatorios")
			}
			if content == "" && managedPath == "" && externalURL == "" {
				return fmt.Errorf("debes indicar --content, --path o --url")
			}
			var item artifactItem
			body := map[string]any{
				"project_id":   projectID,
				"task_id":      taskID,
				"run_id":       runID,
				"name":         name,
				"kind":         kind,
				"bucket":       bucket,
				"content":      content,
				"managed_path": managedPath,
				"external_url": externalURL,
			}
			if err := workPost(cmd, getClient(), "/artifacts", body, &item); err != nil {
				return err
			}
			PrintBanner("KNOWLEDGE CENTER")
			fmt.Printf("%s Artifact %s creado\n", styleOK.Render("OK"), item.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "proyecto asociado (obligatorio)")
	cmd.Flags().StringVar(&taskID, "task", "", "tarea asociada")
	cmd.Flags().StringVar(&runID, "run", "", "run UUID asociado")
	cmd.Flags().StringVar(&name, "name", "", "nombre (obligatorio)")
	cmd.Flags().StringVar(&kind, "kind", "markdown", "markdown|image|link|diff|build_report")
	cmd.Flags().StringVar(&bucket, "bucket", "raw", "raw|wiki|outputs para artifacts gestionados")
	cmd.Flags().StringVar(&content, "content", "", "contenido inline")
	cmd.Flags().StringVar(&managedPath, "path", "", "ruta gestionada")
	cmd.Flags().StringVar(&externalURL, "url", "", "URL externa")
	return cmd
}
