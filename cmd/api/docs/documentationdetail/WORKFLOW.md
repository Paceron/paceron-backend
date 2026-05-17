# Flujo de Trabajo

## Ejecutar

```bash
go run cmd/api/main.go
# Inicia en :8080
```

## Tests

```bash
go test ./...            # Todos los tests
go test ./cmd/api/...    # Solo tests de la API
go test -v ./cmd/api/services/   # Verboso, package específico
```

## Build & Vet

```bash
go build ./...
go vet ./...
```

## Dependencias

```bash
go mod tidy    # Limpiar go.mod + go.sum
go get github.com/ejemplo/pkg@v1.0.0   # Agregar dependencia
```

## Swagger

```bash
# Generar docs después de cambiar anotaciones
swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs

# Ver en http://localhost:8080/swagger/index.html
```

## Agregar una nueva funcionalidad

1. **Modelo de dominio** → `domains/<nombre>/` (DTOs)
2. **RestClient o DAO** → `restclients/<nombre>/` o `daos/` (interfaz + impl)
3. **Service** → `services/` (interfaz + impl, inyectar DAO/Client)
4. **Delegate** (opcional) → `delegates/` si se necesitan múltiples servicios
5. **Controller** → `controllers/` (validar + delegar al service)
6. **Cablear** → `app/app.go` (construir + inyectar)
7. **Ruta** → `app/url_mappings.go` (agregar endpoint)
8. **Swagger** → Agregar anotaciones, regenerar docs
9. **Properties** → `config/properties/` si se necesita config de restclient
10. **Tests** → Escribir tests con interfaces mockeadas

## Variables de entorno

| Variable | Descripción | Valor por defecto |
|----------|-------------|-------------------|
| `environment` | Entorno de ejecución | `local` |
| `db_host` | Host de Postgres | - |
| `db_port` | Puerto de Postgres | - |
| `db_user` | Usuario de Postgres | - |
| `db_password` | Contraseña de Postgres | - |
| `db_name` | Base de datos Postgres | - |

Para depuración con VS Code: usar `.vscode/launch.json` con bloque `env`.

## Errores comunes

- Dos directorios customlogger: usar solo `infrastructure/customlogger/`
- El nombre del package debe coincidir con el nombre del directorio
- No usar `fmt.Println` — usar `customlogger`
- No importar services desde otros services — usar delegates
- Los archivos `.properties` deben existir para el entorno actual
