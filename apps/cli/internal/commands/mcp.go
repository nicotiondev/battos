// mcp.go — subcomando `battos mcp` que arranca un servidor MCP sobre stdio.
//
// El servidor es un proxy delgado al API HTTP de BattOS (/memory/*).
// No abre el SQLite directamente — todo pasa por client.Client, igual que el resto del CLI.
// Esto permite usarlo tanto en local como apuntando a un nodo remoto con --api.
//
// Herramientas expuestas:
//   - memory_search   → POST /memory/search
//   - memory_recent   → GET  /memory/recent
//   - memory_save     → POST /memory/save
//   - memory_stats    → GET  /memory/stats
package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/spf13/cobra"
)

// NewMCPCmd construye el subcomando `battos mcp`.
// getAPIURL y getToken son funciones que devuelven los valores resueltos de los
// flags persistentes --api y --token (evaluadas en tiempo de ejecución).
func NewMCPCmd(getClient func() *client.Client, getAPIURL func() string, getToken func() string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Servidor MCP sobre stdio — expone el Memory Core a agentes externos",
		Long: `Arranca un servidor MCP (Model Context Protocol) sobre stdin/stdout.

Úsalo como MCP server en Claude Code, Codex u otro agente compatible,
o registra el servidor automáticamente con:

  battos mcp install

El servidor es un proxy al API HTTP de BattOS y requiere que éste esté
corriendo (docker compose up -d).  Respeta --api y --token (o las
variables de entorno BATTOS_API_URL / BATTOS_API_TOKEN).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPServer(cmd.Context(), getClient())
		},
	}

	cmd.AddCommand(newMCPInstallCmd(getAPIURL, getToken))
	return cmd
}

// runMCPServer crea y arranca el servidor MCP sobre stdio.
// Bloquea hasta que el peer cierra la conexión o el contexto se cancela.
func runMCPServer(ctx context.Context, c *client.Client) error {
	return newMCPServer(c).Run(ctx, &mcp.StdioTransport{})
}

// newMCPServer construye el servidor MCP con sus 4 tools registradas.
// mcp.AddTool entra en panic si el JSON Schema inferido de un arg struct es
// inválido (p. ej. un tag jsonschema malformado), así que construir el servidor
// es la verificación temprana del arranque — cubierta por TestNewMCPServer.
func newMCPServer(c *client.Client) *mcp.Server {
	srv := mcp.NewServer(
		&mcp.Implementation{
			Name:    "battos",
			Version: "v0.1.0",
		},
		nil,
	)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "memory_search",
		Description: "Búsqueda FTS5 BM25 en el Memory Core de BattOS. Devuelve observaciones con score de relevancia.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args memorySearchArgs) (*mcp.CallToolResult, any, error) {
		return memorySearchToolHandler(ctx, c, args)
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "memory_recent",
		Description: "Devuelve las N observaciones más recientes del Memory Core.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args memoryRecentArgs) (*mcp.CallToolResult, any, error) {
		return memoryRecentToolHandler(ctx, c, args)
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "memory_save",
		Description: "Guarda (o actualiza vía topic_key) una observación en el Memory Core de BattOS.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args memorySaveArgs) (*mcp.CallToolResult, any, error) {
		return memorySaveToolHandler(ctx, c, args)
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "memory_stats",
		Description: "Métricas agregadas del Memory Core (total items, últimas 24h, proyectos, agentes).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ memoryStatsArgs) (*mcp.CallToolResult, any, error) {
		return memoryStatsToolHandler(ctx, c)
	})

	// --- Team tools (Fase B): inter-comunicación multi-agente vía mailbox ---
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "team_send_message",
		Description: "Envía un mensaje al inbox de otro agente (inter-comunicación multi-agente). Úsalo para delegar trabajo, coordinar o reportar a un lead.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args teamSendMessageArgs) (*mcp.CallToolResult, any, error) {
		return teamSendMessageToolHandler(ctx, c, args)
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "team_read_inbox",
		Description: "Lee los mensajes dirigidos a un agente. unread_only=true devuelve solo los no leídos.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args teamReadInboxArgs) (*mcp.CallToolResult, any, error) {
		return teamReadInboxToolHandler(ctx, c, args)
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "team_mark_read",
		Description: "Marca un mensaje del inbox como leído (para no reprocesarlo).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args teamMarkReadArgs) (*mcp.CallToolResult, any, error) {
		return teamMarkReadToolHandler(ctx, c, args)
	})

	return srv
}

// --- tipos de argumentos para cada tool ---
// El SDK infiere el JSON Schema de los campos del struct.

// El SDK (google/jsonschema-go) toma el valor COMPLETO del tag `jsonschema`
// como descripción del campo; no admite directivas tipo `required,` ni
// `description=`. La obligatoriedad se deriva del tag `json`: un campo SIN
// `omitempty` queda en `required`, con `omitempty` queda opcional.
type memorySearchArgs struct {
	Query     string `json:"query"                jsonschema:"Texto de búsqueda FTS5"`
	Type      string `json:"type,omitempty"       jsonschema:"Filtrar por type: decision|architecture|bugfix|pattern|discovery|learning|manual"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"Filtrar por project_id"`
	AgentID   string `json:"agent_id,omitempty"   jsonschema:"Filtrar por agent_id"`
	Scope     string `json:"scope,omitempty"      jsonschema:"Filtrar por scope: project|personal"`
	Limit     int    `json:"limit,omitempty"      jsonschema:"Máximo de resultados (0 = 10 por defecto)"`
}

type memoryRecentArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"Número de observaciones a devolver (0 = 20 por defecto)"`
}

