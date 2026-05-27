# 13 - Comparativa de Fuentes Agent OS y BattOS

> Analisis realizado el 25 de mayo de 2026 antes de iniciar Fase 3.
> Los tres documentos externos son fuentes conceptuales; sus afirmaciones
> sobre herramientas, rendimiento o casos de uso no se validan aqui como hechos.
>
> **Addendum 2026-05-26**: de esta comparativa se adopto para v0.1 ejecucion
> supervisada con Claude Code/Codex, NovaCore opcional, Knowledge Center y el
> dashboard completo. La definicion vigente esta en
> `14-producto-final-y-roadmap.md` y `adr/0011-v01-ejecucion-supervisada.md`;
> cualquier referencia a un MVP solo de lectura describe el plan anterior.

## Fuentes

| Clave | Documento | Enfoque principal |
|---|---|---|
| A | `Informe de Investigacion_ Arquitectura y Despliegue de un Sistema Operativo Agentico (Agentic OS).md` | Vision integral: chasis/motor, skills, Obsidian, dashboard, ROI y Dreaming |
| B | `Informe de Investigacion_ Arquitectura y Operatividad del Agentic OS.md` | Operacion del stack de cuatro capas: Claude, OpenClaw, Hermes, Obsidian, dashboard local |
| C | `Sistema Operativo de Agentes (Agent OS).md` | Blueprint ampliado de siete capas: hardware, memoria, cerebro, agentes, Mission Control, produccion y Loop |
| BattOS | Este repositorio | Producto self-hosted con API/CLI, registries, Memory Core y roadmap por fases |

Nota: la fuente A contiene dos versiones casi duplicadas del mismo informe.

## Resultado Ejecutivo

BattOS comparte la tesis central de las tres fuentes:

- El sistema importa mas que el modelo.
- Claude, Codex, Gemini, Hermes u OpenClaw son motores o runtimes sustituibles.
- La memoria persistente evita el cold start de las conversaciones aisladas.
- Un centro de comando debe exponer estado, actividad y control.
- Skills y automatizaciones convierten prompts sueltos en procesos reutilizables.

BattOS no es una replica de esos informes. Es una reinterpretacion mas
productizable y conservadora:

| Dimension | Fuentes A/B/C | BattOS |
|---|---|---|
| Centro del sistema | Vault local + agentes externos + dashboard | API Go como autoridad, CLI/Web como clientes |
| Memoria | Obsidian/Markdown; C agrega OMI para captura | SQLite + FTS5 embebido en el API |
| Ejecucion | Se propone temprano, incluso headless/autonoma | Deliberadamente excluida de v0.1 |
| Persistencia operativa | Largamente conceptual | PostgreSQL schema + Memory Core ya implementados |
| Contratos | No especificados | OpenAPI planeado, sqlc ya adoptado |
| Seguridad | Mencionada de forma general | Read-only v0.1, riesgos y HITL documentados |

La conclusion no es elegir documentos o BattOS. La mejor arquitectura es
usar BattOS como plano de control y absorber de las fuentes sus capas de
conocimiento, produccion, objetivos y aprendizaje, sin introducir autonomia
prematura.

## Como Funciona Cada Sistema

### Fuente A: Agentic OS de skills y memoria

```text
Dominio
  -> Tarea
    -> Skill
      -> Automatizacion
        -> Motor externo
          -> Output
            -> Obsidian / Journal / indices
              -> Dreaming + ROI
```

Su aporte diferencial es organizar la actividad del usuario en una jerarquia
de trabajo y convertir tareas repetidas en skills evaluables. Tambien propone
medir valor economico y ejecutar un analisis periodico de mejora.

### Fuente B: Mission Stack operativo

```text
Memoria/objetivos en Obsidian
  -> Hermes investiga y orquesta
    -> Claude razona o genera
      -> OpenClaw ejecuta localmente
        -> Dashboard supervisa agentes y objetivos
```

Su aporte diferencial es presentar una composicion concreta de herramientas y
una experiencia de usuario: control rooms, goals tracker, Kanban y entrada por
voz.

### Fuente C: Agent OS de siete capas

```text
Hardware local
  -> Memoria (Obsidian + OMI)
    -> Cerebro/model router
      -> Harnesses/agentes
        -> Mission Control
          -> Produccion (Goal Mode / SEO / Studio)
            -> Loop de rearchivo y aprendizaje
```

