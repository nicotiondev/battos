// memory.go — subcomandos `battos memory ...` para interactuar con el Memory Core.
package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/spf13/cobra"
)

// Estilo específico para el rank de los resultados de search.
var styleRank = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#8B5CF6")). // violeta
	Bold(true)

// NewMemoryCmd construye el árbol `battos memory ...`.
func NewMemoryCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Memory Core — guardar, buscar y consultar memoria persistente",
		Long: `Memory Core es la memoria persistente de BattOS (SQLite + FTS5 embebido).

Guarda observaciones tipadas (decisión, bugfix, pattern, ...) con metadata
de proyecto y agente, y permite búsqueda full-text con ranking BM25.`,
	}
	cmd.AddCommand(newMemoryRecentCmd(getClient))
	cmd.AddCommand(newMemorySearchCmd(getClient))
	cmd.AddCommand(newMemorySaveCmd(getClient))
	cmd.AddCommand(newMemoryStatsCmd(getClient))
	return cmd
}

// --- recent ---

func newMemoryRecentCmd(getClient func() *client.Client) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "recent",
		Short: "Últimas N observaciones",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			var resp struct {
				Items []memoryItem `json:"items"`
				Count int          `json:"count"`
			}
			c := getClient()
			if err := getJSON(ctx, c, fmt.Sprintf("/memory/recent?limit=%d", limit), &resp); err != nil {
				return err
			}
			if resp.Count == 0 {
				PrintBanner("MEMORY CORE")
				fmt.Println(styleSubtle.Render("(memoria vacía)"))
				return nil
			}
			PrintBanner("MEMORY CORE")
			fmt.Println(styleHeader.Render(fmt.Sprintf("Últimas %d observaciones", resp.Count)))
			fmt.Println()
			for _, it := range resp.Items {
				printMemoryItem(it, 0)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "número de observaciones a mostrar")
	return cmd
}

// --- search ---

func newMemorySearchCmd(getClient func() *client.Client) *cobra.Command {
	var (
		limit     int
		typeFlag  string
		projectID string
		agentID   string
		scope     string
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Búsqueda FTS5 con ranking BM25",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			query := strings.Join(args, " ")

			body := map[string]any{
				"query": query,
				"limit": limit,
				"filter": map[string]any{
					"type":       typeFlag,
					"project_id": projectID,
					"agent_id":   agentID,
					"scope":      scope,
				},
			}
			var resp struct {
				Results []memoryResult `json:"results"`
				Count   int            `json:"count"`
				Query   string         `json:"query"`
			}
			c := getClient()
			if err := postJSON(ctx, c, "/memory/search", body, &resp); err != nil {
				return err
			}
			PrintBanner("MEMORY CORE")
			fmt.Println(styleHeader.Render(fmt.Sprintf("%d resultados para %q", resp.Count, query)))
			fmt.Println()
			if resp.Count == 0 {
				fmt.Println(styleSubtle.Render("(sin coincidencias)"))
				return nil
			}
			for _, r := range resp.Results {
				printMemoryResult(r)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "máximo de resultados")
	cmd.Flags().StringVar(&typeFlag, "type", "", "filtrar por type (decision|bugfix|...)")
	cmd.Flags().StringVar(&projectID, "project", "", "filtrar por project_id")
	cmd.Flags().StringVar(&agentID, "agent", "", "filtrar por agent_id")
	cmd.Flags().StringVar(&scope, "scope", "", "filtrar por scope (project|personal)")
	return cmd
}

// --- save ---

func newMemorySaveCmd(getClient func() *client.Client) *cobra.Command {
	var (
		title     string
		content   string
		typeFlag  string
		topicKey  string
		projectID string
		agentID   string
		scope     string
	)
	cmd := &cobra.Command{
		Use:   "save",
		Short: "Guardar una observación nueva (o upsert por --topic-key)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title es obligatorio")
			}
			if scope == "" {
				scope = "project"
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			body := map[string]any{
				"title":      title,
				"content":    content,
				"type":       typeFlag,
				"topic_key":  topicKey,
				"project_id": projectID,
				"agent_id":   agentID,
				"scope":      scope,
			}
			var saved memoryItem
			c := getClient()
			if err := postJSON(ctx, c, "/memory/save", body, &saved); err != nil {
				return err
			}
			PrintBanner("MEMORY CORE")
			fmt.Println(styleOK.Render(fmt.Sprintf("✓ Observación guardada (id %d)", saved.ID)))
			printMemoryItem(saved, 0)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "título corto y searchable (obligatorio)")
	cmd.Flags().StringVar(&content, "content", "", "cuerpo markdown")
	cmd.Flags().StringVar(&typeFlag, "type", "manual", "decision|architecture|bugfix|pattern|discovery|learning|manual")
	cmd.Flags().StringVar(&topicKey, "topic-key", "", "clave para upsert (misma key reemplaza la observación previa)")
	cmd.Flags().StringVar(&projectID, "project", "", "project_id asociado")
	cmd.Flags().StringVar(&agentID, "agent", "", "agent_id asociado")
	cmd.Flags().StringVar(&scope, "scope", "project", "project|personal")
	return cmd
}

