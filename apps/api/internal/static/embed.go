// Package static embeds el build estático del dashboard Next.js.
//
// El directorio out/ se genera corriendo `npm run build` en apps/web/,
// que produce apps/web/out/, y luego el script de build lo copia a
// apps/api/internal/static/out/.
//
// Si out/ no existe o solo contiene .gitkeep al compilar Go, el handler
// estático no se monta (ver NewRouter en internal/server/router.go) y
// el servidor loguea "dashboard estático no disponible".
package static

import "embed"

// FS contiene el build estático del dashboard Next.js.
// El directorio apps/web/out/ se genera con `npm run build` en apps/web/.
// Si out/ no existe al compilar Go, el embed falla — correr el build web primero.
//
//go:embed all:out
var FS embed.FS