Su aporte diferencial es incluir captura ambiental, artefactos multimedia,
modos de produccion de largo horizonte y retroalimentacion automatica.

### BattOS actual y planeado

```text
Config + Registries PostgreSQL
  -> Agent identity + Agent Runtime
    -> API Go / CLI / futuro Command Center
      -> Memory Core SQLite+FTS5
        -> futuro execution engine + usage + NovaCore
```

Al cierre de Fase 2, BattOS tiene:

- API de system status y Memory Core.
- CLI de status y memoria.
- Schema PostgreSQL para runtimes, providers, models, projects, agents,
  skills, MCPs, ejecuciones, usage, logs y NovaCore.
- Memory Core con save, search, recent, stats y health.

Todavia no tiene:

- OpenAPI y CRUD HTTP/CLI de registries.
- Detector operativo de CLIs/providers.
- Dashboard.
- Execution engine.
- NovaCore, Goal Mode, artifacts, ROI o Loop.

## Comparativa De Arquitectura

| Tema | Fuente A | Fuente B | Fuente C | BattOS actual | Juicio |
|---|---|---|---|---|---|
| Definicion de OS | Chasis para motores | Panel local integrado | Sistema unificado de siete capas | Capa self-hosted de orquestacion | Mismo nucleo conceptual |
| Motor intercambiable | Claude/Codex/Gemini | Principalmente Claude | Claude/GPT/Grok/Gemini | `agent_runtimes`, providers y models | BattOS modela mejor el desacople |
| Orquestacion | Skills/automatizaciones | Hermes | Harnesses + Mission Control | Diseñada; registries parciales | Fuentes aportan operacion; BattOS aporta plataforma |
| Ejecucion | Headless/local/remota | OpenClaw | Goal Mode 24/7 | Fuera de v0.1 | BattOS es mas seguro |
| Memoria | Obsidian Raw/Wiki/Outputs | Obsidian + journals/goals | Obsidian + OMI | Memory Core SQLite+FTS5 | Combinar capas, no sustituir |
| UI | Dashboard web u Obsidian | Mission Dashboard | Mission Control + previews | Next.js pendiente | Fuente C enriquece alcance visual |
| Observabilidad | Net ROI y logs | Telemetria por agente | Previews, audio, activity logs | System metrics y schema usage/logs | BattOS tiene base, falta producto |
| Mejora continua | Dreaming | Aprendizaje por journals | Loop de rearchivo | NovaCore futuro, no loop | Integrar como recomendaciones controladas |
| Seguridad | Poco detallada | Local-first como mitigacion | Critica SaaS/plomeria | HITL/read-only definidos | BattOS es mas maduro en control |
| Productizacion | Parcial | Scaffolding de herramientas | Ambicion de produccion autonoma | API/CLI/DB/versionado | BattOS es mejor base de producto |

## Mapeo Completo De Capacidades