// --- stats ---

func newMemoryStatsCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Métricas agregadas del Memory Core",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			var s struct {
				TotalItems     int64     `json:"total_items"`
				ItemsLast24h   int64     `json:"items_last_24h"`
				UniqueProjects int64     `json:"unique_projects"`
				UniqueAgents   int64     `json:"unique_agents"`
				OldestItem     time.Time `json:"oldest_item"`
				NewestItem     time.Time `json:"newest_item"`
			}
			c := getClient()
			if err := getJSON(ctx, c, "/memory/stats", &s); err != nil {
				return err
			}
			PrintBanner("MEMORY CORE")
			fmt.Println(styleHeader.Render("Memory Core Stats"))
			fmt.Println()
			printKV("Total items", fmt.Sprintf("%d", s.TotalItems))
			printKV("Last 24h", fmt.Sprintf("%d", s.ItemsLast24h))
			printKV("Projects", fmt.Sprintf("%d", s.UniqueProjects))
			printKV("Agents", fmt.Sprintf("%d", s.UniqueAgents))
			if s.TotalItems > 0 {
				printKV("Oldest", s.OldestItem.Format(time.RFC3339))
				printKV("Newest", s.NewestItem.Format(time.RFC3339))
			}
			fmt.Println()
			return nil
		},
	}
}

// --- structs y helpers locales ---

type memoryItem struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	TopicKey  string    `json:"topic_key,omitempty"`
	ProjectID string    `json:"project_id,omitempty"`
	AgentID   string    `json:"agent_id,omitempty"`
	Scope     string    `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type memoryResult struct {
	memoryItem
	Rank float64 `json:"rank"`
}

func printMemoryItem(it memoryItem, rank float64) {
	header := fmt.Sprintf("[#%d] %s", it.ID, it.Title)
	fmt.Println(styleOK.Render("● ") + header)
	meta := fmt.Sprintf("  %s · %s", it.Type, it.CreatedAt.Format("2006-01-02 15:04"))
	if it.ProjectID != "" {
		meta += " · project:" + it.ProjectID
	}
	if it.AgentID != "" {
		meta += " · agent:" + it.AgentID
	}
	if it.TopicKey != "" {
		meta += " · topic:" + it.TopicKey
	}
	fmt.Println(styleSubtle.Render(meta))
	if it.Content != "" {
		preview := it.Content
		if len(preview) > 200 {
			preview = preview[:200] + "…"
		}
		// Indentar líneas del content.
		for _, line := range strings.Split(preview, "\n") {
			fmt.Println("    " + line)
		}
	}
	fmt.Println()
}

func printMemoryResult(r memoryResult) {
	fmt.Printf("%s %s\n",
		styleRank.Render(fmt.Sprintf("score %.2f", r.Rank)),
		r.Title)
	meta := fmt.Sprintf("  [#%d] %s · %s", r.ID, r.Type, r.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Println(styleSubtle.Render(meta))
	if r.Content != "" {
		preview := r.Content
		if len(preview) > 150 {
			preview = preview[:150] + "…"
		}
		fmt.Println("    " + strings.ReplaceAll(preview, "\n", " "))
	}
	fmt.Println()
}

// getJSON helper de cliente HTTP — usamos métodos directos en vez de extender client.Client
// para no tocar la capa client. En Fase 3 esto se reemplaza por oapi-codegen.
func getJSON(ctx context.Context, c *client.Client, path string, out any) error {
	resp, err := doRequest(ctx, c, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeOrError(resp, out)
}

func postJSON(ctx context.Context, c *client.Client, path string, body any, out any) error {
	resp, err := doRequest(ctx, c, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeOrError(resp, out)
}

func doRequest(ctx context.Context, c *client.Client, method, path string, body any) (*http.Response, error) {
	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	var req *http.Request
	var err error
	url := c.BaseURL() + path
	if bodyReader == nil {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, bodyReader)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	c.Authorize(req)
	httpClient := &http.Client{Timeout: 15 * time.Second}
	return httpClient.Do(req)
}

func decodeOrError(resp *http.Response, out any) error {
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