type memorySaveArgs struct {
	Title     string `json:"title"                jsonschema:"Título corto y searchable"`
	Content   string `json:"content"              jsonschema:"Cuerpo markdown de la observación"`
	Type      string `json:"type,omitempty"       jsonschema:"Tipo: decision|architecture|bugfix|pattern|discovery|learning|manual (default: manual)"`
	TopicKey  string `json:"topic_key,omitempty"  jsonschema:"Clave para upsert — misma key reemplaza la observación previa"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"project_id asociado"`
	AgentID   string `json:"agent_id,omitempty"   jsonschema:"agent_id asociado"`
	Scope     string `json:"scope,omitempty"      jsonschema:"Scope: project|personal (default: project)"`
}

// memoryStatsArgs es un struct vacío — memory_stats no tiene parámetros.
// El SDK requiere que In sea un map o struct para el JSON Schema "object".
type memoryStatsArgs struct{}

type teamSendMessageArgs struct {
	ToAgentID   string `json:"to_agent_id"             jsonschema:"Agente destino — su inbox recibe el mensaje"`
	Body        string `json:"body"                    jsonschema:"Cuerpo del mensaje"`
	Subject     string `json:"subject,omitempty"       jsonschema:"Asunto opcional"`
	FromAgentID string `json:"from_agent_id,omitempty" jsonschema:"Agente emisor (opcional)"`
	ProjectID   string `json:"project_id,omitempty"    jsonschema:"project_id asociado (opcional)"`
	RunID       string `json:"run_id,omitempty"        jsonschema:"run_id que origina el mensaje (opcional)"`
}

type teamReadInboxArgs struct {
	AgentID    string `json:"agent_id"              jsonschema:"Agente cuyo inbox leer"`
	UnreadOnly bool   `json:"unread_only,omitempty" jsonschema:"Solo no leídos (default false)"`
	Limit      int    `json:"limit,omitempty"       jsonschema:"Máximo de mensajes (0 = 50)"`
}

type teamMarkReadArgs struct {
	MessageID string `json:"message_id" jsonschema:"ID del mensaje a marcar leído"`
}

// --- handlers de cada tool ---
// Estos son funciones puras (reciben c *client.Client) para poder testarlas
// sin levantar el servidor MCP completo.