| Capacidad | A | B | C | BattOS | Estado / integracion sugerida |
|---|:---:|:---:|:---:|---|---|
| Chasis independiente del modelo | Si | Si | Si | Diseñado con runtimes | Mantener como principio principal |
| Catalogo de runtimes | Implicito | Herramientas fijas | Harnesses | Schema y seeds listos | Fase 3/4 expone y detecta |
| Multi-provider/model routing | Mencionado | Centrado en Claude/Ollama | Explicito | Tables `providers`/`models` | Model Advisor posterior |
| Dominio -> tarea -> skill -> automation | Si | No detallado | Produccion por modos | No modelado | Decision necesaria antes de ampliar contrato |
| Skills reutilizables | Si | Indirecto | Indirecto | Tabla `skills`, sin API funcional | Completar registry; ampliar lifecycle |
| Skill triage | Si | No | No | No existe | NovaCore futuro puede proponer drafts |
| A/B testing de skills | Si | No | No | No existe | Posterior a ejecuciones |
| Workflows/cascadas | Si | No | Goal Mode/Studio | n8n solo conceptual | Crear modelo futuro de automations |
| Memoria operacional buscable | Parcial via vault | Via vault | Via vault | Implementada con FTS5 | Fortaleza actual de BattOS |
| Vault humano Markdown | Si | Si | Si | No | Adapter opcional, no reemplazo |
| Raw/Wiki/Outputs | Si | No explicito | Captura/estructuracion/output | No | Knowledge workspace futuro |
| Journal diario | Si | Si | Loop/captura | No | Tipo artifact/memory o integracion vault |
| Indices para navegar vault | Si | No | No | No | Solo si se incorpora vault grande |
| Captura continua pantalla/audio (OMI) | No | No | Si | No | Alto riesgo de privacidad; fuera de corto plazo |
| Prompt como activo/IP | Implícito por skills | Si | Memoria de outputs | No formal | Registrar prompts/templates versionados |
| Dashboard web | Si | Si | Si | Planeado | Fase 5 |
| Control rooms por agente | No explicito | Si | Mission Control | No | Agregar al diseño frontend |
| Goals tracker | Contexto negocio | Si | Goal Mode | No | Modelo de goals antes de autonomia |
| Kanban | No | Si | No | No | Opcional en UX, tras tasks |
| Voz/audio UX | No | Si | Si | No | Posterior, no condicionante |
| App previews | No | Casos web | Si | No | Artifact viewer futuro |
| Galeria multimedia | No | No | Si | No | Artifact registry futuro |
| Ejecucion local | Si | OpenClaw | Hardware/harness | Excluida v0.1 | Fase posterior con runner |
| Ejecucion remota | Si | No clara | Autonomia 24/7 | No | Runners/nodes posterior |
| Headless commands | Si | OpenClaw | Goal Mode | No | Solo con sandbox y HITL |
| Goal Mode largo horizonte | No formal | No formal | Si | No | No priorizar hasta execution engine |
| ROI / valor del trabajo | Si explicito | Caso ROI | Beneficio narrativo | Costos/budgets preparados | Agregar modelo de valor luego |
| Cost tracking | Si | No detallado | No detallado | Schema preparado | Activar al ejecutar |
| Logs/auditoria | Si | Telemetria | Si | Schema + status parcial | Prioritario al ejecutar |
| Dreaming / auto-mejora | Si | Aprendizaje acumulativo | Loop | NovaCore futuro parcial | Recomendaciones, no mutacion automatica |
| Rearchivo automatico de outputs | Implícito | Memoria acumulativa | Si | No | Artifact -> memory pipeline |
| Soberania local-first | Si | Si | Si | Self-hosted/local memory | Extender con privacy policies |
| Ollama/modelos locales | Si | Si | Implicito local-first | No configurado | Provider/runtime futuro |
| Integracion Hermes | Si | Central | Harness | Runtime seed | Adapter futuro |
| Integracion OpenClaw | Si | Central | Harness | Runtime seed | Adapter futuro |
| Integracion MCP | No central | No | No | Prevista explicitamente | Ventaja de BattOS |
| Contrato API tipado | No | No | No | Planeado con OpenAPI | Ventaja de BattOS |
| DB estructurada de control | No | No | Pide robustez local | PostgreSQL + SQLite | Ventaja de BattOS |
| Inicio vacio y portable | No; stack opinionado | Herramientas prefijadas | Stack opinionado | ADR-0008 | Ventaja de producto general |
| HITL/permisos | No desarrollado | No desarrollado | Autonomia promocionada | Diseñado explicitamente | Mantener como requisito |

## Que Tiene Cada Fuente Que BattOS No Tiene

### De la Fuente A

| Pieza faltante | Valor para BattOS | Forma sana de integrarla |
|---|---|---|
| Jerarquia dominios/tareas/skills/automatizaciones | Organiza el producto por trabajo real, no solo recursos tecnicos | Diseñar ontologia; no es necesario implementarla completa en v0.1 |
| Skill triage | Permite descubrir automatizaciones desde rutinas reales | NovaCore crea drafts revisables, nunca instala solo |
| Composicion de skills | Convierte el OS en sistema de produccion | Modelo `automations`/`workflow_steps` posterior |
| Raw/Wiki/Outputs | Orden humano de conocimiento y entregables | `knowledge_workspaces`/`artifacts` con vault opcional |
| Net ROI | Ayuda a decidir que automatizar | Value metadata + analytics despues de execution |
| Dreaming | Detecta mejoras y memoria sucia | Job de recomendaciones con HITL |

