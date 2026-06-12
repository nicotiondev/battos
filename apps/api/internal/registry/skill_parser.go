// Package registry contiene herramientas para cargar y parsear definiciones
// de skills desde archivos Markdown con frontmatter YAML.
package registry

import (
	"fmt"
	"strings"
)

// ParseSkillMD parsea un archivo .md con frontmatter YAML + body markdown.
//
// El frontmatter esta delimitado por "---" al inicio del archivo.
// Ejemplo:
//
//	---
//	name: my-skill
//	description: Does something useful
//	author: Nico
//	version: "1.0"
//	---
//
//	# My Skill
//	Body content in markdown...
//
// Parsea el frontmatter linea por linea (formato key: value).
// Retorna error si el frontmatter esta ausente o malformado.
// Campos no reconocidos se ignoran silenciosamente.
func ParseSkillMD(content string) (name, description, author, version, body string, err error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")

	if !strings.HasPrefix(content, "---") {
		return "", "", "", "", "", fmt.Errorf("frontmatter ausente: el archivo debe comenzar con ---")
	}

	// Buscar el cierre del frontmatter (segundo ---)
	// El primer "---" es el caracter inicial.
	rest := content[3:]
	// Permitir "---\n" o "---\r\n" al inicio
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}

	closeIdx := strings.Index(rest, "\n---")
	if closeIdx == -1 {
		// Podria ser que cierre justo al final sin newline
		if strings.HasSuffix(strings.TrimRight(rest, "\n"), "---") && strings.Count(rest, "---") >= 1 {
			closeIdx = strings.LastIndex(rest, "---") - 1
			if closeIdx < 0 {
				closeIdx = 0
			}
		} else {
			return "", "", "", "", "", fmt.Errorf("frontmatter no cerrado: falta el --- de cierre")
		}
	}

	frontmatter := rest[:closeIdx]
	afterClose := rest[closeIdx+4:] // skip "\n---"
	// Saltar newline inmediato despues del cierre
	if len(afterClose) > 0 && afterClose[0] == '\n' {
		afterClose = afterClose[1:]
	}
	body = afterClose

	// Parsear frontmatter linea por linea
	fields := map[string]string{}
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Quitar comillas opcionales alrededor del valor
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		fields[key] = val
	}

	name = fields["name"]
	description = fields["description"]
	author = fields["author"]
	version = fields["version"]

	if name == "" {
		return "", "", "", "", "", fmt.Errorf("campo 'name' obligatorio ausente en el frontmatter")
	}

	return name, description, author, version, body, nil
}
