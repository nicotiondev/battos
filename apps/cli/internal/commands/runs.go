package commands

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/spf13/cobra"
)

type runItem struct {
	ID               string  `json:"id"`
	ProjectID        string  `json:"project_id"`
	TaskID           string  `json:"task_id"`
	AgentID          string  `json:"agent_id"`
	SkillID          string  `json:"skill_id"`
	RuntimeAdapterID string  `json:"runtime_adapter_id"`
	RepositoryID     string  `json:"repository_id"`
	Prompt           string  `json:"prompt"`
	RequestedNetwork bool    `json:"requested_network"`
	NetworkEnabled   bool    `json:"network_enabled"`
	Status           string  `json:"status"`
	BranchName       string  `json:"branch_name"`
	ResultSummary    string  `json:"result_summary"`
	ErrorMessage     string  `json:"error_message"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

type runApprovalResult struct {
	Run      runItem `json:"run"`
	Approval struct {
		ID       string `json:"id"`
		Kind     string `json:"kind"`
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	} `json:"approval"`
}

type runLogItem struct {
	ID      int64  `json:"id"`
	Stream  string `json:"stream"`
	Message string `json:"message"`
}

func NewRunCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{Use: "run", Short: "Runs supervisados y approvals"}
	cmd.AddCommand(newRunListCmd(getClient), newRunProposeCmd(getClient), newRunShowCmd(getClient), newRunApproveCmd(getClient), newRunCancelCmd(getClient), newRunLogsCmd(getClient), newRunRememberCmd(getClient))
	return cmd
}

func newRunListCmd(getClient func() *client.Client) *cobra.Command {
	var projectID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Listar runs supervisados",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/runs"
			if strings.TrimSpace(projectID) != "" {
				path += "?project_id=" + url.QueryEscape(projectID)
			}
			var items []runItem
			if err := workGet(cmd, getClient(), path, &items); err != nil {
				return err
			}
			PrintBanner("RUN CONTROL")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin runs)"))
				return nil
			}
			printWorkHeader("ID", "PROJECT", "TASK", "RUNTIME", "STATUS")
			for _, item := range items {
				fmt.Printf("%s  %s  %s  %s  %s\n", styleOK.Render(item.ID), styleSubtle.Render(item.ProjectID), styleSubtle.Render(item.TaskID), styleSubtle.Render(item.RuntimeAdapterID), item.Status)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "filtrar por proyecto")
	return cmd
}

func newRunProposeCmd(getClient func() *client.Client) *cobra.Command {
	var projectID, taskID, agentID, skillID, runtimeID, repositoryID, prompt string
	var network bool
	cmd := &cobra.Command{
		Use:   "propose",
		Short: "Proponer un run; queda esperando approval",
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectID == "" || taskID == "" || agentID == "" || runtimeID == "" || prompt == "" {
				return fmt.Errorf("--project, --task, --agent, --runtime y --prompt son obligatorios")
			}
			body := map[string]any{
				"project_id":         projectID,
				"task_id":            taskID,
				"agent_id":           agentID,
				"skill_id":           skillID,
				"runtime_adapter_id": runtimeID,
				"repository_id":      repositoryID,
				"prompt":             prompt,
				"requested_network":  network,
			}
			var item runItem
			if err := workPost(cmd, getClient(), "/runs", body, &item); err != nil {
				return err
			}
			PrintBanner("RUN CONTROL")
			fmt.Printf("%s Run %s propuesto en estado %s\n", styleOK.Render("OK"), item.ID, styleSubtle.Render(item.Status))
			fmt.Println(styleSubtle.Render("Nota: este comando no ejecuta nada; usa run approve --kind execute para encolarlo."))
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "proyecto asociado (obligatorio)")
	cmd.Flags().StringVar(&taskID, "task", "", "tarea asociada (obligatorio)")
	cmd.Flags().StringVar(&agentID, "agent", "", "agente asociado (obligatorio)")
	cmd.Flags().StringVar(&skillID, "skill", "", "skill opcional")
	cmd.Flags().StringVar(&runtimeID, "runtime", "", "runtime adapter: claude-code|codex (obligatorio)")
	cmd.Flags().StringVar(&repositoryID, "repo", "", "repositorio opcional")
	cmd.Flags().StringVar(&prompt, "prompt", "", "instruccion del run (obligatorio)")
	cmd.Flags().BoolVar(&network, "network", false, "solicitar permiso de red para el run")
	return cmd
}

func newRunShowCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "show <run_id>",
		Short: "Ver detalle de un run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var item runItem
			if err := workGet(cmd, getClient(), "/runs/"+url.PathEscape(args[0]), &item); err != nil {
				return err
			}
			PrintBanner("RUN CONTROL")
			printRunDetail(item)
			return nil
		},
	}
}

func newRunApproveCmd(getClient func() *client.Client) *cobra.Command {
	var kind, decision, reason string
	cmd := &cobra.Command{
		Use:   "approve <run_id>",
		Short: "Registrar approval de execute/network/commit/push",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"kind": kind, "decision": decision, "reason": reason}
			var result runApprovalResult
			if err := workPost(cmd, getClient(), "/runs/"+url.PathEscape(args[0])+"/approvals", body, &result); err != nil {
				return err
			}
			PrintBanner("RUN CONTROL")
			fmt.Printf("%s Approval %s/%s registrado; run ahora esta %s\n", styleOK.Render("OK"), result.Approval.Kind, result.Approval.Decision, styleSubtle.Render(result.Run.Status))
			if result.Approval.Kind == "execute" && result.Approval.Decision == "approved" {
				fmt.Println(styleSubtle.Render("Nota: queda queued; el worker aislado lo procesara si esta corriendo."))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "execute", "execute|network|commit|push")
	cmd.Flags().StringVar(&decision, "decision", "approved", "approved|rejected")
	cmd.Flags().StringVar(&reason, "reason", "", "motivo de la decision")
	return cmd
}

func newRunCancelCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <run_id>",
		Short: "Cancelar un run no terminal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var item runItem
			if err := workPost(cmd, getClient(), "/runs/"+url.PathEscape(args[0])+"/cancel", map[string]any{}, &item); err != nil {
				return err
			}
			PrintBanner("RUN CONTROL")
			fmt.Printf("%s Run %s cancelado\n", styleOK.Render("OK"), item.ID)
			return nil
		},
	}
}

func newRunLogsCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "logs <run_id>",
		Short: "Ver logs persistidos de un run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var items []runLogItem
			if err := workGet(cmd, getClient(), "/runs/"+url.PathEscape(args[0])+"/logs", &items); err != nil {
				return err
			}
			PrintBanner("RUN CONTROL")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin logs todavia)"))
				return nil
			}
			for _, item := range items {
				fmt.Printf("%s  %s\n", styleSubtle.Render(item.Stream), item.Message)
			}
			return nil
		},
	}
}

func newRunRememberCmd(getClient func() *client.Client) *cobra.Command {
	var (
		title            string
		typeFlag         string
		topicKey         string
		scope            string
		includeLogs      bool
		includePrompt    bool
		allowNonTerminal bool
		logLimit         int
	)
	cmd := &cobra.Command{
		Use:   "remember <run_id>",
		Short: "Guardar un resumen aprobado del run en Memory Core",
		Long: `Guarda un resumen del run como memoria persistente de BattOS.

Es una accion explicita: BattOS no guarda automaticamente el aprendizaje hasta
que el usuario ejecuta este comando. Por defecto no guarda el prompt completo;
usa --include-prompt si quieres incluirlo.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]
			var item runItem
			if err := workGet(cmd, getClient(), "/runs/"+url.PathEscape(runID), &item); err != nil {
				return err
			}
			if !allowNonTerminal && !isTerminalRunStatus(item.Status) {
				return fmt.Errorf("solo se recuerdan runs terminales; estado actual: %s (usa --allow-non-terminal si realmente quieres guardarlo)", item.Status)
			}
			var logs []runLogItem
			if includeLogs {
				if err := workGet(cmd, getClient(), "/runs/"+url.PathEscape(runID)+"/logs", &logs); err != nil {
					return err
				}
			}
			if title == "" {
				title = fmt.Sprintf("Run %s %s", shortID(item.ID), item.Status)
			}
			if typeFlag == "" {
				typeFlag = "learning"
			}
			if scope == "" {
				scope = "project"
			}
			if topicKey == "" {
				topicKey = fmt.Sprintf("%s/runs/%s/summary", item.ProjectID, item.ID)
			}
			content := renderRunMemorySummary(item, logs, runMemorySummaryOptions{
				IncludeLogs:   includeLogs,
				IncludePrompt: includePrompt,
				LogLimit:      logLimit,
			})
			body := map[string]any{
				"title":      title,
				"content":    content,
				"type":       typeFlag,
				"topic_key":  topicKey,
				"project_id": item.ProjectID,
				"agent_id":   item.AgentID,
				"scope":      scope,
			}
			var saved memoryItem
			if err := workPost(cmd, getClient(), "/memory/save", body, &saved); err != nil {
				return err
			}
			PrintBanner("MEMORY CORE")
			fmt.Printf("%s Run %s guardado como memoria #%d\n", styleOK.Render("OK"), styleSubtle.Render(item.ID), saved.ID)
			fmt.Println(styleSubtle.Render("topic: " + saved.TopicKey))
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "titulo de la memoria")
	cmd.Flags().StringVar(&typeFlag, "type", "learning", "learning|decision|bugfix|pattern|discovery|manual")
	cmd.Flags().StringVar(&topicKey, "topic-key", "", "clave de upsert; default <project>/runs/<run>/summary")
	cmd.Flags().StringVar(&scope, "scope", "project", "project|personal")
	cmd.Flags().BoolVar(&includeLogs, "include-logs", true, "incluir logs del run")
	cmd.Flags().BoolVar(&includePrompt, "include-prompt", false, "incluir prompt original completo")
	cmd.Flags().BoolVar(&allowNonTerminal, "allow-non-terminal", false, "permitir recordar runs no terminales")
	cmd.Flags().IntVar(&logLimit, "log-limit", 20, "maximo de logs a incluir")
	return cmd
}