### De la Fuente B

| Pieza faltante | Valor para BattOS | Forma sana de integrarla |
|---|---|---|
| Control Rooms | Visibilidad individual de runtimes/agentes | Vistas del dashboard basadas en executions/logs |
| Goals Tracker | Alinea agentes con objetivos | Entidad `goals` asociada a projects/domains |
| Kanban | Interfaz de tareas comprensible | Solo si se incorpora `tasks` |
| Entrada por voz | Reduce friccion operativa | Extender UI mas adelante; no condiciona backend |
| Prompts como IP | Formaliza instrucciones valiosas | Templates/versiones dentro del registry de skills |
| Ollama offline | Privacidad y operacion local | Provider/runtime local con health y modelo catalogado |

### De la Fuente C

| Pieza faltante | Valor para BattOS | Forma sana de integrarla |
|---|---|---|
| Hardware/runners como capa | Define donde corre cada tarea y que puede tocar | Entidad `execution_nodes` antes de execution engine |
| OMI/captura ambiental | Ingesta automatica de contexto | No priorizar; requiere privacidad/consentimiento fuertes |
| Artifact previews y galeria | Hace visible el resultado del OS | `artifacts` + viewer en Command Center |
| Goal Mode | Trabajos largos sin supervision constante | Future orchestration con approvals/budget/cancel |
| Production modules | Convierte plataforma en producto por casos de uso | Modulos instalables sobre skills/workflows |
| The Loop | Reinyecta resultados utiles al conocimiento | Pipeline output -> review -> memory/vault |

## Que Es Mejor Segun La Dimension

| Dimension | Mejor propuesta | Motivo |
|---|---|---|
| Vision de habilidades y procesos | Fuente A | Es la unica que estructura dominio, tarea, skill y automatizacion |
| UX operativa del dashboard | Fuente B | Control rooms, goals tracker y Kanban son conceptos concretos |
| Amplitud de producto futuro | Fuente C | Incluye artifacts, produccion, Goal Mode y Loop |
| Arquitectura de plataforma implementable | BattOS | Tiene API, DB, CLI, schema, pruebas y roadmap incremental |
| Memoria operacional del sistema | BattOS | FTS5 y API son mas consultables y auditables que solo archivos |
| Memoria humana y activos legibles | Fuentes A/B/C | Markdown/vault es superior para notas y outputs humanos |
| Motor/runtimes intercambiables | BattOS | Lo modela explicitamente sin fijarse a Claude/Hermes/OpenClaw |
| Seguridad y gobernanza | BattOS | No confunde autonomia deseable con permisos ya resueltos |
| Local-first completo/offline | Fuente B + BattOS futuro | B agrega Ollama; BattOS aporta plataforma self-hosted |
| Autonomia | Fuente C, como vision | Es la mas ambiciosa, pero requiere controles no descritos |

## Evaluacion Critica De Las Fuentes

| Propuesta | Acierto | Riesgo o limite |
|---|---|---|
| Obsidian como memoria | Excelente para soberania y lectura humana | No basta como sistema transaccional/API/metricas |
| Hermes/OpenClaw como stack fijo | Facilita imaginar un sistema ensamblado | Acopla producto a herramientas que pueden cambiar |
| Ejecucion headless temprana | Demuestra valor rapidamente | Abre riesgos de permisos, costos y auditoria |
| Goal Mode 24/7 | Norte de producto potente | Peligroso sin quotas, cancelacion, sandbox y HITL |
| OMI/captura continua | Contexto rico | Privacidad y volumen de datos muy sensibles |
| n8n/Zapier como plomeria fragil | Advierte contra flujos opacos | BattOS no debe descartar conectores: son runtimes utiles bajo control |
| Dreaming/Loop | Mejora continua valiosa | Debe sugerir y archivar con aprobacion, no reescribir solo |

## Donde BattOS Ya Supera A Los Documentos

