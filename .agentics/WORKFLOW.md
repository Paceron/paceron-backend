# Development Workflow

## Run

```bash
go run cmd/api/main.go
# Starts on :8080
```

## Test

```bash
go test ./...            # All tests
go test ./cmd/api/...    # Only API tests
go test -v ./cmd/api/services/   # Verbose, specific package
```

## Build & Vet

```bash
go build ./...
go vet ./...
```

## Dependencies

```bash
go mod tidy    # Clean up go.mod + go.sum
go get github.com/example/pkg@v1.0.0   # Add dependency
```

## Swagger

```bash
# Generate docs after changing annotations
swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs

# View at http://localhost:8080/swagger/index.html
```

## Adding a new feature

### 1. Spec-driven development (OpenSpec)

Before writing code, define intent using OpenSpec:

```bash
/opsx:propose "descripción del cambio"
```

This creates:
- `openspec/changes/<name>/proposal.md` — qué y por qué
- `openspec/changes/<name>/specs/` — especificaciones detalladas
- `openspec/changes/<name>/design.md` — diseño técnico
- `openspec/changes/<name>/tasks.md` — tareas de implementación

When ready, implement with:

```bash
/opsx:apply
```

### 2. Implementation steps

1. **Domain model** → `domains/<name>/` (DTOs)
3. **RestClient or DAO** → `restclients/<name>/` or `daos/` (interface + impl)
4. **Service** → `services/` (interface + impl, inject DAO/Client)
5. **Delegate** (optional) → `delegates/` if multiple services needed
6. **Controller** → `controllers/` (validate + delegate to service)
7. **Wire** → `app/app.go` (construct + inject)
8. **Route** → `app/url_mappings.go` (add endpoint)
9. **Swagger** → Add annotations, regenerate docs
10. **Properties** → `config/properties/` if restclient config needed
11. **Tests** → Write tests with mocked interfaces

> **Importante:** No saltar pasos. Cada paso debe completarse y verificarse antes de pasar al siguiente. Prohibido el "live coding".

Archive when done:

```bash
/opsx:archive
```

## Environment variables

| Variable | Description | Default |
|----------|-------------|---------|
| `environment` | Environment scope | `local` |
| `db_host` | Postgres host | - |
| `db_port` | Postgres port | - |
| `db_user` | Postgres user | - |
| `db_password` | Postgres password | - |
| `db_name` | Postgres database | - |

For VS Code debugging: use `.vscode/launch.json` with `env` block.

## Common pitfalls

- Two customlogger directories: use only `infrastructure/customlogger/`
- Package name must match directory name
- Don't `fmt.Println` — use `customlogger`
- Don't import services from services → use delegates
- `.properties` files must exist for the current scope
