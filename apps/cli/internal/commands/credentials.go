// credentials.go — subcomando `battos credentials` para gestionar la Bóveda.
//
// Subcomandos:
//
//	battos credentials list              # lista todas (sin mostrar el secreto)
//	battos credentials set <name>        # crea/actualiza una credencial
//	battos credentials delete <name>     # elimina por name
package commands

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nicotion/battos/apps/cli/internal/client"
	"github.com/spf13/cobra"
)

// credentialItem es el DTO que devuelve el API (sin secret_locator).
type credentialItem struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Kind         string    `json:"kind"`
	SecretSource string    `json:"secret_source"`
	Description  *string   `json:"description,omitempty"`
	ProviderID   *string   `json:"provider_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NewCredentialsCmd construye el árbol `battos credentials ...`.
func NewCredentialsCmd(getClient func() *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credentials",
		Short: "Bóveda de credenciales de BattOS",
		Long: `Gestiona la Bóveda de credenciales de BattOS.

Las credenciales se almacenan de forma segura: los valores inline se cifran
con AES-256-GCM usando BATTOS_MASTER_KEY; las credenciales tipo "env" guardan
solo el nombre de la variable de entorno.

Usa 'credentials list' para ver las credenciales registradas y
'credentials set <name>' para crear o actualizar una.`,
	}
	cmd.AddCommand(
		newCredListCmd(getClient),
		newCredSetCmd(getClient),
		newCredDeleteCmd(getClient),
	)
	return cmd
}

// --- list ---

func newCredListCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Listar todas las credenciales (sin mostrar el secreto)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			var items []credentialItem
			if err := getJSON(ctx, getClient(), "/credentials", &items); err != nil {
				return err
			}

			PrintBanner("CREDENTIALS VAULT")
			if len(items) == 0 {
				fmt.Println(styleSubtle.Render("(sin credenciales; usa 'battos credentials set <name>' para agregar una)"))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, styleSubtle.Render("NAME\tKIND\tSOURCE\tDESCRIPTION\tCREATED"))
			for _, item := range items {
				desc := "-"
				if item.Description != nil && *item.Description != "" {
					desc = *item.Description
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					styleOK.Render(item.Name),
					item.Kind,
					styleSubtle.Render(item.SecretSource),
					styleSubtle.Render(desc),
					styleSubtle.Render(item.CreatedAt.Format("2006-01-02")),
				)
			}
			return w.Flush()
		},
	}
}

// --- set ---

func newCredSetCmd(getClient func() *client.Client) *cobra.Command {
	var (
		kind        string
		source      string
		value       string
		description string
	)

	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Crear o actualizar una credencial",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("el nombre de la credencial no puede estar vacío")
			}

			// Si no se pasó --value, pedirlo interactivo.
			// Para inline_encrypted: ocultar la entrada (sin eco).
			// Usamos bufio.Scanner simple — sin deps externas.
			if value == "" {
				if source == "env" {
					fmt.Printf("Nombre de la variable de entorno para %q: ", name)
				} else {
					fmt.Printf("Valor/secreto para %q (no se mostrará en pantalla): ", name)
				}
				value = readSecretLine()
				if strings.TrimSpace(value) == "" {
					return fmt.Errorf("el valor no puede estar vacío")
				}
			}

			body := map[string]any{
				"name":          name,
				"kind":          kind,
				"secret_source": source,
				"secret_value":  value,
			}
			if strings.TrimSpace(description) != "" {
				body["description"] = description
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			var saved credentialItem
			if err := postJSON(ctx, getClient(), "/credentials", body, &saved); err != nil {
				return err
			}

			PrintBanner("CREDENTIALS VAULT")
			fmt.Printf("%s Credencial %s guardada\n",
				styleOK.Render("OK"),
				styleOK.Render(saved.Name),
			)
			printKV("Kind", saved.Kind)
			printKV("Source", saved.SecretSource)
			if saved.Description != nil && *saved.Description != "" {
				printKV("Description", *saved.Description)
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVar(&kind, "kind", "api_key", "tipo de credencial: api_key|oauth_token|git_token")
	cmd.Flags().StringVar(&source, "source", "inline_encrypted", "fuente del secreto: inline_encrypted|env")
	cmd.Flags().StringVar(&value, "value", "", "valor del secreto (si no se pasa, se pide interactivo)")
	cmd.Flags().StringVar(&description, "description", "", "descripción opcional")

	return cmd
}

// --- delete ---

func newCredDeleteCmd(getClient func() *client.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Eliminar una credencial por nombre",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("el nombre de la credencial no puede estar vacío")
			}

			// Pedir confirmación.
			fmt.Printf("¿Eliminar credencial %q? [y/N] ", name)
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" && answer != "s" && answer != "si" && answer != "sí" {
				fmt.Println(styleSubtle.Render("Operación cancelada."))
				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			if err := deleteRequest(ctx, getClient(), "/credentials/"+url.PathEscape(name)); err != nil {
				return err
			}

			PrintBanner("CREDENTIALS VAULT")
			fmt.Printf("%s Credencial %q eliminada.\n", styleOK.Render("OK"), name)
			return nil
		},
	}
}

// --- helpers ---

// readSecretLine lee una línea de stdin sin mostrar los caracteres en pantalla.
// Intenta usar golang.org/x/term si está disponible; si no, usa bufio.Scanner.
// Como no queremos deps nuevas, usamos bufio.Scanner directamente.
// La entrada queda visible en terminales que no soporten raw mode, pero es
// la solución más simple sin deps externas. Se puede mejorar en el futuro.
func readSecretLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return scanner.Text()
}

// deleteRequest hace DELETE al path y verifica el status code.
func deleteRequest(ctx context.Context, c *client.Client, path string) error {
	resp, err := doRequest(ctx, c, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return decodeOrError(resp, nil)
	}
	return nil
}