| Ventaja BattOS | Evidencia |
|---|---|
| Runtimes generalizados, no stack fijo | `agent_runtimes` incluye CLIs, MCP, webhooks, direct API y manual |
| Memoria operativa real | `apps/api/internal/memory/core.go` y endpoints `/memory/*` |
| Estado observable ya funcional | `/health`, `/version`, `/status` y CLI `battos status` |
| Persistencia estructurada preparada | `apps/api/migrations/0001_init.sql` |
| Costos y auditoria modelados de antemano | `executions`, `usage_events`, `system_logs`, budgets |
| Producto portable y no opinionado | Inicio en blanco segun ADR-0008 |
| Gobernanza de autonomia | ADR-0006 y diseño HITL de NovaCore |
| Camino de contrato estable | OpenAPI/oapi-codegen planificado para Fase 3 |

## Brechas Reales De BattOS

| Brecha | Consecuencia | Prioridad |
|---|---|---|
| No existe ontologia de domains/tasks/goals | El sistema registra componentes, pero no expresa por que se usan | Alta, definir antes de contratos extensos |
| No existe registry de artifacts/outputs | No puede mostrar producto del trabajo ni alimentar Loop | Alta en diseño; implementar luego |
| Skills sin lifecycle operativo | No hay triage, composicion, evaluacion ni uso medible | Alta despues del registry basico |
| No hay knowledge workspace/vault | Se pierde la capa humana de Raw/Wiki/Outputs | Media-alta |
| No hay execution nodes/runners | No puede diferenciar local, VPS o privado | Alta antes de ejecutar |
| No existe Goal Mode/automations | No puede orquestar trabajos de horizonte largo | Posterior a ejecucion segura |
| No hay ROI funcional | No puede demostrar valor de negocio | Posterior a datos reales |
| No hay Loop/Dreaming | El sistema no aprende del uso por si mismo | Posterior a logs/artifacts |
| No existe Ollama/local model | Self-hosted aun depende de proveedores futuros para IA | Media |
| Frontend no implementado | Brecha de acceso sigue abierta | Ya planeado, Fase 5 |

## Arquitectura Combinada Recomendada

La arquitectura resultante debe preservar la base robusta de BattOS y sumar la
profundidad funcional de las fuentes:

```text
Layer 1 - Control Plane (BattOS)
  API Go + OpenAPI + PostgreSQL + policy/HITL + audit

Layer 2 - Knowledge Plane
  Memory Core SQLite+FTS5 (memoria operacional)
  + Knowledge Workspace Markdown/Obsidian opcional (Raw/Wiki/Outputs)

Layer 3 - Work Model
  Domains -> Goals -> Tasks -> Versioned Skills -> Automations

Layer 4 - Runtime Plane
  Providers/Models + Agent Runtimes + MCP + execution nodes

Layer 5 - Execution Plane
  Jobs, approvals, sandbox, budgets, output/artifact capture

Layer 6 - Experience Plane
  Command Center, control rooms, goals view, artifacts/previews, NovaCore

Layer 7 - Learning Plane
  Usage/ROI, skill evaluation, memory hygiene, Loop/Dreaming recommendations
```

## Decisiones Previas A Fase 3

Fase 3 sigue siendo el paso correcto, pero debe empezar con decisiones de
modelo, porque OpenAPI cristalizara los objetos del producto.

| Decision | Recomendacion |
|---|---|
| Incorporar Obsidian como DB primaria | No. Mantener Memory Core como memoria operacional |
| Permitir un vault Markdown/Obsidian | Aprobado: `knowledge workspace` opcional, compatible con Obsidian, según ADR-0010 |
| Crear `domains`, `goals`, `tasks` ahora | Diseñar ahora; decidir implementacion minima junto a registries |
| Crear `automations` ahora | Diseñar extension; implementar al existir execution engine |
| Crear `artifacts` | Diseñarlo temprano; es clave para dashboard, outputs y Loop |
| Modelar execution nodes/runners | Si antes de cualquier ejecucion local/remota |
| Incluir Ollama | Agregar al roadmap como provider/runtime local |
| Implementar ROI/Dreaming pronto | No ejecutar aun; reservar datos necesarios |
| Implementar Goal Mode | Solo despues de seguridad, jobs, budgets y cancelacion |

### Decisión Adoptada: Workspace No Es Runtime

Obsidian no se incorpora como motor ni como requisito de instalación. BattOS
podrá producir o alojar una carpeta Markdown compatible con Obsidian para
documentos y outputs, mientras Memory Core y PostgreSQL conservan la verdad
operativa. En un VPS la carpeta puede existir sin interfaz gráfica; el usuario
puede sincronizarla o montarla y abrirla localmente con Obsidian si lo desea.

