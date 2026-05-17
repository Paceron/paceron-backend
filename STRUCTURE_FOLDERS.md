# Estructura de Carpetas — Mapa Arquitectónico

```
simple-arq-golang/
│
├── ci/                          # CI/CD
│
├── .agentics/                   # Contexto para asistentes de IA
│
├── cmd/
│   └── api/
│       ├── docs/                # Swagger generado (no tocar)
│       │   └── documentationdetail/  # Documentación en español
│       │
│       ├── app/                 # Bootstrap + DI
│       ├── config/              # Configuración externalizada
│       │   └── properties/      # Archivos .properties por entorno
│       │
│       ├── constants/           # Constantes globales
│       ├── controllers/         # Handlers HTTP
│       ├── delegates/           # Orquestación entre servicios
│       ├── services/            # Lógica de negocio
│       ├── daos/                # Acceso a base de datos
│       ├── restclients/         # Clientes de APIs externas
│       ├── domains/             # DTOs y modelos
│       │   ├── apierror/        # Error HTTP uniforme
│       │   └── dbs/             # Modelos de base de datos
│       │
│       ├── infrastructure/      # Herramientas transversales
│       │   ├── customlogger/    # Logger estructurado
│       │   ├── httpclient/      # Cliente HTTP genérico
│       │   ├── httputils/       # Utilidades HTTP
│       │   └── postgresdb/      # Conexión a PostgreSQL
│       │
│       ├── metrics/             # Métricas y telemetría
│       ├── testutils/           # Utilidades para tests
│       └── utils/               # Funciones utilitarias
```

---

## Descripción de cada carpeta

### `ci/` — Integración y despliegue continuo
Almacena todo lo necesario para pipelines automatizados: scripts de build, reportes de cobertura (`test_coverage/`), configuraciones de GitHub Actions, Jenkins o similares. Es el puente entre el código y su automatización.

**¿Qué va aquí?** Scripts `.sh`, archivos YAML de pipelines, Makefiles, configuraciones de Docker.

**Contrato:** Todo lo que hay acá es ejecutable por un sistema de CI. No contiene lógica de la aplicación.

---

### `.agentics/` — Contexto para asistentes de IA
Documentación en markdown diseñada para ser leída por agentes de IA (Claude, GPT, etc.) antes de tocar el código. Define reglas de arquitectura, convenciones de código, flujo de trabajo y glosario del scaffolding.

**Propósito:** Que cualquier agente de IA entienda cómo contribuir sin romper la arquitectura.

**Convención:** Archivos en markdown, uno por tema (STRUCTURE, CONVENTIONS, WORKFLOW, GLOSSARY).

---

### `cmd/api/` — Raíz del binario
Todo lo que está dentro de `cmd/api/` compila para formar un solo ejecutable Go. Es el límite del sistema: afuera quedan configuraciones de CI, documentación externa, etc.

**No es un package Go.** `main.go` y `docs.go` están en `package main`. El resto de carpetas dentro de `cmd/api/` son packages independientes.

---

### `cmd/api/docs/` — Documentación Swagger generada
Contiene `docs.go`, `swagger.json` y `swagger.yaml` generados automáticamente por `swag init`. Se sirve en `/swagger/index.html`.

**⚠️ No se edita manualmente.** Cualquier cambio se pierde al regenerar. Para modificar la documentación, se cambian las anotaciones en los controllers y se regenera.

**Regeneración:**
```bash
swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs
```

---

### `cmd/api/docs/documentationdetail/` — Documentación del scaffolding en español
Copias traducidas de los archivos de `.agentics/`, pero en español. Pensada para desarrolladores junior que necesitan entender el proyecto sin barrera de idioma.

---

### `cmd/api/app/` — Bootstrap e inyección de dependencias
**Es el corazón del wiring.** Aquí se construyen todos los objetos del sistema y se inyectan unos en otros. Ningún package fuera de `app/` sabe cómo construir sus dependencias.

**Responsabilidades:**
- Crear el engine Gin, configurar logger y middleware global
- Construir cada capa en orden (DAO → Service → Delegate → Controller)
- Inyectar dependencias por constructor
- Registrar rutas HTTP → controllers
- Arrancar el servidor

