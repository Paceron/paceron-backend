# Design

## Context

Ver `proposal.md` para la motivación. Lo que condiciona el approach:

**El guard tiene 10 call sites de escritura, todos en un archivo.**
`isCalendarDayClosed` se llama desde `closedDayForRequest` (helper) y directo. Repartidos:
`UpsertDay` (4 caminos: no-tx y tx, cada uno con su rama), `DeleteDay` (2), `Stamp` (1),
`Bulk` (2), `BulkClear` (2 loops), `Shift` (2 loops). Todos terminan en
`ErrCalendarDayClosed` (`calendar_service.go:33`), que `mapCalendarError`
(`calendar_controller.go:67-68`) mapea a **422** con el mensaje; la forma envuelta
(`calendarClosedDaysError`, `:62-77`) agrega las fechas cuando la operación las abarca.

**La corrección tiene que entrar por un solo lugar.** Los 10 call sites pasan `time.Now()`
como argumento a una función que ya recibe `now` — el design ya estaba preparado para
inyectar el reloj, solo que en producción siempre se inyecta el del proceso. Eso significa
que el fix de zona no tiene que tocar los call sites: se toca la función y el `now` que le
llega.

**`PresencialTimeFrom` es un `timestamptz` con convención UTC, no una hora de reloj.**
El modelo (`cmd/api/domains/dbs/group_calendar_day.go:26`) lo documenta: se guarda con
fecha `0000-01-01` y **hay que llamar `.UTC()` antes de extraer `Hour()`/`Minute()`**, que
es exactamente lo que hace la línea 395. El DSN fija la zona de sesión en UTC
(`infrastructure/postgresdb/postgres.go:26`), así que el driver devuelve los valores
normalizados. `day.Date` es un `date` (sin componente horario) que pgtype decodifica a
medianoche en UTC.

**El bug de la línea 395 es una mezcla de zonas, no un error de conversión.** Se toman las
horas **UTC** del horario planificado y se las interpolan dentro de una `time.Date` construida
con `now.Location()`. Hoy eso da la respuesta correcta **por coincidencia**: la location del
proceso es UTC, así que las dos mitades coinciden. Es una bomba de tiempo, no un descuido — el
comentario del modelo advierte explícitamente del `.UTC()` y la función lo cumple a medias.

**Config: `os.Getenv` + `godotenv`, sin viper ni envconfig** (`cmd/api/config/config.go:142`).
Los loaders son `loadDBConfig`, `loadMailerConfig`, `loadMercadoPagoConfig`,
`loadStorageConfig`, y usan helpers chicos: `getEnvOrDefault`, `getDurationOrDefault`,
`envBoolDefaultTrue` (`:224-234`). Este último **trata ausente e inválido como `true`**,
que es lo contrario de lo que necesita un flag de seguridad de escritura.

**Tests que asumen semántica local y van a fallar.** `calendar_service_task3_test.go:418-429`
arma `now := time.Date(2026,3,15,12,0,0,0,time.Local)` y `today` en la misma location;
`calendar_service_task5_test.go:317-324` usa `time.Now()` con `now.Location()`. Los dos
verifican el comportamiento correcto del guard, pero anclado a la zona del proceso.

## Goals / Non-Goals

**Goals:**

- Que el resultado del guard no dependa de la zona del host.
- Que la ventana de la presencial deje de mezclar zonas.
- Un flag de bypass que degrade a "cerrado" ante cualquier duda.
- Que el flag se pueda documentar y desplegar siguiendo el patrón que ya existe.

**Non-Goals:**

- No se cambia el criterio de día cerrado (ver `proposal.md`).
- No se cambian endpoints, formas de request/response ni el mapeo a 422.
- No se tocan los filtros de lectura de los DAO, que ya normalizan a UTC con
  `AT TIME ZONE 'UTC'` en SQL y no pasan por esta función.
- No se cambia la zona de sesión de la base de datos.
- No se agrega un clock inyectable como dependencia en el service. La firma ya acepta `now`;
  alcanza con discipline en el call site.

## Decisions

### D1 — La zona de referencia es un offset fijo, no `LoadLocation`

`time.FixedZone("ART", -3*60*60)` en `cmd/api/constants/`, en vez de
`time.LoadLocation("America/Argentina/Buenos_Aires")`.

La razón es que **Argentina no tiene horario de verano desde 2009** — está en UTC-3 de forma
continua. Un offset fijo es exactamente correcto para toda la historia relevante del
producto, y elimina una clase entera de fallas: `time.LoadLocation` depende de la base de
datos de zonas horarias del sistema, que no está garantizada en una imagen Docker mínima, y
si falla hay que decidir qué hacer en runtime (panic? fallback silencioso? error?).

