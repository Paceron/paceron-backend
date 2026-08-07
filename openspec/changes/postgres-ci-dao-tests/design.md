## Context

`daos` era el paquete con más código real y menos coverage del proyecto (4.1%, ~90 métodos across 14 archivos, todos GORM CRUD/query-building relativamente simples pero sin ejercitar). El resto del proyecto (`services`/`controllers`) ya está en 90%+ usando mocks — pero un DAO no tiene mucha lógica propia más allá de la query en sí, así que mockear su única dependencia (GORM/SQL) deja muy poco que testear de verdad.

## Goals / Non-Goals

**Goals:**
- Cruzar el 80% de coverage total del proyecto de forma genuina (comportamiento real verificado), no maquillada.
- Que el gate (`threshold.total: 80`) sea sostenible: `go test ./...` sigue funcionando para cualquier dev sin Docker instalado.
- Que el costo de CI sea bajo (container efímero, no un servicio persistente).

**Non-Goals:**
- 100% de coverage en `daos` — no se persiguen ramas de error de conexión rota u otros casos no reproducibles sin manipular la conexión a mitad de test.
- Tocar el código de producción de `daos/*.go` — este cambio es puramente de tests + infra de CI.
- Resolver el tema de ambientes de Supabase test/prod (separado, próxima iniciativa).

## Decisions

### 1. Postgres real en CI, no sqlmock ni SQLite

**Evaluadas las 3 alternativas explícitamente antes de decidir:**

- **sqlmock**: requiere predecir el SQL exacto que GORM genera (regex por query), frágil ante cambios de query/versión de GORM, y termina testeando "¿armé la llamada que yo mismo esperaba?" en vez de "¿el comportamiento es correcto?". No detecta problemas reales de Postgres.
- **SQLite en memoria**: diverge de Postgres en funciones que el proyecto ya usa activamente — `ILIKE` (usado en `user_dao.SearchActive`) no existe en SQLite, forzaría reescribir la query solo para el test, dando falsa confianza (el test pasa, el comportamiento real en prod podría no).
- **Postgres real** (elegida): única opción que verifica comportamiento real. Costo bajo: `postgres:16-alpine` (~294MB, tag oficial más liviano), container efímero por job de CI, arranque ~5-15s vía healthcheck, sin persistencia entre corridas.

### 2. Transacción por test, no truncar tablas

**Por qué**: con `db.Begin()` al inicio de cada test y `tx.Rollback()` en `t.Cleanup`, cada test ve una base limpia sin necesidad de:
- Truncar tablas entre tests (más lento, requiere orquestar orden por FKs).
- Preocuparse por colisiones de índices únicos entre tests distintos — una transacción no confirmada nunca es visible para otra (aislamiento MVCC de Postgres), así que dos tests pueden usar el mismo email/nombre sin chocar.

Costo: los DAOs reciben la transacción como si fuera el `*gorm.DB` normal (mismo tipo), sin cambios en su código de producción.

### 3. `testutils.SetupTestDB` reusa `postgresdb.ConfigDB`, no duplica la conexión

**Por qué**: `ConfigDB` ya hace `AutoMigrate` con la lista real de modelos que usa la app en arranque. Si `testutils` armara su propia conexión+migración, el schema de test podría divergir silenciosamente del de producción (un modelo nuevo agregado a `AutoMigrate` y olvidado en el test setup). Reusar la misma función garantiza que siempre estén sincronizados.

### 4. Skip automático sin `TEST_DB_HOST`, no Docker obligatorio para todos

**Por qué**: forzar Docker para cualquier `go test ./...` local rompería el flujo de cualquier dev que solo esté tocando un controller o service sin relación con DAOs. Mismo patrón ya usado en el repo (`mailer_test.go`'s `TestSendEmail_RealEmail_Integration`, que skipea sin credenciales SMTP) — consistencia con una convención ya establecida, no una nueva.

### 5. Container en :5433 local, no :5432

**Por qué**: evita chocar con un Postgres local que el dev ya pueda tener corriendo (ej. para otro proyecto, o Supabase CLI local) en el puerto default.

### 6. `threshold.total: 80`, no 85 (el número real actual)

**Por qué**: el número real hoy es ~85%, pero fijar el umbral ahí no deja margen — la primera fluctuación menor (un archivo nuevo con 2 líneas sin cubrir) rompería CI por ruido. 80 es el valor que ya estaba documentado como plan en `CLAUDE.md` desde antes de esta iniciativa, y deja ~5 puntos de colchón.

## Risks / Trade-offs

- **CI un poco más lento**: +15-30s por el arranque del container Postgres. Aceptado — bajo comparado con el beneficio de tests reales.
- **Local dev con Docker es opcional pero recomendado**: un dev que solo corre `go test ./...` sin `test-db-up` no ve el coverage real de `daos` reflejado localmente, solo en CI. Aceptado — mismo patrón que ya existía para SMTP.
- **`postgres:16-alpine` fijo, no sigue la versión exacta de Supabase**: no crítico — el SQL usado (`ILIKE`, `IN`, updates simples) es estándar entre versiones recientes de Postgres. Si en el futuro se detecta una divergencia real, se ajusta el tag de la imagen.
