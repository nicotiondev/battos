package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/spf13/cobra"
)

type novaChatResponse struct {
	ConversationID string `json:"conversation_id"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	TokensIn       int    `json:"tokens_in"`
	TokensOut      int    `json:"tokens_out"`
}

func NewAskCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "ask <pregunta>",
		Short: "Preguntar algo a NovaCore (Asistente del OS)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := args[0]
			body := map[string]any{
				"content": prompt,
			}
			var resp novaChatResponse
			err := workPost(cmd, getClient(), "/novacore/chat", body, &resp)
			if err != nil {
				return err
			}
			PrintBanner("NOVACORE")
			fmt.Println(resp.Content)
			fmt.Println()
			fmt.Printf(styleSubtle.Render("Costo del turno: %d tokens in / %d tokens out\n"), resp.TokensIn, resp.TokensOut)
			return nil
		},
	}
}

func NewChatCmd(getClient func() *client.Client) *cobra.Command {
	var conversationID string
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Chatear de forma interactiva con NovaCore",
		Long:  `Inicia una conversacion interactiva y continua con NovaCore (System Assistant).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			PrintBanner("NOVACORE INTERACTIVE CHAT")
			fmt.Println("🤖 ¡Hola! Soy NovaCore, tu asistente de sistema de BattOS.")
			fmt.Println("Puedes preguntarme sobre proyectos, tareas, agentes o diagnosticar el estado del OS.")
			fmt.Println("Escribe 'salir' o 'exit' para terminar la sesion.")
			fmt.Println()

			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Print("👤 Tú > ")
				input, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				input = strings.TrimSpace(input)
				if input == "" {
					continue
				}
				if input == "salir" || input == "exit" {
					fmt.Println("🤖 ¡Hasta luego!")
					break
				}

				body := map[string]any{
					"content": input,
				}
				if conversationID != "" {
					body["conversation_id"] = conversationID
				}

				var resp novaChatResponse
				errPost := workPost(cmd, getClient(), "/novacore/chat", body, &resp)
				if errPost != nil {
					fmt.Printf("%s Error al enviar mensaje: %v\n", styleDown.Render("ERR"), errPost)
					continue
				}

				conversationID = resp.ConversationID
				fmt.Println()
				fmt.Printf("🤖 NovaCore > %s\n", resp.Content)
				fmt.Println()
			}
			return nil
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "nova",
		Short: "Conversar con NovaCore",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Parent().RunE(cmd.Parent(), args)
		},
	})
	return cmd
}
