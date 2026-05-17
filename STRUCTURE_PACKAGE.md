# Estructura del Package — Plantilla de Arquitectura

Este documento describe la estructura de carpetas y la responsabilidad de cada una, sin incluir archivos de negocio específicos. Es la plantilla base del scaffolding.

```
simple-arq-golang/
│
├── ci/                              # Artefactos de CI/CD (build, coverage, scripts)
│
├── .agentics/                       # Documentación para agentes de IA
│
├── cmd/api/                         ← Raíz del ejecutable Go
│   ├── main.go                      # Punto de entrada. Solo llama a app.StartApp()
│   ├── docs.go                      # Metadatos globales de Swagger (título, versión, contacto)
│   │
│   ├── docs/                        # Documentación Swagger GENERADA (no editar)
│   │   ├── docs.go
│   │   ├── swagger.json
│   │   └── swagger.yaml
│   │
│   ├── app/                         # Bootstrap e inyección de dependencias
│   │   ├── app.go                   # Construye todas las dependencias y las inyecta
│   │   ├── router.go                # Arranca el server Gin en :8080
│   │   ├── url_mappings.go          # Define las rutas HTTP → controllers
│   │   └── middleware.go            # Middlewares globales (Request ID, etc.)
│   │
│   ├── config/                      # Configuración externalizada
│   │   ├── config.go                # Variables de entorno (DB, environment scope)
│   │   ├── properties.go            # Carga archivos .properties por ambiente
│   │   └── properties/              # Archivos application-{local|test|stage|prod}.properties
│   │
│   ├── constants/                   # Constantes compartidas (nombres de métricas, etc.)
│   │
│   ├── controllers/                 # MANEJADORES HTTP (capas más delgadas posibles)
│   │   └── ping_controller.go       # Health check — único controller obligatorio
│   │
│   ├── delegates/                   # ORQUESTACIÓN entre servicios
│   │   # Aquí van structs que combinan 2+ servicios sin acoplarlos
│   │
│   ├── services/                    # LÓGICA DE NEGOCIO
│   │   # Interfaces + implementaciones. Cada servicio inyecta DAOs o RestClients.
│   │
│   ├── daos/                        # ACCESO A BASE DE DATOS (GORM)
│   │   # Interfaces + implementaciones. Solo hablan con la DB.
│   │
│   ├── restclients/                 # CLIENTES DE APIs EXTERNAS
│   │   # Un subdirectorio por API externa. Usan httpclient de infraestructura.
│   │
│   ├── domains/                     # MODELOS DE DOMINIO (DTOs)
│   │   ├── apierror/                # Estructura única para errores HTTP
│   │   └── dbs/                     # Modelos GORM que mapean tablas de la DB
│   │   # Agregar un subdirectorio por nuevo dominio (user/, weather/, etc.)
│   │
│   ├── infrastructure/              # HERRAMIENTAS TRANSVERSALES REUTILIZABLES
│   │   ├── customlogger/            # Logger estructurado (logrus)
│   │   ├── httpclient/              # Cliente HTTP genérico con retry + circuit breaker
│   │   ├── httputils/               # Utilidades HTTP (parseo de headers, status codes)
│   │   └── postgresdb/              # Conexión a PostgreSQL con GORM
│   │
│   ├── metrics/                     # Helpers de métricas (Datadog, Prometheus, etc.)
│   │
│   ├── testutils/                   # Utilidades para tests (mocks, helpers)
│   │
│   └── utils/                       # Utilidades generales (validación, transformación)
│
├── go.mod                           # Dependencias del módulo Go
├── go.sum                           # Checksums de dependencias
├── .gitignore
└── README.md
```

---

## Responsabilidades de cada carpeta

### `ci/`
**Artefactos de CI/CD.** Almacena scripts de build, reportes de cobertura de tests, configuraciones de pipelines (GitHub Actions, Jenkins, etc.). La subcarpeta `test_coverage/` está en `.gitignore` por ser contenido generado.

### `.agentics/`
**Contexto para asistentes de IA.** Documentación en markdown que define las reglas del proyecto (convenciones, arquitectura, flujo de trabajo, glosario). Cualquier agente de IA que lea esto puede entender cómo contribuir sin romper la arquitectura.

### `cmd/api/main.go`
**Punto de entrada.** Es intencionalmente mínimo: importa el package `app` y ejecuta `app.StartApp()`. No contiene lógica alguna.

### `cmd/api/docs.go`
**Metadatos Swagger.** Define título, versión, descripción, contacto y licencia de la API. Swaggo lee esto para generar la documentación interactiva.

