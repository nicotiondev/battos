# Go Primer para BattOS

Conceptos mínimos de Go que vas a necesitar leyendo este repo. Pensado para alguien que viene de TypeScript/Python y nunca tocó Go.

> No es un curso. Es lo justo para leer el código sin frenarte.

---

## 1. Workspace, módulos y paquetes

- Un **módulo** es lo que define `go.mod`. Tiene un nombre tipo `github.com/nicotion/battos/apps/api` y una versión de Go.
- Un **paquete** es un directorio con archivos `.go` que comparten `package <nombre>`. Todos los archivos de un mismo directorio deben tener el mismo nombre de paquete.
- Un **workspace** (`go.work` en la raíz) lista los módulos locales que se usan juntos. Permite que `apps/cli` importe `packages/core` sin pasar por GitHub.

En este repo:
```
go.work             ← lista los 3 módulos
apps/api/go.mod     ← módulo 1
apps/cli/go.mod     ← módulo 2
packages/core/go.mod ← módulo 3
```

## 2. Exportación

Una función, struct, campo o variable se **exporta** (es visible desde otro paquete) **si su nombre empieza con mayúscula**. No hay `export` keyword.

```go
func DoStuff() {}   // exportada
func doStuff() {}   // privada al paquete
```

Aplicable también a campos de struct: `Name` se ve fuera, `name` no.

## 3. `internal/`

Cualquier paquete dentro de un directorio llamado `internal/` **solo puede ser importado por código del mismo módulo**. Es la forma idiomática de "esto no es API pública".

En este repo todo lo que es lógica del API vive en `apps/api/internal/*`. Si quisieras importar `apps/api/internal/store` desde `apps/cli`, Go lo prohibe — y eso está bien, fuerza a pasar por HTTP.

## 4. Structs e interfaces

```go
type Project struct {
    ID   string
    Slug string
    Name string
}

type Repository interface {
    GetProject(ctx context.Context, slug string) (*Project, error)
}
```

- Las interfaces se **implementan implícitamente**. Si un tipo tiene el método con la firma correcta, ya implementa la interface. No hay `implements`.
- Idiomático: interfaces chicas (1–3 métodos) donde se *consumen*, no donde se *implementan*.

## 5. Errores

Go no tiene excepciones. Las funciones devuelven `error` como último valor:

```go
project, err := store.GetProject(ctx, "red-nbl")
if err != nil {
    return fmt.Errorf("cargando proyecto: %w", err)
}
```

- `%w` envuelve el error original para que `errors.Is` y `errors.As` puedan inspeccionarlo.
- Convención del repo: **nunca tragar errores**. Si no querés propagar, logueá explícitamente.

## 6. `context.Context`

Todo handler HTTP, función que toca DB o IO, **recibe un `context.Context` como primer argumento**. Sirve para:
- Cancelar operaciones si el cliente desconecta.
- Pasar deadlines/timeouts.
- Llevar valores request-scoped (logger, traceID).

```go
func GetProject(ctx context.Context, slug string) (*Project, error) {
    // ...
}
```

## 7. Goroutines y channels

```go
go doSomething()  // lanza una goroutine

results := make(chan int)
go func() {
    results <- 42
}()
fmt.Println(<-results)
```

- `go <func>` lanza concurrencia. Es muy barato (KB de stack, no MB como un thread).
- Channels son tubos tipados para comunicar entre goroutines.
- En el repo los usamos sobre todo para SSE (broadcast a clientes conectados) y watch de procesos.

## 8. Defer

```go
file, err := os.Open(path)
if err != nil { return err }
defer file.Close()
```

`defer` agenda una llamada para cuando la función retorne. Es como `finally` pero en una línea. Útil para cerrar files, locks, transactions.

## 9. Pointers (los justos)

`*Project` es un puntero a `Project`. Los usamos sobre todo cuando:
- Devolvemos algo que puede ser `nil`: `func GetProject(...) (*Project, error)`.
- Queremos modificar el receptor de un método: `func (p *Project) SetName(...)`.

Para slices, maps y channels **no hace falta pointer** — ya son referencias internamente.

## 10. JSON

```go
type Project struct {
    ID   string `json:"id"`
    Slug string `json:"slug"`
    Name string `json:"name"`
}
```

Los tags `json:"..."` controlan cómo se serializa. `encoding/json` de la stdlib alcanza para todo el repo.

## 11. Tooling esencial

```bash
go build ./...           # compila todo
go test ./...            # tests recursivos
go run ./cmd/api         # corre el binario sin instalar
go install ./apps/cli/cmd/battos  # instala el binario en $GOBIN
go vet ./...             # linter básico stdlib
go mod tidy              # limpia dependencias en go.mod/go.sum
gofmt -w .               # formatea
```

## 12. Estilo del proyecto

- Indentación: tabs (estándar Go).
- Línea máx: no hay hard limit, pero ~120 chars es razonable.
- Imports agrupados: stdlib, vacío, externos, vacío, internos del módulo.
- Comentarios de paquete: `// Package xxx hace …` arriba del primer archivo.
- Comentarios de funciones exportadas: `// FuncName hace …` empezando con el nombre.

## 13. Aprendizaje rápido

Para profundizar (sin matarte):
1. [Tour of Go](https://go.dev/tour) — 30 min, no más.
2. [Effective Go](https://go.dev/doc/effective_go) — leer secciones puntuales cuando algo no cuadre.
3. Mirar código del repo: empezar por `apps/cli/cmd/battos/main.go` (más simple) y subir.

## 14. Cosas que NO necesitás aprender ahora

- Reflection (`reflect`).
- Build tags más allá de `//go:build linux/windows`.
- cgo (este repo evita cgo a propósito).
- Generics avanzados (los pocos casos que aparecen son simples).

Si aparece algo que no cuadra, agregar acá una sección con la explicación.
