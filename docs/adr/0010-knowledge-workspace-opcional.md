# ADR-0010: Knowledge Workspace Markdown opcional, compatible con Obsidian

- **Status**: Accepted
- **Fecha**: 2026-05-25
- **Decidido por**: Nico + Codex

## Context

Los informes externos de Agent OS aportan una capa valiosa que BattOS todavia
no tiene: documentos, diarios y entregables faciles de leer y navegar como
archivos Markdown. Las fuentes proponen Obsidian como memoria principal, pero
BattOS ya decidio que su memoria operacional es Memory Core (SQLite + FTS5) y
que PostgreSQL sostiene los registries y la telemetria.

Ademas, BattOS debe poder instalarse en un VPS o servidor minimo sin obligar a
instalar una aplicacion de escritorio. Obsidian es util para navegar Markdown,
pero no es un servicio de runtime para un servidor headless.

## Decision

BattOS incorporara un **Knowledge Workspace** opcional: una carpeta portable
de documentos Markdown y adjuntos, compatible con Obsidian y con cualquier
editor o visor de Markdown.

- Memory Core sigue siendo la memoria operacional consultable por API/CLI.
- PostgreSQL sigue siendo la fuente de verdad de registries, estados y
  telemetria.
- El Knowledge Workspace sera un modulo habilitable por configuracion; un
  despliegue sin el modulo conserva toda la funcionalidad central.
- Su layout inicial recomendado sera `Raw/`, `Wiki/` y `Outputs/`, con indices
  Markdown cuando el volumen lo requiera.
- Obsidian sera una experiencia de lectura/edicion opcional sobre esa carpeta,
  normalmente desde el equipo del usuario mediante sync o montaje; no una
  dependencia del API ni del VPS.

La primera integracion debe ser conservadora: exportar documentos y artifacts
seleccionados a Markdown portable. Importar o sincronizar cambios de vuelta
hacia BattOS requerira reglas explicitas de identidad, conflictos, auditoria y
permisos antes de implementarse.

## Respaldo Y Portabilidad

El vault Markdown puede actuar como respaldo legible y portable de documentos,
outputs y exportaciones seleccionadas. No reemplaza un backup operativo de
PostgreSQL ni de la base SQLite del Memory Core. Un despliegue serio debe
respaldar ambos almacenes por sus mecanismos propios y, si el modulo esta
habilitado, respaldar tambien la carpeta Markdown.

## Perfiles De Despliegue

| Perfil | Componentes | Uso |
|---|---|---|
| Servidor minimo | BattOS + PostgreSQL + Memory Core | API/CLI/Command Center sin vault |
| VPS con workspace | Servidor minimo + carpeta Markdown exportable | Documentos y outputs portables; sin UI Obsidian requerida |
| Estacion de conocimiento | BattOS + workspace sincronizado + Obsidian local opcional | Navegacion y edicion humana mas comoda |

## Consequences

### Positivas

- Suma la capa humana de Raw/Wiki/Outputs sin cambiar la identidad de BattOS.
- Permite deployments ligeros y self-hosted; Obsidian nunca bloquea el arranque.
- Los documentos quedan portables, versionables y legibles fuera del producto.
- Abre el camino para previews, artifacts y journals sin forzar ejecucion
  autonoma.

### Negativas

- Aparece una segunda representacion de cierta informacion exportada.
- Una sincronizacion bidireccional futura tendra problemas reales de conflictos
  y permisos que no deben ocultarse.
- Requiere documentar con precision que esta respaldado en Markdown y que solo
  vive en las bases operativas.

## Non-Goals

- Reemplazar Memory Core por Obsidian o archivos Markdown.
- Requerir Obsidian en el servidor o como dependencia de BattOS.
- Incluir captura continua de pantalla/audio o memoria invasiva.
- Convertir el workspace en canal de ejecucion autonoma o sincronizacion
  bidireccional temprana.

## Roadmap

- **v0.1**: administrar knowledge workspaces, journals y artifacts canonicos
  dentro de BattOS y mostrarlos en Knowledge Center.
- **v0.2**: exportacion manual o programable de Markdown hacia un vault
  compatible con Obsidian, configuracion del exporter y backups documentados.
- **Posterior**: importacion/sync solo despues de versionado, auditoria y
  resolucion de conflictos.

## Related

- `docs/05-memory-core.md` - memoria operacional existente.
- `docs/10-roadmap.md` - incorporacion incremental.
- `docs/13-comparativa-agent-os-sources.md` - motivacion desde las fuentes.
- ADR-0004 - Memory Core propio.
- ADR-0011 - ejecucion supervisada de `v0.1`; no modifica la frontera del vault.
