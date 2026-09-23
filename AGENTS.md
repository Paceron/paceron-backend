# Paceron Backend — Guía de trabajo (OpenCode)

Equivalente de [`CLAUDE.md`](CLAUDE.md) para sesiones de OpenCode — mismo contenido de fondo, reorganizado para no depender de herramientas específicas de Claude Code (skills, memoria automática entre sesiones, subagentes). Si algo cambia acá, reflejarlo también en `CLAUDE.md` y viceversa — son el mismo conjunto de convenciones para dos agentes distintos trabajando el mismo repo. **Este repo es ahora el que se desarrolla desde OpenCode** — Claude Code quedó reservado para `paceron-frontend` (otro repo), ver §8.

**Si tomás una decisión relevante para el equipo (workflow, arquitectura, configuración de proyecto), reflejala en ambos archivos** (`AGENTS.md` y `CLAUDE.md`) para que aplique a todos, no solo a la sesión donde se decidió.

## 0. Cómo interpretar un comentario — discusión vs. orden de implementar

No todo lo que se dice en una conversación es un pedido de cambiar código. Antes de editar/crear/borrar un archivo, distinguir:

- **Discusión/exploración** ("¿no sería mejor...", "che, esto está raro", "estuve pensando en...", una pregunta): responder, opinar, investigar si hace falta — **no** tocar código todavía.
- **Pedido explícito de acción** ("hacé X", "implementá Y", "arreglá Z", confirmar una propuesta ya discutida): ahí sí, implementar.
- **Ambiguo:** confirmar el alcance antes de escribir nada, aunque sea con una pregunta corta — más barato que revertir un cambio no pedido.

Esto aplica en cualquier agente/modo, no es exclusivo de un modo "plan" separado — un comentario casual no debería disparar una edición de archivo por las dudas. Igualar el alcance de la acción al pedido real, no al máximo que se podría inferir.

## 1. Stack

Go 1.26 + Gin (HTTP) + GORM (ORM sobre PostgreSQL/Supabase) + JWT (`golang-jwt/jwt`) + Swagger (`swaggo/swag`). Arquitectura en capas: Controllers → Delegates → Services → DAOs/RestClients → Infrastructure (diagrama completo en [`README.md`](README.md)). Frontend separado (Expo/React Native, otro repo, otro agente — ver §8).

Documentación técnica detallada:

- [`docs/STATE_MACHINES.md`](docs/STATE_MACHINES.md) — estados/transiciones/invariantes por entidad. Fuente de verdad de los valores: `cmd/api/domains/constants/`.
- [`docs/CATALOGO_Y_CALENDARIO.md`](docs/CATALOGO_Y_CALENDARIO.md) — modelo de datos, guards, endpoints del catálogo (`Exercise`/`Session`/`TrainingPlan`) y calendario de grupos (`GroupCalendarDay`), incluyendo el mecanismo de instanciación al asignar (§8). Actualizado al modelo de `asignacion-por-instanciacion`.
- [`docs/DEUDA_TECNICA_Y_PENDIENTES.md`](docs/DEUDA_TECNICA_Y_PENDIENTES.md) — **leer antes de arrancar**: bugs conocidos no arreglados, datos desactualizados en testing, features deferidas con diseño ya charlado, decisiones de "no tocar X". Conocimiento que vivía solo en memoria de sesiones previas de Claude Code, volcado acá para no perderlo al migrar a OpenCode.
- [`.agentics/CONVENTIONS.md`](.agentics/CONVENTIONS.md) — convenciones de código y capas (qué no está permitido: service-to-service imports, DAO directo desde controller).
- [`.agentics/STRUCTURE.md`](.agentics/STRUCTURE.md) / [`STRUCTURE_FOLDERS.md`](STRUCTURE_FOLDERS.md) / [`STRUCTURE_PACKAGE.md`](STRUCTURE_PACKAGE.md) — estructura de carpetas (hay 3 versiones con distinto nivel de detalle, se solapan a propósito — cualquiera de las 3 sirve, `.agentics/STRUCTURE.md` es la que referencia `CLAUDE.md`).
- [`.agentics/WORKFLOW.md`](.agentics/WORKFLOW.md) — cómo correr, testear, buildear, regenerar swagger.
- [`.agentics/GLOSSARY.md`](.agentics/GLOSSARY.md) — glosario del dominio.
- [`openspec/`](openspec/) — spec-driven development ya configurado (usar este esquema, no inventar uno nuevo).

