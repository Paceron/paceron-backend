# Paceron Backend — Guía de trabajo

Convenciones de workflow, git y decisiones para trabajar en este repo — humanos y agentes de IA por igual. **Si tomás una decisión relevante para el equipo (workflow, arquitectura, configuración de proyecto), reflejala acá** para que aplique a todos, no solo a la sesión donde se decidió.

Esto incluye, sin limitarse a: cambios de contexto relevantes en backend o frontend (ej. el frontend depende de un modelo de roles que hoy no existe acá), incorporar una tecnología/librería nueva de peso, o configuración a nivel proyecto que afecte a todo el equipo. Si en el momento no amerita su propia sección, al menos dejar una línea en "Quirks conocidos" — mejor una nota corta que nada.

## Stack

Go 1.26 + Gin (HTTP) + GORM (ORM sobre PostgreSQL/Supabase) + JWT (`golang-jwt/jwt`) + Swagger (`swaggo/swag`). Arquitectura en capas: Controllers → Delegates → Services → DAOs/RestClients → Infrastructure (ver diagrama completo en [`README.md`](README.md)). Frontend separado (Expo/React Native, otro repo), ver sección Frontend.

Documentación técnica detallada ya existe en [`.agentics/`](.agentics/) (en inglés) y en `cmd/api/docs/documentationdetail/` (en español) — este archivo **no la duplica**, es la capa de convenciones de trabajo/git sobre esa base:

- [`.agentics/CONVENTIONS.md`](.agentics/CONVENTIONS.md) — convenciones de código, capas, qué no está permitido (ej. service-to-service imports, DAO directo desde controller).
- [`.agentics/STRUCTURE.md`](.agentics/STRUCTURE.md) — estructura de carpetas.
- [`.agentics/WORKFLOW.md`](.agentics/WORKFLOW.md) — cómo correr, testear, buildear, regenerar swagger, agregar una feature paso a paso.
- [`.agentics/GLOSSARY.md`](.agentics/GLOSSARY.md) — glosario de términos del dominio.
- [`openspec/`](openspec/) — spec-driven development ya configurado en el repo (usar este esquema, no inventar uno nuevo tipo `docs/superpowers/`).

## Workflow de branches y PRs

- **Rama base:** `develop`. Producción es `master`, se llega ahí vía release (`release/<version>` → PR manual a `master`).
- **Nomenclatura de rama:** `feature/<kebab-case>`, `fix/<kebab-case>`, `chore/<kebab-case>` — siempre creada desde `develop` actualizado. Un cambio = una rama = un tema. No mezclar en la misma rama cosas sin relación (ej. no meter un fix de CORS en la misma rama que un setup de CI).
- **Ciclo de PR:** al hacer push de una rama `feature/*`, `fix/*` o `chore/*`, un workflow de GitHub Actions (`auto-pr.yml`) crea automáticamente una PR en draft hacia `develop` con título/descripción placeholder. Se actualiza título y descripción (ver formato abajo), se marca como lista (`gh pr ready`), se espera CI verde (`ci.yml`), y se mergea. Después: `git checkout develop && git pull && git branch -d <rama> && git remote prune origin`.
- **Quién corre los comandos git:** a elección de cada dev. El agente de IA puede ejecutarlos directamente, o armar el bloque de comandos para que el desarrollador lo corra él mismo. Ninguna de las dos es "la forma correcta" — se acuerda con quien esté trabajando.
- Hoy en la práctica solo se usa `feature/*` → `develop`. El modelo completo (`release/*`, `hotfix/*`, `backport/*`) está soportado por los workflows pero todavía no se usó — si empieza a usarse, documentar acá lo que se aprenda del flujo real.

### Mensajes de commit

