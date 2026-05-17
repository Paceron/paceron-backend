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

1. **Domain model** → `domains/<name>/` (DTOs)
2. **RestClient or DAO** → `restclients/<name>/` or `daos/` (interface + impl)
3. **Service** → `services/` (interface + impl, inject DAO/Client)
4. **Delegate** (optional) → `delegates/` if multiple services needed
5. **Controller** → `controllers/` (validate + delegate to service)
6. **Wire** → `app/app.go` (construct + inject)
7. **Route** → `app/url_mappings.go` (add endpoint)
8. **Swagger** → Add annotations, regenerate docs
9. **Properties** → `config/properties/` if restclient config needed
10. **Tests** → Write tests with mocked interfaces

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