## 2. Workflow de branches y PRs

- **Rama base:** `develop`. Producción es `master`, vía release (`release/<version>` → PR manual a `master`).
- **Nomenclatura:** `feature/<kebab-case>`, `fix/<kebab-case>`, `chore/<kebab-case>`, `docs/<kebab-case>` — siempre desde `develop` actualizado. Una rama = un tema.
- **Ciclo de PR:** al pushear una rama `feature/*`/`fix/*`/`chore/*`/`docs/*`, `auto-pr.yml` crea automáticamente una PR draft hacia `develop`. Se actualiza título/descripción, se marca lista (`gh pr ready` o editar por API si `gh pr edit` falla — ver quirk en §6), se espera CI verde, se mergea.
- **Quién mergea:** el usuario mergea las PRs y sincroniza `develop` él mismo, siempre — no asumir que hace falta hacerlo por él. Si no avisa que mergeó, verificar con `gh pr view <n> --json state,mergedAt` antes de asumir estado.
- Después de mergear: `git checkout develop && git pull && git branch -d <rama> && git remote prune origin`.

### Mensajes de commit

[Conventional Commits](https://www.conventionalcommits.org/): `tipo(alcance): resumen corto en imperativo`. Tipos: `feat`, `fix`, `docs`, `refactor`, `chore`. Subject solo alcanza casi siempre — cuerpo (1-2 oraciones) únicamente cuando el "por qué" no es obvio desde el diff.

### Formato de PR

**Título:** mismo estilo que el commit principal. **Descripción:**

```markdown
## Qué cambió
- Bullet corto por cambio.

Spec: `openspec/changes/<nombre>/` ← solo si esta rama usó una spec

## Cómo probarlo
Pasos mínimos. `go test ./...` → todo verde.
```

## 3. Cuándo usar OpenSpec

| Tamaño del cambio | Spec de OpenSpec |
|---|---|
| Retoque chico (1-3 archivos, sin decisión de arquitectura) | No — charlar y aprobar alcanza |
| Feature nueva, endpoint nuevo, cambio de modelo/schema | Sí |
| Cambio grande o que afecta varias capas con decisiones no triviales | Sí |

Sea cual sea el tamaño: **crear la rama dedicada antes de tocar código**, incluso si se saltea la spec.

**Comandos de OpenCode ya configurados** (`.opencode/commands/`, ver también [`SETUP.md`](SETUP.md)):
- `/opsx-explore` — pensar/investigar sin implementar (no escribe código, puede crear artefactos de OpenSpec).
- `/opsx-propose "<descripción>"` — crea el change y genera `proposal.md`/`design.md`/`tasks.md` en un paso.
- `/opsx-apply <change>` — implementa las tasks de un change ya propuesto.
- `/opsx-sync <change>` — sincroniza specs delta hacia las specs principales (merge inteligente, no copy-paste).
- `/opsx-archive <change>` — archiva un change completado.

Nota real de este repo: el paso de archive casi no se practica en la práctica — `openspec/specs/` tiene una sola capability archivada (`user-bank-alias`) contra ~30 changes ya mergeados. Si vas a escribir un delta `MODIFIED`/`RENAMED` contra una spec que nunca se archivó, `openspec validate --strict` avisa que el archive lo rechazaría — en ese caso, declarar el requirement completo bajo `## ADDED Requirements` en vez de `MODIFIED`, con una nota aclarando qué reemplaza (ver `openspec/changes/asignacion-por-instanciacion/specs/group-calendar/spec.md` como ejemplo ya hecho así).

## 4. Testing

`go test ./...` corre sobre `*_test.go` co-ubicados (convención `testify`). `services`/`controllers`/`delegates` usan mocks. `daos` corre contra Postgres real vía `testutils.SetupTestDB(t)` — se skipea solo sin `TEST_DB_HOST`, así que `go test ./...` anda sin Docker en cualquier máquina. Detalle: [`docs/TESTING.md`](docs/TESTING.md).

```bash
make test-db-up   # levanta postgres:16-alpine en :5433 (si el container ya existe, `docker start paceron-test-db`)
TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test go test ./...
```

### Coverage

`make coverage` (sin DB, parcial) / `make coverage-with-db` (con DB real, número real — mismo comando que corre `ci.yml`). **`.testcoverage.yml`: `threshold.total: 85`, bloquea merge si baja.** No ajustar el umbral hacia abajo si baja — agregar tests. `exclude.paths` cubre solo paquetes sin lógica real (DTOs puros, constants, swagger autogenerado, `testutils`, `main.go`) — un paquete con lógica real y 0% nunca entra ahí a propósito, para que la deuda sea visible.

## 5. CORS

`CORSMiddleware()` en `cmd/api/app/middleware.go` lee `CORS_ALLOWED_ORIGINS` (env, orígenes separados por coma), cae a una lista default hardcodeada si no está seteada. En producción (Render, `render.yaml`) se configura explícitamente. Al agregar un dominio nuevo, actualizar **ambos lugares** (fallback en código + `render.yaml`, los 2 services: producción y preview de develop).

## 6. Quirks conocidos

- `gh pr edit` puede fallar con `GraphQL: Projects (classic) is being deprecated... (repository.pullRequest.projectCards)` — es un bug del CLI leyendo un campo ajeno, no un error real del edit. Workaround: `gh api repos/<owner>/<repo>/pulls/<n> -X PATCH -f title="..." -f body="..."`.
- El toolchain de Go 1.26 vía `GOTOOLCHAIN=auto` no trae `covdata` — falla `go test -coverprofile` sobre paquetes sin `_test.go`. Por eso `make coverage`/`ci.yml` filtran a paquetes con `TestGoFiles` (`go list -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./... | xargs go test ...`), no `./...` directo.
- `attendances.training_session_id` y `workout_feedback.assigned_session_id`/`assigned_exercise_id` son FK **opacas** (BIGINT > 0, sin constraint de DB) — anticipan tablas/entidades que todavía no existen del todo. Ver §7: `asignacion-por-instanciacion` es la pieza a la que apuntaban las columnas de `workout_feedback` (el borrado de instancia superada ya las consulta), aunque la FK real sigue siendo deuda.
- El deploy en Render tiene cold-start de ~20-25s en la primera request tras inactividad (plan free) — no es error real.
- Dos proyectos de Supabase separados (testing/producción) — `master` en Render pega a producción, todo lo demás (`develop`/local) pega a testing por default, sin flag. Detalle: [`docs/ENVIRONMENTS.md`](docs/ENVIRONMENTS.md).

## 7. Rework de catálogo/calendario — instanciación (implementado)

Decisión de equipo (2026-09-19), spec en `openspec/changes/asignacion-por-instanciacion/` — **implementada en esta rama** (`feature/asignacion-por-instanciacion`):

- El mecanismo anterior (`calendario-asignacion-grupos` D8/D13 + `congelar-ejercicio-en-clon`) — clonado por divergencia al editar `Session`/`Exercise` del catálogo — se juzgó demasiado complejo/frágil de razonar y mantener, y **fue eliminado por completo** (no convive con el nuevo).
- **Reemplazado por instanciación proactiva**: `Exercise`/`Session`/`TrainingPlan` son 100% template. Asignar contenido a un día de calendario crea copias inmutables en tablas propias (`session_instances`/`session_exercise_instances`/`exercise_instances`, separadas del catálogo) en el momento del `save`, no al editar después. `GroupCalendarDay.session_id` pasó a `session_instance_id`.
- El chequeo de "día cerrado" (`isCalendarDayClosed`, misma regla de fecha/horario, ahora vive en `calendar_service.go`) cambió de rol: de "trigger de clonado" a "guard de escritura" — no se puede asignar/reasignar/borrar contenido de un día ya cerrado; `cancelled` sigue permitido sobre cerrado.
- Las respuestas de calendario (`CalendarDayResponse`/`NextSessionResponse`) embeben el detalle completo de la instancia (`session_instance` con `exercises`), sin `session_id` bare.
- Impacto para el frontend resumido en [`docs/FRONTEND_IMPACTO_INSTANCIACION.md`](docs/FRONTEND_IMPACTO_INSTANCIACION.md); la doc de dominio viviente es [`docs/CATALOGO_Y_CALENDARIO.md`](docs/CATALOGO_Y_CALENDARIO.md) §8.

## 8. Coordinación con Claude Code (paceron-frontend)

Desde 2026-09-19, `paceron-backend` se desarrolla desde OpenCode; Claude Code quedó reservado para `paceron-frontend` (juzgado más complejo). Si algo del rework de §7 (o cualquier cambio de contrato de API) afecta al frontend, avisar explícitamente — no asumir que la sesión de Claude en el otro repo se entera sola. Puntos concretos que el rework de §7 rompió del lado frontend (**confirmados en código**, detalle en [`docs/FRONTEND_IMPACTO_INSTANCIACION.md`](docs/FRONTEND_IMPACTO_INSTANCIACION.md)):
- `PUT /sessions/{id}` pierde `exclude_group_ids`/`clone_name`/`clone_description`.
- El shape de lo que devuelve el calendario cambió: `session_id` bare fue reemplazado por `session_instance` embebido (detalle completo en D9/design) — decisión "a definir en implementación" cerrada como embebido.
- `docs/BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md` §5 (clonado por divergencia, spec del lado frontend) quedó obsoleto — coordinar su actualización con quien mantiene ese repo.

## 9. Skills, subagentes y modelos

**Todo lo de esta sección es config personal, no de repo.** `opencode.json` (este archivo, tracked, compartido con el equipo) solo tiene el MCP de Mercado Pago — nada acá pisa el flujo de un compañero que también use OpenCode en este repo. Lo que sigue va en tu config global (`~/.config/opencode/opencode.jsonc` en tu caso), que aplica a vos en cualquier repo sin tocar la máquina/el setup de nadie más.

### Superpowers (plugin de skills)

Agregar `"superpowers@git+https://github.com/obra/superpowers.git"` al array `plugin` de tu config global — trae la misma librería de skills que se usó para diseñar este repo en Claude Code (`brainstorming`, `systematic-debugging`, `test-driven-development`, `writing-plans`, `subagent-driven-development`, `verification-before-completion`, `requesting-code-review`/`receiving-code-review`, `using-git-worktrees`, `finishing-a-development-branch`), mapeadas 1:1 a las herramientas nativas de OpenCode (`task`, `skill`, `todowrite`, `bash`, `grep`/`glob`, `webfetch` — ver `docs/README.opencode.md` del propio plugin). Reiniciar OpenCode después para que cargue. Verificar con "Tell me about your superpowers" o "use skill tool to list skills".

Es un plugin de terceros (corre JS instalado vía git) — reversible con solo sacar la entrada del array si no convence.

### Subagentes: cuándo sí, cuándo no

OpenCode tiene agentes **primary** (con los que hablás directo, ej. `build`/`plan`) y **subagentes** (invocables por el primary automáticamente o a mano con `@nombre`), cada uno con su propio modelo configurable en `agent.<nombre>.model` (formato `provider/model-id`, en tu config global — no acá). Si un agente no especifica modelo, el primary usa el global configurado y el subagente hereda el del primary que lo invocó.

**No existe selección de modelo dinámica por tarea todavía** (a la fecha de este documento, 2026-09-20) — hay issues abiertas en el repo de OpenCode pidiendo exactamente eso (parámetro `model` en el `task` tool, sintaxis `@agent:provider/model`), sin shippear. Lo que sí funciona hoy: armar una lista fija de subagentes con nombre, cada uno con su modelo, y `description`s claras — el agente primary decide **a cuál de esos** despachar según la tarea, de forma autónoma. Es autonomía acotada a la lista que vos armás de antemano, no elección libre modelo-por-modelo en cada llamada (eso sí lo tenía Claude Code en esta sesión vía el parámetro `model` del tool `Agent`, no es 1:1 portable a OpenCode hoy).

**Para el rework de `asignacion-por-instanciacion` específicamente: usar `subagent-driven-development`, no todo inline con un solo modelo.** El `tasks.md` de ese change ya está partido en 7 tareas acotadas — exactamente la forma que esa skill espera. Mismo patrón que ya funcionó en esta sesión para construir todo el dominio de catálogo/calendario: modelo barato/rápido para tareas mecánicas (DTOs, DAOs simples, wiring, tests que siguen un patrón), modelo fuerte para las tareas con juicio real (la lógica de instanciación en `calendar_service.go`, la interacción con `workout_feedback` en el borrado de instancia superada) y para el review de cada tarea + un review final de toda la rama.

**Cuándo NO usar subagentes:** cambios de 1-3 archivos sin ambigüedad (mismo criterio que la tabla de OpenSpec del §3) — ahí es más rápido y más barato en tokens ir directo con el agente `build`, el overhead de armar el paquete de review no se paga solo.

**No confiar en que `build` despache solo a `@implementer-mecanico`/`@code-reviewer` por su cuenta** — la autonomía de dispatch no está garantizada hoy. Comando personal `~/.config/opencode/commands/hacer-tarea.md` (namespace `user:`, se invoca `/user:hacer-tarea <tarea o número>`) fuerza el ciclo explícito: ubicar la tarea → decidir tier (mecánica → `@implementer-mecanico`, con juicio → el agente actual) → `@code-reviewer` obligatorio contra esa tarea puntual → si hay hallazgos, corregir y repetir el review → recién ahí tildar en `tasks.md`. Es personal (vive en tu config global, no en `.opencode/commands/` de este repo) porque referencia nombres de agentes que solo existen en tu config — un compañero sin esos agentes no podría correrlo.

### Configuración personal recomendada (config global, no `opencode.json` de este repo)

IDs confirmados contra `opencode models` (namespace real `opencode-go/`, plan del usuario, 2026-09-20):

```json
{
  "agent": {
    "build":                { "mode": "primary", "model": "opencode-go/gpt-5.6-luna" },
    "plan":                 { "mode": "primary", "model": "opencode-go/gpt-5.6-luna", "permission": { "edit": "deny", "bash": "deny" } },
    "implementer-mecanico": { "mode": "subagent", "model": "opencode-go/glm-5.3-flash" },
    "code-reviewer":        { "mode": "subagent", "model": "opencode-go/grok-4.6", "permission": { "edit": "deny" } }
  }
}
```

Criterio de asignación (juicio propio sobre modelos de 2026 sin benchmarks propios — **verificar con uso real, no tomar como verdad de laboratorio**):

- **`build`/`plan` (orquestación, diseño, decisiones de arquitectura):** `gpt-5.6-luna` — flagship de propósito general del plan, mejor apuesta por defecto para razonamiento no trivial.
- **`implementer-mecanico` (tareas acotadas tipo receta):** `glm-5.3-flash` — variante rápida/barata de una familia que en la práctica rinde bien en código estructurado. Candidatos a probar en paralelo dentro del mismo plan: `deepseek-v4-flash`, `qwen3.8-flash`, y el especializado `kimi-k2.7-code` (branding "Code" — vale la pena testear específicamente contra tareas Go de este repo, podría rendir mejor que los Flash genéricos justo por ser coding-specific).
- **`code-reviewer` (segunda opinión, busca lo que el implementador/orquestador no vio):** `grok-4.6`, deliberadamente **distinto** del modelo de `build` — dos familias de modelo distintas reducen puntos ciegos correlacionados.
- **Evitar por ahora en roles críticos** hasta validar informalmente: `hy4-preview` (estado preview, probable inestabilidad), la familia `muse-spark-*-contributor` (sufijo "Contributor" sugiere tier de distillation/comunidad, no flagship), y los `opencode/*-free` (tier gratuito de OpenCode, no del plan pago — probablemente modelos más débiles reservados para tareas triviales o fallback).

### Contexto y compactación — por qué "se marea"

Config de compactación (`compaction.auto`/`prune`/reserved buffer) existe en OpenCode pero no hay evidencia clara de qué tan bien preserva detalle fino al resumir — no es el lever principal para esto. El lever que sí funciona, y que ya tenés armado:

1. **Mantené la sesión primary corta — delegá a subagentes en vez de acumular todo en un solo hilo largo.** Un subagente arranca con contexto limpio (no hereda la historia pesada del primary), hace su tarea acotada, devuelve un resultado corto. El primary nunca necesita comprimir 50 mensajes de implementación de detalle si esos 50 mensajes pasaron en subagentes aparte.
2. **Persistí estado en archivos, no solo en la conversación** — exactamente lo que ya hace este repo: `tasks.md` con checkboxes marcados a medida que se completa cada tarea (mismo patrón usado en `congelar-ejercicio-en-clon`), y si usás `subagent-driven-development` completo, esa skill arma su propio ledger (`progress.md`) en disco. Si el contexto se comprime o se pierde el hilo, una sesión nueva (o vos mismo) puede reorientarse leyendo el archivo en vez de depender de que la compactación haya conservado el detalle correcto.
3. Para el rework puntual de `asignacion-por-instanciacion`: al ejecutar `/opsx-apply` o `subagent-driven-development` sobre ese `tasks.md`, tildar cada tarea en el archivo apenas se completa (no solo mentalmente/en el chat) — es la memoria persistente real, la conversación no lo es.
4. **Nunca commitear artefactos del workspace SDD** — `.superpowers/sdd/` es scratch auto-ignoreado (su propio `.gitignore` con `*`): ledger, briefs y reports sirven para recuperar el hilo dentro de una sesión, no para el historial de git. En `asignacion-por-instanciacion` se coló `task-3-report.md` trackeado por un `git add` amplio del implementador (pendiente de borrado, ver [`docs/DEUDA_TECNICA_Y_PENDIENTES.md`](docs/DEUDA_TECNICA_Y_PENDIENTES.md) "Pendientes de limpieza"). Todo dispatch a implementadores debe exigir explícitamente: stageear solo rutas del task, jamás `git add -A`/`git add .` ni force-add de paths ignorados.

### Config global vs. de proyecto — no se pisan

Confirmado: los config sources se **mergean**, no se reemplazan — orden `Remote → Global (~/.config/opencode/) → Custom → Project (opencode.json de este repo)`, cada nivel posterior gana **solo en las claves que se solapan**. Por eso los agentes/modelos de §9 van en tu config global y no acá: aplican a vos en cualquier repo, sin que un compañero que también use OpenCode en `paceron-backend` herede tus preferencias de modelo o el plugin de superpowers — el `opencode.json` de este repo se mantiene con solo lo que es genuinamente compartido (hoy: el MCP de Mercado Pago).

**Ojo si tenés más de un archivo de config en el mismo directorio** (`opencode.json` y `opencode.jsonc` conviven, por ejemplo): ambos se leen, pero la clave `plugin` (un array) colisionando entre los dos no tiene precedencia clara documentada — más seguro consolidar todo en un solo archivo por directorio que confiar en el merge para esa clave puntual.

### Economía de tokens

- Usar `/opsx-explore` (o el agente `plan`, `edit:deny`) para la fase de pensar/ajustar diseño — recién pasar a `build` cuando el plan esté firme. Ya lo tenés como hábito con `/opsx-propose`/`/opsx-apply`, solo hay que no saltear `explore` cuando el alcance todavía no está cerrado.
- No agregar plugins/MCP nuevos "por las dudas" — este repo ya tiene lo que hace falta a nivel compartido (`mercadopago` MCP, `openspec-*` skills/commands). Cada plugin/MCP nuevo es contexto que se carga en cada sesión; las preferencias personales (superpowers, agentes, otros MCP) van en tu config global, no acá.
