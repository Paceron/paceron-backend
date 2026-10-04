# Proposal

## Objetivo

Hoy el backend no deja editar sesiones de calendario que deberían ser editables, y la
razón es que no tiene idea de qué día es "hoy" para el producto.

`isCalendarDayClosed` (`cmd/api/services/calendar_service.go:380`) calcula el "hoy" con
`now.Location()`, o sea la zona horaria **del proceso**. En Render eso es UTC. El producto
opera en una zona sola (Argentina, UTC-3). A las 22:21 hora de Buenos Aires del 30/09 ya es
01:21 del **1 de octubre** en UTC, y a partir de ahí:

- La sesión del **30/09** se evalúa como fecha pasada → cerrada.
- La sesión del **1/10** se evalúa como "hoy, no presencial" → cerrada.

Las dos cerradas, y el cierre se traduce en `ErrCalendarDayClosed` → **422**. El entrenador
no puede corregir nada de lo que tiene cargado, y no hay forma de distinguir "esta sesión
caducó" de "el servidor está en otro huso".

Este change hace dos cosas: que "hoy" se resuelva contra una zona de referencia explícita del
producto, y que exista una variable de entorno para desactivar el bloqueo de días cerrados
cuando haga falta operar sobre el calendario.

## Alcance

- **Zona de referencia explícita** para el cálculo de "hoy" en el guard de día cerrado, en
  lugar de la zona del proceso. Argentina (UTC-3) como constante del producto.
- **Corregir el segundo desfasaje de la misma función**: la ventana horaria de la
  presencial se arma con `day.PresencialTimeFrom.UTC().Hour()` combinado con
  `now.Location()` — horas UTC en una location local. Hoy eso solo "funciona" por la
  coincidencia de que la location del proceso es UTC.
- **Variable de entorno de bypass** (`CALENDAR_EDITION_UNRESTRICTED`), por defecto en
  `false`, que desactiva el bloqueo en los guard de escritura cuando está en `true`.
- Documentación de la variable en `.env.example` y `render.yaml`.
- Tests que cubran la nueva resolución de zona y el comportamiento del bypass.

## No alcance

- **No se cambia la regla de negocio** de qué es un día cerrado. Sigue siendo: fecha
  pasada cerrada; hoy cerrada salvo presencial con horario todavía no arrancado; futuro
  nunca cerrada. Lo que cambia es **en qué huso se evalúa**, no el criterio.
- **No se agregan endpoints** ni se modifica ninguna forma de request o response. El 422
  sigue siendo 422 y con el mismo cuerpo.
- **No se cambian los filtros de lectura.** Los DAO filtran días en SQL con sus propios
  criterios (y ya normalizan a UTC con `AT TIME ZONE 'UTC'`); ese camino no pasa por
  `isCalendarDayClosed` y no se toca acá.
- **No se toca el frontend.** El bypass vive y se decide acá. Ver "Consecuencia conocida".
- **No se cambia la zona horaria de la sesión de base de datos.** Sigue en UTC por DSN
  (`infrastructure/postgresdb/postgres.go:26`), que es lo correcto para almacenar.

## Consecuencia conocida del bypass

Con `CALENDAR_EDITION_UNRESTRICTED=true` **la API acepta** editar una sesión cerrada. El
frontend, en cambio, tiene su propio `isCalendarDayClosed` advisory
(`utils/calendar-day-closed.js`) que deshabilita el botón de guardar y atenúa el día en el
calendario, y ese código no se cambia en este change. Traducido: el bypass destraba la API,
pero **para usarlo de punta a punta desde la UI hace falta un cambio posterior en el
frontend** que exponga o respete el estado del flag. Queda anotado acá para que no se
confunda "el 422 ya no aparece" con "la UI ya deja editar".

## Métrica de éxito

- Con el servidor en UTC y la hora real de Buenos Aires pasada las 20:00, una sesión de
  **hoy** y una de **mañana** de un grupo con un entrenador autenticado responden `200` al
  `PUT` del día, donde antes respondían `422`.
- Un test cubre el instante crítico: `2026-09-30T22:21:00-03:00` (que en UTC es el 1 de
  octubre) resuelve "hoy" como el **30 de septiembre**.
- Un test cubre que con el bypass ausente la mutación sigue rechazándose con 422, y con el
  bypass presente la misma mutación se acepta.
- La suite completa (`go test ./...`) en verde.

## Capabilities

### New Capabilities

- `calendar-day-closed`: la regla que decide si un día del calendario de un grupo está
  cerrado y por lo tanto no admite escritura, en qué zona horaria se resuelve "hoy", y la
  posibilidad de desactivar el bloqueo por configuración.

### Modified Capabilities

Ninguna. Los cuatro specs existentes (`session-registration-review`, `user-bank-alias`,
`workout-feedback`, `workout-feedback-gps-points`) no tocan el calendario.

## Impact

**Código**

- `cmd/api/services/calendar_service.go` — `isCalendarDayClosed` y el helper
  `closedDayForRequest`, más los ~10 call sites de escritura que pasan `time.Now()`.
- `cmd/api/config/config.go` — carga de la variable de bypass, siguiendo el patrón de
  `os.Getenv` + `godotenv` ya usado. El helper existente `envBoolDefaultTrue` sirve para
  flags que faltan en `true`; este necesita el default en `false`, así que va un helper
  espejo.
- `cmd/api/config/config.go` + `cmd/api/constants/` — la zona de referencia del producto.
  Va en `constants/` según `STRUCTURE_FOLDERS.md`, que reserva esa carpeta para
  "constantes compartidas", y no en el service.

**Tests**

- `calendar_service_task3_test.go` y `calendar_service_task5_test.go` construyen `now` con
  `time.Local` y asumen semántica local. **Van a fallar** con el cambio y hay que
  actualizarlos; es esperado, no una regresión.

**Configuración / despliegue**

- `.env.example` — documentar la variable nueva.
- `render.yaml` — declarar la variable en los servicios de producción y develop, en
  `false`, con `sync: false` siguiendo el patrón de las demás.

**Fuera de este repo**

El `isCalendarDayClosed` del frontend (`utils/calendar-day-closed.js`) y el de
`utils/session-start-window.js` siguen calculando "hoy" con la zona del dispositivo. Hoy
dicen ser "réplica exacta" de esta regla y dejan de serlo en mecanismo —siguen dando la
misma respuesta porque el producto es de zona única, pero el comentario queda desactualizado
y conviene corregirlo junto con este change.