### `cmd/api/docs/`
**Documentación Swagger generada automáticamente.** NO se edita manualmente. Se regenera con:
```bash
swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs
```

### `cmd/api/app/`
**Corazón del bootstrap.** Aquí ocurre la inyección de dependencias:
- `app.go`: construye cada capa (DAO → Service → Delegate → Controller) y las inyecta. Es el único lugar del sistema que conoce todas las implementaciones concretas.
- `router.go`: crea el engine Gin, llama a `NewApplication()`, registra rutas y arranca el server.
- `url_mappings.go`: mapeo explícito de cada endpoint a su controller.
- `middleware.go`: middlewares globales (generación de Request ID, logging, recovery).

### `cmd/api/config/`
**Configuración externalizada.** Separa los valores que cambian por ambiente del código fuente:
- `config.go`: lee variables de entorno para DB y scope.
- `properties.go`: parsea archivos `.properties` con configuración de RestClients (base URL, timeout, reintentos).
- `properties/`: un archivo por ambiente (`local`, `test`, `stage`, `prod`).

### `cmd/api/constants/`
**Valores fijos compartidos.** Strings, códigos, nombres de métricas que se usan en múltiples packages. Centralizarlos evita duplicación y errores de tipeo.

### `cmd/api/controllers/`
**Capa HTTP (la más delgada).** Cada controller recibe un request, valida los parámetros de entrada, llama al service o delegate correspondiente, y devuelve la respuesta HTTP.

⚠️ Un controller NUNCA debe contener lógica de negocio, llamar a DAOs o RestClients directamente.

El `ping_controller.go` es el único controller obligatorio (health check). Los demás se crean por cada funcionalidad.

### `cmd/api/delegates/`
**Orquestadores entre servicios.** Cuando un endpoint necesita combinar datos de dos o más servicios, se crea un delegate en lugar de importar un service dentro de otro. Esto mantiene los servicios desacoplados y permite testearlos de forma independiente.

### `cmd/api/services/`
**Lógica de negocio.** Cada servicio:
- Tiene una **interfaz** pública y una **implementación** privada
- Recibe sus dependencias por constructor (DAOs o RestClients)
- No conoce HTTP, no importa Gin
- Contiene validaciones de negocio, transformaciones, cálculos

### `cmd/api/daos/`
**Acceso a base de datos.** Usa GORM para consultar y persistir datos. Cada DAO:
- Tiene interfaz + implementación
- Recibe `*gorm.DB` por constructor
- Solo es inyectado en Services

### `cmd/api/restclients/`
**Clientes para APIs externas.** Un subdirectorio por cada proveedor externo. Cada cliente:
- Usa el `httpclient.Client` genérico de infraestructura
- Sabe armar la request específica del proveedor (path, headers, query params)
- Sabe parsear la response del proveedor

### `cmd/api/domains/`
**Modelos de datos (DTOs).** Structs sin lógica:
- `apierror/`: estructura uniforme para todas las respuestas de error HTTP.
- `dbs/`: modelos GORM que representan tablas de la base de datos.
- Por cada nuevo dominio se agrega un subdirectorio (ej: `user/`, `weather/`).

### `cmd/api/infrastructure/`
**Herramientas transversales.** Código reutilizable que no pertenece a un dominio específico:
- `customlogger/`: logger estructurado con soporte de tags contextuales. Reemplaza `fmt.Println`.
- `httpclient/`: cliente HTTP con timeout, reintentos automáticos, circuit breaker y telemetría.
- `httputils/`: funciones auxiliares para manejo de HTTP (status codes, headers).
- `postgresdb/`: inicializa la conexión a PostgreSQL vía GORM.

### `cmd/api/metrics/`
**Sistema de métricas.** Helpers para enviar métricas a sistemas externos (Datadog, New Relic, Prometheus). Actualmente con implementación por consola, diseñado para ser reemplazado.

### `cmd/api/testutils/`
**Utilidades para tests.** Helpers reutilizables: creación de contextos Gin falsos, mocks de requests HTTP, carga de JSON desde archivos.

### `cmd/api/utils/`
**Funciones utilitarias generales.** Operaciones que no pertenecen a un dominio: conversiones de tipos, validaciones de formato, búsqueda en slices.

---

## Reglas de dependencia entre capas

```
Controllers → Delegates → Services → DAOs / RestClients → Infrastructure
Controllers → Services → DAOs / RestClients → Infrastructure
```

- Una capa solo puede depender de la capa inmediatamente inferior
- Nunca una capa superior importa a otra del mismo nivel (ej: Service no importa otro Service)
- Infrastructure es la base: cualquier capa puede usarla
- `domains/` es transversal: cualquier capa puede importar sus DTOs
