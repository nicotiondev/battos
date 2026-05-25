package handlers

import (
	"encoding/json"
	"io"
)

// encodeJSON helper interno usado por los handlers para no importar
// el paquete server (que crearía un ciclo de imports).
func encodeJSON(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}