func memorySearchToolHandler(ctx context.Context, c *client.Client, args memorySearchArgs) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}
	req := client.MemorySearchRequest{
		Query: args.Query,
		Filter: client.MemorySearchFilter{
			Type:      args.Type,
			ProjectID: args.ProjectID,
			AgentID:   args.AgentID,
			Scope:     args.Scope,
		},
		Limit: limit,
	}
	resp, err := c.MemorySearch(ctx, req)
	if err != nil {
		return toolError(fmt.Sprintf("memory_search: %s", err.Error())), nil, nil
	}
	return toolJSON(resp)
}

func memoryRecentToolHandler(ctx context.Context, c *client.Client, args memoryRecentArgs) (*mcp.CallToolResult, any, error) {
	limit := args.Limit
	if limit <= 0 {
		limit = 20
	}
	resp, err := c.MemoryRecent(ctx, limit)
	if err != nil {
		return toolError(fmt.Sprintf("memory_recent: %s", err.Error())), nil, nil
	}
	return toolJSON(resp)
}

func memorySaveToolHandler(ctx context.Context, c *client.Client, args memorySaveArgs) (*mcp.CallToolResult, any, error) {
	t := args.Type
	if t == "" {
		t = "manual"
	}
	scope := args.Scope
	if scope == "" {
		scope = "project"
	}
	req := client.MemorySaveRequest{
		Title:     args.Title,
		Content:   args.Content,
		Type:      t,
		TopicKey:  args.TopicKey,
		ProjectID: args.ProjectID,
		AgentID:   args.AgentID,
		Scope:     scope,
	}
	saved, err := c.MemorySave(ctx, req)
	if err != nil {
		return toolError(fmt.Sprintf("memory_save: %s", err.Error())), nil, nil
	}
	return toolJSON(saved)
}

func memoryStatsToolHandler(ctx context.Context, c *client.Client) (*mcp.CallToolResult, any, error) {
	stats, err := c.MemoryStats(ctx)
	if err != nil {
		return toolError(fmt.Sprintf("memory_stats: %s", err.Error())), nil, nil
	}
	return toolJSON(stats)
}

func teamSendMessageToolHandler(ctx context.Context, c *client.Client, args teamSendMessageArgs) (*mcp.CallToolResult, any, error) {
	msg, err := c.SendAgentMessage(ctx, client.SendAgentMessageRequest{
		ToAgentID:   args.ToAgentID,
		Body:        args.Body,
		Subject:     args.Subject,
		FromAgentID: args.FromAgentID,
		ProjectID:   args.ProjectID,
		RunID:       args.RunID,
	})
	if err != nil {
		return toolError(fmt.Sprintf("team_send_message: %s", err.Error())), nil, nil
	}
	return toolJSON(msg)
}

func teamReadInboxToolHandler(ctx context.Context, c *client.Client, args teamReadInboxArgs) (*mcp.CallToolResult, any, error) {
	msgs, err := c.ListAgentInbox(ctx, args.AgentID, args.UnreadOnly, args.Limit)
	if err != nil {
		return toolError(fmt.Sprintf("team_read_inbox: %s", err.Error())), nil, nil
	}
	return toolJSON(msgs)
}

func teamMarkReadToolHandler(ctx context.Context, c *client.Client, args teamMarkReadArgs) (*mcp.CallToolResult, any, error) {
	msg, err := c.MarkAgentMessageRead(ctx, args.MessageID)
	if err != nil {
		return toolError(fmt.Sprintf("team_mark_read: %s", err.Error())), nil, nil
	}
	return toolJSON(msg)
}

// --- utilidades de respuesta ---

// toolJSON serializa v como JSON pretty-printed y lo devuelve como TextContent.
func toolJSON(v any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return toolError(fmt.Sprintf("encoding response: %s", err.Error())), nil, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(b)},
		},
	}, nil, nil
}

// toolError devuelve un CallToolResult con IsError=true.
// NO retorna un error de protocolo — el agente puede ver el mensaje y auto-corregir.
func toolError(msg string) *mcp.CallToolResult {
	res := &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}
	return res
}