Formato [Conventional Commits](https://www.conventionalcommits.org/): `tipo(alcance): resumen corto en imperativo`. Tipos usados: `feat`, `fix`, `docs`, `refactor`, `chore`.

Preferir simple: **el subject alcanza en la mayoría de los casos.** Agregar cuerpo (1-2 oraciones) solo cuando el "por qué" no sea obvio desde el diff — no narrar el "qué" (el diff ya lo muestra).

```
# Bien — subject solo, cambio mecánico y autoexplicativo
feat(cors): sumar dominios de Vercel a origenes permitidos

# Bien — cuerpo corto, explica un "por qué" no obvio
fix(auth): invalidar refresh token en logout, no solo el access token

El refresh token sobrevivía al logout y permitía reautenticar sin login.

# Evitar — cuerpo largo narrando el "qué", eso ya está en el diff
feat(user): nuevo endpoint de actualización de estado

Agrega PATCH /users/:id/status que permite cambiar el estado del
usuario entre active/inactive/pause/blocked/suspended...
[3 párrafos más]
```

### Formato de PR

**Título:** mismo estilo que el commit principal — `tipo(alcance): resumen corto`.

**Descripción — corta y concreta:**

```markdown
## Qué cambió
- Bullet corto por cambio, no párrafos.
- Otro bullet.

Spec: `openspec/changes/<nombre>/` ← SOLO si esta rama usó una spec de OpenSpec

## Cómo probarlo
Pasos mínimos para confirmar que funciona.

`go test ./...` → todo verde.
```

## Cuándo usar OpenSpec

El repo ya tiene `openspec/` configurado para spec-driven development (ver [`SETUP.md`](SETUP.md) sección OpenSpec). Regla práctica:

| Tamaño del cambio | Spec de OpenSpec |
|---|---|
| Retoque/corrección chica (1-3 archivos, sin decisión de arquitectura) | No — alcanza con charlarlo y aprobar en el chat |
| Feature nueva, endpoint nuevo, cambio de modelo/schema | Sí |
| Cambio grande o que afecta varias capas con decisiones no triviales | Sí |

Sea cual sea el tamaño: **siempre crear la rama dedicada antes de tocar código**, incluso si se saltea la spec — no quedan commits sueltos en `develop`.

## Testing

`go test ./...` corre sobre archivos `*_test.go` co-ubicados con el código que testean (convención `testify`). No hay base de datos real en CI — los tests que tocan capas con DB usan structs vacíos/mocks (ver `cmd/api/daos/user_dao_test.go`), no una conexión Postgres viva. Antes de mergear, la suite completa debe estar en verde (`ci.yml` la corre automáticamente en cada push/PR).

## CORS

`CORSMiddleware()` en [`cmd/api/app/middleware.go`](cmd/api/app/middleware.go) lee `CORS_ALLOWED_ORIGINS` (env var, orígenes separados por coma). Si no está seteada, cae a una lista default hardcodeada en el código (hoy incluye localhost de desarrollo + los dominios de Vercel del frontend). En producción (Render, ver [`render.yaml`](render.yaml)) se configura explícitamente vía esa env var — al agregar un nuevo dominio de frontend, actualizar **ambos** lugares (el fallback en código y `render.yaml`) para que quede documentado en el repo, no solo en el dashboard de Render.

## Frontend

- Repo separado (Expo/React Native + React Native Web), no vive en este working directory, lo mantiene otro miembro del equipo.
- Se comunica vía REST, ver tabla de endpoints en [`README.md`](README.md).
- Deploy: producción en Vercel (`https://paceron-frontend.vercel.app`), preview de `develop` en `https://paceron-frontend-git-develop-paceron.vercel.app` — ambos dominios deben estar en `CORS_ALLOWED_ORIGINS`.

## Quirks conocidos

- `render.yaml` tiene `branch: main`, pero el repo no tiene rama `main` (usa `master`/`develop`) — revisar si esto es intencional o un desalineamiento antes de tocar el deploy config.
- El deploy en Render tiene cold-start de ~20-25s en la primera request tras inactividad (plan free) — no es un error real si el backend "no responde" al toque.