**Regla de oro:** Es el único lugar del sistema que conoce todas las implementaciones concretas. El resto del código solo conoce interfaces.

---

### `cmd/api/config/` — Configuración externalizada
Separa los valores que cambian por ambiente del código fuente. Evita hardcodear URLs, timeouts, credenciales o cualquier valor que difiera entre local, test, stage y prod.

**Dos mecanismos:**
- **Variables de entorno** (`config.go`): para conexión a DB, scope del ambiente.
- **Archivos .properties** (`properties.go` + `properties/`): para configuración de RestClients (base URL, timeout, reintentos).

**Cuándo agregar algo aquí:** Cuando un valor cambia entre ambientes o cuando podría cambiar sin necesidad de recompilar.

---

### `cmd/api/config/properties/` — Archivos de configuración por ambiente
Contiene `application-{local,test,stage,prod}.properties`. Cada archivo define la configuración específica de ese entorno.

**Formato:** clave=valor (simple parser, sin dependencias externas).

**Carga automática:** según la variable de entorno `environment`, se carga el archivo correspondiente.

---

### `cmd/api/constants/` — Valores fijos compartidos
Strings, códigos, nombres de métricas, etiquetas que se usan en múltiples packages. Centralizarlos evita:
- Duplicación de strings mágicos
- Errores de tipeo
- Dificultad para encontrar dónde se define un valor

**Cuándo crear una constante aquí:** Cuando el mismo valor aparece en 2+ lugares o cuando un valor tiene significado de negocio pero no cambia por ambiente.

---

### `cmd/api/controllers/` — Handlers HTTP
Es la **capa más delgada** del sistema. Cada controller:
1. Recibe un request HTTP
2. Valida los parámetros de entrada (formato, tipos, rangos)
3. Llama al Service o Delegate correspondiente
4. Devuelve la respuesta HTTP (JSON)

**⚠️ Reglas estrictas:**
- No contiene lógica de negocio
- No llama a DAOs ni RestClients directamente
- No usa `fmt.Println`
- No realiza transformaciones complejas de datos

**Único controller obligatorio:** `PingController` (health check `GET /ping`).

**Patrón:** `nombreController.GetX` → valida → `service.X(ctx, params)` → arma response.

---

### `cmd/api/delegates/` — Orquestación entre servicios
Resuelve el problema del **acoplamiento horizontal**. Cuando un Service necesita datos de otro Service, la tentación es importarlo directamente. Eso rompe la arquitectura porque crea dependencias entre pares del mismo nivel.

**Solución:** El Delegate conoce ambos Services y los orquesta. El Controller inyecta el Delegate, no los Services por separado.

**¿Cuándo crear un Delegate?**
- Cuando un endpoint necesita datos de 2 o más servicios
- Cuando una operación requiere una secuencia de pasos que cruza dominios

**¿Qué NO hace un Delegate?** Lógica de negocio. Solo coordina llamadas.

---

### `cmd/api/services/` — Lógica de negocio
Es la **capa más importante** del proyecto. Aquí vive toda la lógica que hace que la aplicación tenga valor: cálculos, validaciones de negocio, transformaciones, flujos de decisión.

**Características:**
- Tiene una **interfaz pública** (ej: `UserServiceInterface`) y una **implementación privada** (ej: `userService`)
- Recibe dependencias por constructor (DAOs o RestClients)
- No conoce HTTP (no importa Gin, no recibe `*gin.Context`)
- Es testeable de forma aislada (las dependencias se mockean por la interfaz)

**Lo que NO debe hacer un Service:**
- Importar otro Service (usar Delegate)
- Acceder a la base de datos directamente (usar DAO)
- Hacer requests HTTP directamente (usar RestClient)
- Escribir a la respuesta HTTP

---

### `cmd/api/daos/` — Acceso a base de datos
Capa que encapsula todas las consultas a la base de datos usando GORM.

**Características:**
- Interfaz pública + implementación privada
- Recibe `*gorm.DB` por constructor
- Cada método representa una operación atómica contra la DB
- Solo es inyectado en Services

**Lo que NO hace un DAO:**
- Lógica de negocio
- Llamadas a APIs externas
- Transformaciones complejas

---