**Alternativas descartadas:**

- *`LoadLocation` con fallback.* Agrega una rama de error y una dependencia de tzdata por un
  beneficio —manejar DST— que el país no tiene.
- *Hacer la zona configurable por env.* Rechazada: es una decisión de negocio, no una
  propiedad del ambiente. Si algún día el producto opera en otra zona, se cambia la
  constante y se bumpea la versión.
- *Configurar `TZ=America/Argentina/Buenos_Aires` en el host.* parece la solución de una
  línea, y no sirve: implícitamente sigue al install del host, depende de que el host de Render y cada
  entorno local estén bien seteados, no queda registrada en el código, y no protege contra
  un dev que corra el backend con otra TZ sin darse cuenta. La constante en el código hace
  que el comportamiento sea el mismo en todas partes por construcción.

### D2 — El fix de zona entra por la función, no por los 10 call sites

`isCalendarDayClosed` ya recibe `now time.Time`. El cambio es hacer que dentro de la función
todo se evalúe en la zona de referencia:

```go
refNow := now.In(calendarReferenceLocation())
today := time.Date(refNow.Year(), refNow.Month(), refNow.Day(), 0, 0, 0, 0, calendarReferenceLocation())
```

`dayDate` sigue construyéndose con los componentes Y/M/D de `day.Date` en la misma location
—solo importan los componentes, así que la location de origen es irrelevante— y eso
**deja el problema del driver resuelto por construcción**: no hay que asumir nada de cómo
pgtype decodifica un `date`.

Los 10 call sites siguen pasando `time.Now()`. No se tocan. El `now` inyectado ya existía
para los tests; en producción era siempre el del proceso, y ahora la función lo corrige.

**Por qué no el `now` en el call site** (`time.Now().In(refLoc)` en los 10): funciona igual,
pero reparte la decisión en 10 lugares y cualquiera que agregue un call site nuevo la
reintroduce mal. Concentrarla en la función hace que el comportamiento correcto sea el
default y que el error sea visible en un solo punto.

### D3 — La ventana de la presencial se convierte, no se reensambla

```go
planned := day.PresencialTimeFrom.In(calendarReferenceLocation())
threshold := time.Date(refNow.Year(), refNow.Month(), refNow.Day(),
    planned.Hour(), planned.Minute(), 0, 0, calendarReferenceLocation())
```

El horario planificado **se convierte** a la zona de referencia y de ahí se toman las
componentes. Reemplaza al `day.PresencialTimeFrom.UTC().Hour()` + `now.Location()`, que
mezcla dos zonas.

El `.In()` es correcto porque el valor underlying es un instante UTC (así se guardó y así
llega, con el DSN en UTC) —`In` no reinterpreta nada, cambia la proyección. Y como el
horario planificado es una hora **de reloj local del producto** (14:00 significa las 14:00
del usuario), al proyectarlo a la zona de referencia da 14:00, que es lo que la función
pretendía calcular y lo que el bug rompía en cuanto el host dejara de estar en UTC.

**Consecuencia que hay que dejar escrita:** el comentario del modelo
(`group_calendar_day.go:19-25`) que dice "hay que llamar `.UTC()` antes de extraer
`Hour()`/`Minute()`" pasa a estar desactualizado para esta función. El `.UTC()` sigue siendo
correcto para formatear la hora en las responses (`:872`, `:876`), que viajan al cliente
como `15:04`; lo que cambia es que **para comparar contra "ahora" hay que proyectar, no
extraer**. Es la distinción que hizo el bug posible y hay que documentarla donde alguien la
va a volver a pisar.

### D4 — El flag se resuelve en `config`, se lee en el service, y degrada a cerrado

`cmd/api/config/config.go` gana un `CALENDAR_EDITION_UNRESTRICTED bool` cargado en un
`loadCalendarConfig()` siguiendo el patrón de los otros loaders, con un helper **espejo** de
`envBoolDefaultTrue`: ausente, vacío o no booleano reconocible → `false`.

El espejo es necesario y no es un detalle de estilo: `envBoolDefaultTrue` existe para
`MP_OAUTH_TEST_TOKEN`, donde faltar la variable debe significar "dejame pasar". Acá lo
opuesto — si el deploy se rompe y la variable no se lee, el sistema tiene que **cerrar**,
no abrir. Un helper que devuelve `true` ante un `os.Getenv` vacío sería un fail-open en un
flag que habilita escritura sobre datos históricos.

