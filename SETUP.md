# Setup Local — paceron-backend

Guía para desarrollar este proyecto localmente.

## Requisitos

| Herramienta | Versión mínima |
|-------------|----------------|
| Go | 1.26 |
| Node.js | 20.19+ |
| PostgreSQL | 15+ |
| openspec CLI | 1.5.0 |

## Instalación

```bash
# 1. Clonar el repositorio
git clone <repo-url>
cd paceron-backend

# 2. Instalar dependencias Go
go mod tidy

# 3. Instalar OpenSpec CLI (si no está instalado)
npx @fission-ai/openspec --version

# 4. Configurar base de datos PostgreSQL
createdb paceron
# O crear la base de datos manualmente

# 5. Variables de entorno (crear .env en la raíz)
cat > .env << EOF
environment=local
db_host=localhost
db_port=5432
db_user=postgres
db_password=postgres
db_name=paceron
EOF
```

## Ejecutar

```bash
# Iniciar servidor
go run cmd/api/main.go

# Servidor disponible en http://localhost:8080
# Swagger: http://localhost:8080/swagger/index.html
```

## Tests

```bash
go test ./...
go test -v ./cmd/api/...
```

Los tests de `daos` que necesitan Postgres real se skipean solos sin Docker. Para correrlos localmente, ver [`docs/TESTING.md`](docs/TESTING.md) (`make test-db-up` + `make test-with-db`).

## OpenSpec — Spec-Driven Development

Este proyecto usa OpenSpec para spec-driven development.

```bash
# Ver estado de cambios activos
openspec list

# Iniciar un nuevo cambio
# En OpenCode: /opsx:propose "descripcion del cambio"
# En Qwen Code: /opsx:propose "descripcion del cambio"
```

Los artefactos se crean en `openspec/changes/<change-name>/`:
- `proposal.md` — qué y por qué
- `specs/` — especificaciones detalladas
- `design.md` — diseño técnico
- `tasks.md` — tareas de implementación

### Idiomas

- **Comunicación con AI**: español
- **Código**: Go (identificadores en inglés)
- **Documentación técnica**: español

## Configuración de AI

### OpenCode

El proyecto incluye skills y comandos preconfigurados en `.opencode/`.

Skills disponibles:
- `openspec-propose` — crear propuestas
- `openspec-apply-change` — implementar cambios
- `openspec-archive-change` — archivar cambios completados
- `openspec-explore` — modo exploración
- `openspec-sync-specs` — sincronizar especificaciones

### Qwen Code

Configuración equivalente en `.qwen/`.

Comandos slash:
- `/opsx:propose`
- `/opsx:apply`
- `/opsx:archive`
- `/opsx:explore`
- `/opsx:sync`

## Swagger

```bash
# Regenerar documentación Swagger tras modificar anotaciones
swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs
```

## Estructura del proyecto

Ver [`STRUCTURE_FOLDERS.md`](STRUCTURE_FOLDERS.md) y [`STRUCTURE_PACKAGE.md`](STRUCTURE_PACKAGE.md).