### `cmd/api/restclients/` — Clientes de APIs externas
Un subdirectorio por cada proveedor externo que consumimos. Cada cliente:
- Usa `httpclient.Client` (genérico de infraestructura)
- Sabe armar la request específica del proveedor (path, headers, query params)
- Sabe parsear la response del proveedor
- Expone una interfaz para que los Services la consuman

**Diferencia clave con DAOs:** DAOs hablan con DB (protocolo interno), RestClients hablan con APIs HTTP (protocolo externo).

**Patrón de creación:** `restclients/micliente/client.go` con interfaz `MiclienteInterface`.

---

### `cmd/api/domains/` — Modelos de dominio
**Data Transfer Objects (DTOs).** Structs puras sin lógica que representan los datos que entran y salen del sistema.

**Subcarpetas fijas:**
- `apierror/`: estructura única `APIError` para todas las respuestas de error HTTP. Garantiza que el cliente siempre reciba el mismo formato de error.
- `dbs/`: modelos GORM que mapean tablas de la base de datos.

**Subcarpetas por dominio:** Se crea una carpeta por cada nuevo dominio de negocio (ej: `user/`, `weather/`, `payment/`). Cada una contiene los DTOs de request y response de ese dominio.

**Regla:** Los structs aquí no tienen métodos, solo campos con tags (JSON, GORM, validate).

---

### `cmd/api/infrastructure/` — Herramientas transversales
Código reutilizable que cualquier capa puede usar y que no pertenece a un dominio específico. Es la **base de la pirámide** de dependencias.

**Subcarpetas:**

**`customlogger/`** — Logger estructurado basado en logrus. Tiene:
- Logger principal con soporte de tags contextuales
- Adapter para que `httpclient` pueda loguear con el mismo logger
- Reemplaza a `fmt.Println` en todo el sistema

**`httpclient/`** — Cliente HTTP genérico configurable con:
- Timeout
- Reintentos automáticos (exponential backoff)
- Circuit breaker
- Telemetría
- Logger inyectable
Se inyecta en RestClients, no se usa directamente desde Services o Controllers.

**`httputils/`** — Utilidades para manejo HTTP: parseo de status codes, headers, construcción de respuestas.

**`postgresdb/`** — Inicializa la conexión a PostgreSQL usando GORM. Lee la configuración de `config.MyDB`.

---

### `cmd/api/metrics/` — Métricas y telemetría
Helpers para enviar métricas a sistemas externos de monitoreo (Datadog, New Relic, Prometheus, etc.). La implementación actual imprime por consola, pero la interfaz está diseñada para ser reemplazada por un proveedor real sin modificar el código que las usa.

---

### `cmd/api/testutils/` — Utilidades para tests
Helpers reutilizables que facilitan la escritura de tests:
- Creación de contextos Gin falsos
- Mockeo de requests HTTP
- Carga de JSON desde archivos mock
- Aserciones comunes

**No contiene tests en sí mismo.** Contiene herramientas que los tests usan.

---

### `cmd/api/utils/` — Funciones utilitarias
Operaciones genéricas que no pertenecen a un dominio:
- `Contains(slice, elemento)` — búsqueda en slices
- `StringToInt64(s)` / `Int64ToString(n)` — conversiones
- `IsPositiveInteger(s)` / `ParseInt64(s)` — validaciones de formato
- `TransformUtils` — transformaciones de datos

**Cuándo crear una función aquí:** Cuando la misma operación se necesita en 2+ lugares y no está relacionada con un dominio específico.

---

## Mapa de dependencias entre carpetas

```
controllers → delegates → services → daos
                                    → restclients → infrastructure/httpclient
controllers → services → daos
                        → restclients → infrastructure/httpclient

infrastructure/*  ← cualquier capa puede usarlo
domains/*         ← cualquier capa puede usarlo (solo DTOs)
```

**Reglas:**
1. Una carpeta solo puede depender de la carpeta inmediatamente inferior
2. Ninguna carpeta del mismo nivel se importa entre sí
3. `infrastructure/` es la base: todos pueden usarla
4. `domains/` es transversal: todos pueden importar sus DTOs
5. `config/` se usa solo en `app/` y en `infrastructure/` para inicializar conexiones