La lectura ocurre en el service, en el punto donde hoy se llama a `isCalendarDayClosed`. La
opción de cortocircuitar es en `closedDayForRequest` más un chequeo temprano en cada
operación: **en vez de tocar los 10 call sites, el bypass se resuelve DENTRO de
`isCalendarDayClosed`**, que ya es el punto único por el que pasan todos. Con el flag
activo la función devuelve `false` y ninguna escritura se rechaza.

Eso tiene un efecto secundario que hay que decidir explícitamente: `closedDayForRequest`
devuelve `bool` y sus callers la usan para **elegir entre dos caminos** (ej. no-tx vs tx,
`:709`/`:736`), no solo para rechazar. Si la función devuelve `false` siempre, esos callers
toman el camino alternativo — que es precisamente lo que queremos cuando el bypass está
activo. El bypass no necesita ningún branch extra: **reutiliza el return de la función**.

**Riesgo a mirar:** `BulkClear` y `Shift` recorren listas y con el bypass activo se
comportarían como si ningún día estuviera cerrado. Es lo pedido, pero conviene que el test lo
cubra explícitamente para que no sea un efecto colateral nadie revisó.

### D5 — El nombre de la variable

`CALENDAR_EDITION_UNRESTRICTED`. Venía pedido como `EDITION_IRRESTRICTED_DATE`, que tiene
dos problemas: el prefijo `IRRESTRICTED` no es una palabra (es `UNRESTRICTED`), y el sufijo
`_DATE` no corresponde a un flag booleano. El nombre nuevo sigue la convención del repo
(variables de dominio con prefijo, en SCREAMING_SNAKE: `ATTENDANCE_BASE_URL`, `MP_OAUTH_*`) y
deja el alcance explícito. **Es un contrato de despliegue: si el equipo ya configuró algo con
el nombre anterior, hay que renombrarlo acá.** Se documenta en `.env.example` y `render.yaml`
con el valor `false` explícito en ambos servicios, para que nadie dependa del default.

## Risks / Trade-offs

**[Los tests existentes fallan al cambiar la semántica de `now`]** → Es esperado y
correcto: están anclados a `time.Local` por una razón que este change invalida. Se actualizan
para pasar un `now` con location de referencia explícita, que además los hace deterministas
en cualquier máquina. Se agrega el test del instante crítico (22:21 ART del 30/09) que hoy
no existe y que es el que falla en producción.

**[Cambiar el comportamiento observable del guard puede afectar a un cliente que ya dependa
del 422 indebido]** → El único cliente es el frontend, que ya considera cerrada esa fecha.
Corregirlo es el objetivo. No hay consumidor de terceros.

**[El bypass activo abre escritura sobre datos históricos sin que nada lo registre]** → El
flag es deliberadamente simple y sin auditoría propia: es una herramienta operativa. Lo que
sí tiene es el log estructurado que ya emite el service, así que un stamp masivo con el flag
activo queda registrado como las demás operaciones. Si más adelante hace falta saber *quién*
usó el bypass, eso es un requerimiento nuevo (actor + log dedicado), no una extensión de
este.

**[Con el bypass activo, las respuestas de error de día cerrado desaparecen y el frontend
sigue ocultando el botón]** → Ver "Consecuencia conocida" en `proposal.md`. El bypass
destraba la API, no la UI. Anotado para que no se interprete como fix completo de
edición.

**[`calendar_service_test.go` valida que `today` se construya con `now.Location()` en el
camino de `NextSession`]** → Ese path (`NextSession` / `NextPresencialSession`) **no pasa por
`isCalendarDayClosed`**: arma sus propios `today` y `nowHHMM` y los pasa a los DAO
(`:1496-1541`), que ya normalizan a UTC en SQL. Queda fuera de este change por decisión
(propuesta, "No alcance"). Pero significa que **el mismo concepto de "hoy" se calcula en dos
lugares con dos criterios distintos**, y que arreglar uno no arregla el otro. Es deuda que
esta change deja explícita en vez de heredada; conviene una change posterior que unifique
el cálculo de "hoy" del calendario en un helper único.

## Migration Plan

Sin migración de datos ni de schema. Desplegar = reiniciar el proceso con la variable en
`false` explícita.

Rollback: revertir el commit y reiniciar. El único cambio de comportamiento observable entre
ambas versiones es el del cálculo de "hoy", y volver atrás devuelve exactamente el
comportamiento actual — que es el bug, pero no rompe nada que hoy funcione.

Orden de despliegue sugerido: primero el fix de zona (que es una corrección pura), esperar a
que el equipo confirme que las sesiones de hoy y mañana ya se editan, y recién después
agregar el flag. Deployarlos juntos vuelve imposible distinguir cuál de los dos cambió el
comportamiento si algo falla.

## Open Questions

Ninguna que bloquee la implementación.