Ese workspace funciona como exportación o respaldo legible de los documentos
que contenga. No reemplaza backups de PostgreSQL ni de SQLite.

## Roadmap Derivado Adoptado

| Horizonte | Incorporacion desde las fuentes | Resultado |
|---|---|---|
| v0.1 / Fase 3 | Ontologia, OpenAPI, registries, `goals`/`tasks`/`artifacts`/`knowledge workspaces` y lifecycle/versiones de skills/prompts | Contrato que no limita la vision |
| v0.1 / Fase 4 | Adapters Claude Code/Codex, repositories, runs aislados, approvals, logs y Git controlado | Accion segura desde el inicio |
| v0.1 / Fase 5 | Command Center, Control Room, Work Board, Knowledge Center y NovaCore opcional | Producto usable desde dashboard |
| v0.2 | Export Markdown/Obsidian, Extension Platform, SDD y PR aprobado | Portabilidad y upgrades reversibles |
| v0.3 | Delivery aprobado, mas adapters, Ollama/routing y telemetria costo/valor | Ecosistema productivo |
| v0.4+ | ROI, skill evaluation, Loop/Dreaming recomendado y Goal Mode restringido | Mejora continua medible |

## Auditoria De Cobertura Del Roadmap

La comparativa inicial detectaba las piezas valiosas, pero algunas seguian
descritas solo como brechas. Tras revisar de nuevo las tres fuentes, la
incorporacion recomendada queda asi:

| Capacidad de las fuentes | Juicio | Tratamiento final en BattOS |
|---|---|---|
| Chasis con motores intercambiables | Es la tesis correcta | Ya es núcleo mediante runtimes/providers/MCP |
| Hermes, OpenClaw y CLIs equivalentes | Son motores utiles, no la identidad del OS | Adaptadores/runtimes sustituibles, nunca dependencias fijas |
| Domains, goals, tasks y automations | Necesario para expresar trabajo real | Reservar ontologia desde Fase 3; ejecutar automations despues |
| Skills testeables, triage y prompts como IP | Faltaba explicitarlo en roadmap | Reservar lifecycle/versiones/templates; drafts y revisión humana primero |
| Vault Raw/Wiki/Outputs y journals | Excelente capa humana | Knowledge Workspace opcional + promoción/exportación controlada |
| Manual del workspace e indices navegables | Evita entropia documental | `AGENTS.md` para reglas del repo e indices Markdown opcionales del workspace |
| Obsidian como base primaria | Incorrecto para un control plane | Sólo visor/editor opcional sobre Markdown; ADR-0010 |
| Ollama y routing de modelos | Valioso para local-first | Provider/runtime detectable en Fase 4; inferencia posterior |
| Control Rooms, Kanban, previews y galería | Mejora fuerte de acceso y auditoría visual | Dashboard operativo completo en v0.1 |
| Voz, audio, SEO y Studio especializados | Pueden aportar UX/productos, no plataforma base | Módulos posteriores sobre artifacts/skills, sólo si un caso real los pide |
| ROI y valor compuesto | Valioso sólo con datos reales | Primero telemetría de costo/resultado; métricas después |
| Runners, Goal Mode y ejecución headless | Runs supervisados son necesarios; autonomia no | Runs aislados y HITL en v0.1; Goal Mode restringido despues |
| Dreaming/The Loop | Útil para aprender del trabajo | Recomendaciones y promoción revisable, no auto-mutación temprana |
| OMI/captura continua | Contexto a costo de privacidad excesivo | Excluido del roadmap base; no default de BattOS |

Conclusión de la auditoría: el plan actualizado incorpora lo mejor de cada
fuente en forma compatible con BattOS. Lo que se excluye no se omite por
descuido, sino porque requiere riesgos o acoplamientos incompatibles con un
producto self-hosted, auditable y seguro.

## Regla De Producto

BattOS debe incorporar la ambicion de los documentos sin heredar sus atajos
peligrosos:

> Memoria local y resultados acumulativos, si; motores sustituibles, si;
> autonomia, solo con control plane, auditoria, limites de costo y aprobacion.
