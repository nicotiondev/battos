package commands

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/spf13/cobra"
)

type domainItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type projectItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	DomainID string `json:"domain_id"`
	Status   string `json:"status"`
}

type goalItem struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
}

type taskItem struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
}

func NewDomainCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "domain", Short: "Dominios del Work Board"}
	cmd.AddCommand(newDomainListCmd(getClient), newDomainCreateCmd(getClient))
	return cmd
}

func NewProjectCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "project", Short: "Proyectos del Work Board"}
	cmd.AddCommand(newProjectListCmd(getClient), newProjectCreateCmd(getClient))
	return cmd
}

func NewGoalCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "goal", Short: "Objetivos de un proyecto"}
	cmd.AddCommand(newGoalListCmd(getClient), newGoalCreateCmd(getClient))
	return cmd
}

func NewTaskCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "task", Short: "Tareas del Work Board"}
	cmd.AddCommand(newTaskListCmd(getClient), newTaskCreateCmd(getClient))
	return cmd
}

func newDomainListCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "Listar dominios activos",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []domainItem
			if err := workGet(cmd, getClient(), "/domains", &items); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin dominios)"))
				return nil
			}
			for _, item := range items {
				fmt.Printf("%s  %s  %s\n", styleOK.Render(item.ID), item.Name, styleSubtle.Render(item.Status))
			}
			return nil
		},
	}
}

func newDomainCreateCmd(getClient func() *client.Client) *cobra.Command {
	var name, description, status string
	cmd := &cobra.Command{
		Use: "create <slug>", Short: "Crear un dominio", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name es obligatorio")
			}
			var item domainItem
			if err := workPost(cmd, getClient(), "/domains", map[string]any{"slug": args[0], "name": name, "description": description, "status": status}, &item); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			fmt.Printf("%s Dominio %s creado\n", styleOK.Render("OK"), item.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "nombre del dominio (obligatorio)")
	cmd.Flags().StringVar(&description, "description", "", "descripcion")
	cmd.Flags().StringVar(&status, "status", "active", "active|paused|archived")
	return cmd
}

func newProjectListCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "Listar proyectos activos",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []projectItem
			if err := workGet(cmd, getClient(), "/projects", &items); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin proyectos)"))
				return nil
			}
			for _, item := range items {
				domain := ""
				if item.DomainID != "" {
					domain = " domain:" + item.DomainID
				}
				fmt.Printf("%s  %s  %s\n", styleOK.Render(item.ID), item.Name, styleSubtle.Render(item.Status+domain))
			}
			return nil
		},
	}
}

func newProjectCreateCmd(getClient func() *client.Client) *cobra.Command {
	var name, description, domainID, status string
	cmd := &cobra.Command{
		Use: "create <slug>", Short: "Crear un proyecto", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return fmt.Errorf("--name es obligatorio")
			}
			var item projectItem
			body := map[string]any{"slug": args[0], "name": name, "description": description, "domain_id": domainID, "status": status}
			if err := workPost(cmd, getClient(), "/projects", body, &item); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			fmt.Printf("%s Proyecto %s creado\n", styleOK.Render("OK"), item.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "nombre del proyecto (obligatorio)")
	cmd.Flags().StringVar(&description, "description", "", "descripcion")
	cmd.Flags().StringVar(&domainID, "domain", "", "dominio asociado")
	cmd.Flags().StringVar(&status, "status", "active", "active|paused|archived")
	return cmd
}

func newGoalListCmd(getClient func() *client.Client) *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use: "list", Short: "Listar objetivos del proyecto",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" {
				return fmt.Errorf("--project es obligatorio")
			}
			var items []goalItem
			if err := workGet(cmd, getClient(), "/goals?project_id="+url.QueryEscape(projectID), &items); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			for _, item := range items {
				fmt.Printf("%s  %s  %s\n", styleOK.Render(item.ID), item.Title, styleSubtle.Render(item.Status))
			}
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin objetivos)"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "proyecto asociado (obligatorio)")
	return cmd
}

func newGoalCreateCmd(getClient func() *client.Client) *cobra.Command {
	var projectID, title, description, status string
	cmd := &cobra.Command{
		Use: "create", Short: "Crear un objetivo",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" || title == "" {
				return fmt.Errorf("--project y --title son obligatorios")
			}
			var item goalItem
			body := map[string]any{"project_id": projectID, "title": title, "description": description, "status": status}
			if err := workPost(cmd, getClient(), "/goals", body, &item); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			fmt.Printf("%s Objetivo %s creado\n", styleOK.Render("OK"), item.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "proyecto asociado (obligatorio)")
	cmd.Flags().StringVar(&title, "title", "", "titulo (obligatorio)")
	cmd.Flags().StringVar(&description, "description", "", "descripcion")
	cmd.Flags().StringVar(&status, "status", "planned", "planned|active|completed|cancelled")
	return cmd
}

func newTaskListCmd(getClient func() *client.Client) *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use: "list", Short: "Listar tareas del proyecto",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" {
				return fmt.Errorf("--project es obligatorio")
			}
			var items []taskItem
			if err := workGet(cmd, getClient(), "/tasks?project_id="+url.QueryEscape(projectID), &items); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			for _, item := range items {
				fmt.Printf("%s  %s  %s\n", styleOK.Render(item.ID), item.Title, styleSubtle.Render(item.Status))
			}
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin tareas)"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "proyecto asociado (obligatorio)")
	return cmd
}

func newTaskCreateCmd(getClient func() *client.Client) *cobra.Command {
	var projectID, goalID, title, description, status string
	cmd := &cobra.Command{
		Use: "create", Short: "Crear una tarea",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" || title == "" {
				return fmt.Errorf("--project y --title son obligatorios")
			}
			var item taskItem
			body := map[string]any{"project_id": projectID, "goal_id": goalID, "title": title, "description": description, "status": status}
			if err := workPost(cmd, getClient(), "/tasks", body, &item); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			fmt.Printf("%s Tarea %s creada\n", styleOK.Render("OK"), item.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "proyecto asociado (obligatorio)")
	cmd.Flags().StringVar(&goalID, "goal", "", "objetivo asociado")
	cmd.Flags().StringVar(&title, "title", "", "titulo (obligatorio)")
	cmd.Flags().StringVar(&description, "description", "", "descripcion")
	cmd.Flags().StringVar(&status, "status", "backlog", "backlog|ready|in_progress|review|done|cancelled")
	return cmd
}

func workGet(cmd *cobra.Command, c *client.Client, path string, out any) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()
	return getJSON(ctx, c, path, out)
}

func workPost(cmd *cobra.Command, c *client.Client, path string, body, out any) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()
	return postJSON(ctx, c, path, body, out)
}
