# Paceron Backend — Guía de trabajo (OpenCode)

Equivalente de [`CLAUDE.md`](CLAUDE.md) para sesiones de OpenCode — mismo contenido de fondo, reorganizado para no depender de herramientas específicas de Claude Code (skills, memoria automática entre sesiones, subagentes). Si algo cambia acá, reflejarlo también en `CLAUDE.md` y viceversa — son el mismo conjunto de convenciones para dos agentes distintos trabajando el mismo repo. **Este repo es ahora el que se desarrolla desde OpenCode** — Claude Code quedó reservado para `paceron-frontend` (otro repo), ver §8.

**Si tomás una decisión relevante para el equipo (workflow, arquitectura, configuración de proyecto), reflejala en ambos archivos** (`AGENTS.md` y `CLAUDE.md`) para que aplique a todos, no solo a la sesión donde se decidió.

## 1. Stack

Go 1.26 + Gin (HTTP) + GORM (ORM sobre PostgreSQL/Supabase) + JWT (`golang-jwt/jwt`) + Swagger (`swaggo/swag`). Arquitectura en capas: Controllers → Delegates → Services → DAOs/RestClients → Infrastructure (diagrama completo en [`README.md`](README.md)). Frontend separado (Expo/React Native, otro repo, otro agente — ver §8).

Documentación técnica detallada:

- [`docs/STATE_MACHINES.md`](docs/STATE_MACHINES.md) — estados/transiciones/invariantes por entidad. Fuente de verdad de los valores: `cmd/api/domains/constants/`.
- [`docs/CATALOGO_Y_CALENDARIO.md`](docs/CATALOGO_Y_CALENDARIO.md) — modelo de datos, guards, endpoints del catálogo (`Exercise`/`Session`/`TrainingPlan`) y calendario de grupos (`GroupCalendarDay`). **En proceso de rework, ver §7.**
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

`make coverage` (sin DB, parcial) / `make coverage-with-db` (con DB real, número real — mismo comando que corre `ci.yml`). **`.testcoverage.yml`: `threshold.total: 80`, bloquea merge si baja.** No ajustar el umbral hacia abajo si baja — agregar tests. `exclude.paths` cubre solo paquetes sin lógica real (DTOs puros, constants, swagger autogenerado, `testutils`, `main.go`) — un paquete con lógica real y 0% nunca entra ahí a propósito, para que la deuda sea visible.

## 5. CORS

`CORSMiddleware()` en `cmd/api/app/middleware.go` lee `CORS_ALLOWED_ORIGINS` (env, orígenes separados por coma), cae a una lista default hardcodeada si no está seteada. En producción (Render, `render.yaml`) se configura explícitamente. Al agregar un dominio nuevo, actualizar **ambos lugares** (fallback en código + `render.yaml`, los 2 services: producción y preview de develop).

## 6. Quirks conocidos

- `gh pr edit` puede fallar con `GraphQL: Projects (classic) is being deprecated... (repository.pullRequest.projectCards)` — es un bug del CLI leyendo un campo ajeno, no un error real del edit. Workaround: `gh api repos/<owner>/<repo>/pulls/<n> -X PATCH -f title="..." -f body="..."`.
- El toolchain de Go 1.26 vía `GOTOOLCHAIN=auto` no trae `covdata` — falla `go test -coverprofile` sobre paquetes sin `_test.go`. Por eso `make coverage`/`ci.yml` filtran a paquetes con `TestGoFiles` (`go list -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./... | xargs go test ...`), no `./...` directo.
- `attendances.training_session_id` y `workout_feedback.assigned_session_id`/`assigned_exercise_id` son FK **opacas** (BIGINT > 0, sin constraint de DB) — anticipan tablas/entidades que todavía no existen del todo. Ver §7, `asignacion-por-instanciacion` es probablemente la pieza que las tablas de `workout_feedback` estaban esperando.
- El deploy en Render tiene cold-start de ~20-25s en la primera request tras inactividad (plan free) — no es error real.
- Dos proyectos de Supabase separados (testing/producción) — `master` en Render pega a producción, todo lo demás (`develop`/local) pega a testing por default, sin flag. Detalle: [`docs/ENVIRONMENTS.md`](docs/ENVIRONMENTS.md).

## 7. Trabajo en curso — rework de catálogo/calendario (instanciación)

Decisión de equipo (2026-09-19), spec ya escrita en `openspec/changes/asignacion-por-instanciacion/` (proposal.md/design.md/specs/tasks.md — **leer antes de tocar nada de `services/calendar_service.go`, `services/session_service.go`, `services/exercise_service.go` o `dbs/group_calendar_day.go`**):

- El mecanismo actual (`calendario-asignacion-grupos` D8/D13 + `congelar-ejercicio-en-clon`) calcula, en el momento de editar `Session`/`Exercise` del catálogo, si algún día de calendario ya "cerrado" necesita clonarse para no recibir la edición. Se juzgó demasiado complejo/frágil de razonar y mantener.
- **Se reemplaza por instanciación proactiva**: `Exercise`/`Session`/`TrainingPlan` quedan 100% template. Asignar contenido a un día de calendario crea copias inmutables en tablas propias (`session_instances`/`session_exercise_instances`/`exercise_instances`, separadas del catálogo) en el momento del `save`, no al editar después. `GroupCalendarDay.session_id` pasa a `session_instance_id`.
- El chequeo de "día cerrado" (`isCalendarDayClosed`, misma regla de fecha/horario) se reusa pero cambia de rol: de "trigger de clonado" pasa a "guard de escritura" — no se puede asignar/reasignar/borrar contenido de un día ya cerrado, punto.
- Todo el mecanismo de clonado por divergencia (D8/D13/`congelar-ejercicio-en-clon`) se borra, no convive con el nuevo.
- Ver `openspec/changes/asignacion-por-instanciacion/tasks.md` para el checklist de implementación (todavía no ejecutado al momento de escribir esto).

## 8. Coordinación con Claude Code (paceron-frontend)

Desde 2026-09-19, `paceron-backend` se desarrolla desde OpenCode; Claude Code quedó reservado para `paceron-frontend` (juzgado más complejo). Si algo de este rework (o cualquier cambio de contrato de API) afecta al frontend, avisar explícitamente — no asumir que la sesión de Claude en el otro repo se entera sola. Puntos concretos que el rework de §7 va a romper del lado frontend:
- `PUT /sessions/{id}` pierde `exclude_group_ids`/`clone_name`/`clone_description`.
- El shape de lo que devuelve el calendario puede cambiar si se decide embeber el detalle de la instancia en la respuesta (a definir en implementación, ver `tasks.md` Task 3).
- `docs/BACKEND_CALENDAR_ASSIGNMENTS_SPEC.md` §5 (clonado por divergencia, spec del lado frontend) queda obsoleto una vez implementado esto — coordinar su actualización con quien mantiene ese repo.