func printRunDetail(item runItem) {
	printKV("ID", item.ID)
	printKV("Project", item.ProjectID)
	printKV("Task", item.TaskID)
	printKV("Agent", item.AgentID)
	printKV("Runtime", item.RuntimeAdapterID)
	printKV("Skill", emptyDash(item.SkillID))
	printKV("Repository", emptyDash(item.RepositoryID))
	printKV("Status", item.Status)
	printKV("Network requested", fmt.Sprintf("%t", item.RequestedNetwork))
	printKV("Network enabled", fmt.Sprintf("%t", item.NetworkEnabled))
	printKV("Prompt", item.Prompt)
	printKV("Result", emptyDash(item.ResultSummary))
}

type runMemorySummaryOptions struct {
	IncludeLogs   bool
	IncludePrompt bool
	LogLimit      int
}

func renderRunMemorySummary(item runItem, logs []runLogItem, opts runMemorySummaryOptions) string {
	var b strings.Builder
	b.WriteString("# BattOS Run Summary\n\n")
	b.WriteString(fmt.Sprintf("- Run: %s\n", item.ID))
	b.WriteString(fmt.Sprintf("- Project: %s\n", item.ProjectID))
	b.WriteString(fmt.Sprintf("- Task: %s\n", item.TaskID))
	b.WriteString(fmt.Sprintf("- Agent: %s\n", item.AgentID))
	b.WriteString(fmt.Sprintf("- Runtime: %s\n", item.RuntimeAdapterID))
	b.WriteString(fmt.Sprintf("- Status: %s\n", item.Status))
	if item.ResultSummary != "" {
		b.WriteString(fmt.Sprintf("- Result: %s\n", item.ResultSummary))
	}
	if item.ErrorMessage != "" {
		b.WriteString(fmt.Sprintf("- Error: %s\n", item.ErrorMessage))
	}
	if opts.IncludePrompt && item.Prompt != "" {
		b.WriteString("\n## Prompt\n\n")
		b.WriteString(strings.TrimSpace(item.Prompt))
		b.WriteString("\n")
	}
	if opts.IncludeLogs && len(logs) > 0 {
		b.WriteString("\n## Logs\n\n")
		for _, log := range tailRunLogs(logs, opts.LogLimit) {
			b.WriteString(fmt.Sprintf("- `%s` %s\n", log.Stream, strings.TrimSpace(log.Message)))
		}
	}
	return b.String()
}

func tailRunLogs(logs []runLogItem, limit int) []runLogItem {
	if limit <= 0 || limit >= len(logs) {
		return logs
	}
	return logs[len(logs)-limit:]
}

func isTerminalRunStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func shortID(value string) string {
	if len(value) <= 8 {
		return value
	}
	return value[:8]
}
