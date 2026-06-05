package commands

import (
	"context"
	"fmt"
	"net/url"
	"strings"
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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DomainID    string `json:"domain_id"`
	Status      string `json:"status"`
}

type goalItem struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type taskItem struct {
	ID              string `json:"id"`
	ProjectID       string `json:"project_id"`
	GoalID          string `json:"goal_id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	AssignedAgentID string `json:"assigned_agent_id"`
	Status          string `json:"status"`
	BoardPosition   int32  `json:"board_position"`
}

func NewDomainCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "domain", Short: "Dominios del Work Board"}
	cmd.AddCommand(newDomainListCmd(getClient), newDomainCreateCmd(getClient))
	return cmd
}

func NewProjectCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "project", Short: "Proyectos del Work Board"}
	cmd.AddCommand(newProjectListCmd(getClient), newProjectCreateCmd(getClient), newProjectShowCmd(getClient))
	return cmd
}

func NewGoalCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "goal", Short: "Objetivos del Work Board"}
	cmd.AddCommand(newGoalListCmd(getClient), newGoalCreateCmd(getClient), newGoalShowCmd(getClient))
	return cmd
}

func NewTaskCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "task", Short: "Tareas del Work Board"}
	cmd.AddCommand(newTaskListCmd(getClient), newTaskBoardCmd(getClient), newTaskCreateCmd(getClient), newTaskShowCmd(getClient), newTaskMoveCmd(getClient), newTaskAssignCmd(getClient), newTaskLinkGoalCmd(getClient), newTaskPositionCmd(getClient))
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
			printWorkHeader("ID", "NAME", "STATUS")
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
			printWorkHeader("ID", "NAME", "STATUS", "DOMAIN")
			for _, item := range items {
				fmt.Printf("%s  %s  %s  %s\n", styleOK.Render(item.ID), item.Name, styleSubtle.Render(item.Status), styleSubtle.Render(emptyDash(item.DomainID)))
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

func newProjectShowCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "show <project_id>",
		Short: "Ver detalle de un proyecto",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var item projectItem
			if err := workGet(cmd, getClient(), "/projects/"+url.PathEscape(args[0]), &item); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			printKV("ID", item.ID)
			printKV("Name", item.Name)
			printKV("Status", item.Status)
			printKV("Domain", emptyDash(item.DomainID))
			printKV("Description", emptyDash(item.Description))
			return nil
		},
	}
}

func newGoalListCmd(getClient func() *client.Client) *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use: "list", Short: "Listar objetivos",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []goalItem
			path := "/goals"
			if projectID != "" {
				path += "?project_id=" + url.QueryEscape(projectID)
			}
			if err := workGet(cmd, getClient(), path, &items); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin objetivos)"))
				return nil
			}
			printWorkHeader("ID", "PROJECT", "TITLE", "STATUS")
			for _, item := range items {
				fmt.Printf("%s  %s  %s  %s\n", styleOK.Render(item.ID), styleSubtle.Render(item.ProjectID), item.Title, styleSubtle.Render(item.Status))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "filtrar por proyecto")
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

func newGoalShowCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "show <goal_id>",
		Short: "Ver detalle de un objetivo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var item goalItem
			if err := workGet(cmd, getClient(), "/goals/"+url.PathEscape(args[0]), &item); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			printKV("ID", item.ID)
			printKV("Project", item.ProjectID)
			printKV("Title", item.Title)
			printKV("Status", item.Status)
			printKV("Description", emptyDash(item.Description))
			return nil
		},
	}
}

func newTaskListCmd(getClient func() *client.Client) *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use: "list", Short: "Listar tareas",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []taskItem
			path := "/tasks"
			if projectID != "" {
				path += "?project_id=" + url.QueryEscape(projectID)
			}
			if err := workGet(cmd, getClient(), path, &items); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin tareas)"))
				return nil
			}
			printWorkHeader("ID", "PROJECT", "TITLE", "STATUS")
			for _, item := range items {
				fmt.Printf("%s  %s  %s  %s\n", styleOK.Render(item.ID), styleSubtle.Render(item.ProjectID), item.Title, styleSubtle.Render(item.Status))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "filtrar por proyecto")
	return cmd
}

func newTaskCreateCmd(getClient func() *client.Client) *cobra.Command {
	var projectID, goalID, title, description, status string
	cmd := &cobra.Command{
		Use: "create", Short: "Crear una tarea",
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title es obligatorio")
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
	cmd.Flags().StringVar(&projectID, "project", "", "proyecto asociado; si se omite usa inbox")
	cmd.Flags().StringVar(&goalID, "goal", "", "objetivo asociado")
	cmd.Flags().StringVar(&title, "title", "", "titulo (obligatorio)")
	cmd.Flags().StringVar(&description, "description", "", "descripcion")
	cmd.Flags().StringVar(&status, "status", "backlog", "backlog|ready|in_progress|review|done|cancelled")
	return cmd
}

func newTaskBoardCmd(getClient func() *client.Client) *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use:   "board",
		Short: "Ver tareas agrupadas como Kanban",
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []taskItem
			path := "/tasks"
			if projectID != "" {
				path += "?project_id=" + url.QueryEscape(projectID)
			}
			if err := workGet(cmd, getClient(), path, &items); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin tareas)"))
				return nil
			}
			printTaskBoard(items)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "filtrar por proyecto")
	return cmd
}

func newTaskShowCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "show <task_id>",
		Short: "Ver detalle de una tarea",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var item taskItem
			if err := workGet(cmd, getClient(), "/tasks/"+url.PathEscape(args[0]), &item); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			printTaskDetail(item)
			return nil
		},
	}
}

func newTaskMoveCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "move <task_id> <status>",
		Short: "Cambiar estado de una tarea",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var item taskItem
			if err := workPatch(cmd, getClient(), "/tasks/"+url.PathEscape(args[0]), map[string]any{"status": args[1]}, &item); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			fmt.Printf("%s Tarea %s movida a %s\n", styleOK.Render("OK"), item.ID, styleSubtle.Render(item.Status))
			return nil
		},
	}
}

func newTaskAssignCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "assign <task_id> <project_id>",
		Short: "Asignar una tarea a un proyecto",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var item taskItem
			if err := workPatch(cmd, getClient(), "/tasks/"+url.PathEscape(args[0]), map[string]any{"project_id": args[1]}, &item); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			fmt.Printf("%s Tarea %s asignada a %s\n", styleOK.Render("OK"), item.ID, styleSubtle.Render(item.ProjectID))
			return nil
		},
	}
}

func newTaskLinkGoalCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "link-goal <task_id> <goal_id>",
		Short: "Vincular una tarea a un objetivo del mismo proyecto",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var item taskItem
			if err := workPatch(cmd, getClient(), "/tasks/"+url.PathEscape(args[0]), map[string]any{"goal_id": args[1]}, &item); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			fmt.Printf("%s Tarea %s vinculada a goal %s\n", styleOK.Render("OK"), item.ID, styleSubtle.Render(item.GoalID))
			return nil
		},
	}
}

func newTaskPositionCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "position <task_id> <position>",
		Short: "Cambiar posicion de una tarea en el tablero",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var position int32
			if _, err := fmt.Sscanf(args[1], "%d", &position); err != nil {
				return fmt.Errorf("position debe ser un numero entero")
			}
			if position < 0 {
				return fmt.Errorf("position debe ser mayor o igual a 0")
			}
			var item taskItem
			if err := workPatch(cmd, getClient(), "/tasks/"+url.PathEscape(args[0]), map[string]any{"board_position": position}, &item); err != nil {
				return err
			}
			PrintBanner("WORK BOARD")
			fmt.Printf("%s Tarea %s movida a posicion %d\n", styleOK.Render("OK"), item.ID, item.BoardPosition)
			return nil
		},
	}
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

func workPatch(cmd *cobra.Command, c *client.Client, path string, body, out any) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()
	return patchJSON(ctx, c, path, body, out)
}

func printWorkHeader(columns ...string) {
	fmt.Println(styleSubtle.Render(strings.Join(columns, "  ")))
}

func printTaskDetail(item taskItem) {
	printKV("ID", item.ID)
	printKV("Project", item.ProjectID)
	printKV("Goal", emptyDash(item.GoalID))
	printKV("Title", item.Title)
	printKV("Status", item.Status)
	printKV("Assigned agent", emptyDash(item.AssignedAgentID))
	printKV("Board position", fmt.Sprintf("%d", item.BoardPosition))
	printKV("Description", emptyDash(item.Description))
}

func printTaskBoard(items []taskItem) {
	columns := []string{"backlog", "ready", "in_progress", "review", "done", "cancelled"}
	for _, status := range columns {
		fmt.Println(styleCommand.Render(strings.ToUpper(status)))
		count := 0
		for _, item := range items {
			if item.Status != status {
				continue
			}
			count++
			fmt.Printf("  %s  %s  %s\n", styleOK.Render(item.ID), item.Title, styleSubtle.Render(item.ProjectID))
		}
		if count == 0 {
			fmt.Println(styleSubtle.Render("  (sin tareas)"))
		}
	}
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
