# Tasks

Cada tarea es una unidad atómica, verificable por separado, y un commit lógico. No avanzar a
la siguiente hasta que la anterior está completa y verificada.

## 1. Zona horaria de referencia (constante)

- [ ] 1.1 Agregar en `cmd/api/constants/` la constante de zona horaria de referencia del
  producto: `FixedZone("ART", -3*60*60)`, con un comentario que explique por qué offset fijo
  en vez de `LoadLocation` (Argentina sin DST desde 2009; sin dependencia de tzdata del host).
- [ ] 1.2 Verificar que el paquete compila y que ningún test existente falla por la
  constante nueva: `go build ./... && go test ./cmd/api/constants/...`.

## 2. Fix de zona en `isCalendarDayClosed`

- [ ] 2.1 Reescribir el cálculo de `today` dentro de `isCalendarDayClosed`
  (`calendar_service.go:380`) para que use `now.In(refLoc)` y componentice fecha en esa
  location, en vez de `now.Location()` directo. Mantener la firma (ya recibe `now`), no
  tocar los call sites.
- [ ] 2.2 Corregir la ventana de la presencial (`~:395`): reemplazar
  `day.PresencialTimeFrom.UTC().Hour()` combinado con `now.Location()` por convertir el
  instante con `.In(refLoc)` y componer el threshold en esa misma location.
- [ ] 2.3 Actualizar el comentario de `group_calendar_day.go:19-25` que dice "hay que llamar
  `.UTC()` antes de extraer `Hour()`/`Minute()`": `.UTC()` sigue siendo lo correcto para
  formatear la hora en las responses, pero para comparar contra "ahora" hay que **proyectar**
  con `.In()`, no extraer. Esa distinción es la que hizo posible el bug.
- [ ] 2.4 Correr `go build ./...` y confirmar que los tests de calendario que fallan son
  exactamente los anticipated (task3 y task5), sin otros regresiones.

## 3. Tests de la zona horaria

- [ ] 3.1 Agregar el test del instante crítico: `2026-09-30T22:21:00-03:00` (que en UTC es
  el 1 de octubre) debe resolver "hoy" como **30 de septiembre**. Es el caso que falla en
  producción y hoy no está cubierto.
- [ ] 3.2 Agregar tests de la ventana presencial en zona de referencia: presencial que no
  arrancó → día abierto; presencial que arrancó → día cerrado.
- [ ] 3.3 Agregar el test de invariancia: el mismo instante real evaluado con el `now` en
  UTC y con el `now` en la zona de referencia produce el mismo resultado de guard.
- [ ] 3.4 Actualizar `calendar_service_task3_test.go:418-429` y
  `calendar_service_task5_test.go:317-324`, que asumen `time.Local` y `now.Location()`. El
  fallback es pasar `now` con location de referencia explícita, que además los vuelve
  deterministas en cualquier máquina. Verificar que fallan **antes** de 2.1 y pasan después.
- [ ] 3.5 `go test ./cmd/api/services/...` en verde, sin tests skips ni comentados para
  forzar el paso.

## 4. Flag de bypass en config

- [ ] 4.1 Agregar `CALENDAR_EDITION_UNRESTRICTED bool` al struct de config en
  `cmd/api/config/config.go` y un loader `loadCalendarConfig()` siguiendo el patrón de
  `loadMailerConfig`/`loadStorageConfig` (mismo `os.Getenv` + `godotenv`).
- [ ] 4.2 Agregar el helper espejo de `envBoolDefaultTrue`: ausente, vacío o no
  booleano reconocible → `false`. Especificar en el comentario por qué no se reutiliza el
  existente: aquel es fail-open y sirve para un flag de test; este es un flag que habilita
  escritura sobre datos históricos y tiene que fallar cerrado.
- [ ] 4.3 Tests del helper: ausente → `false`, `true`/`1`/`TRUE` (case-insensitive) →
  `true`, `false`/`0` → `false`, basura (`si`, `1x`, `on-off`) → `false`.
- [ ] 4.4 `go build ./... && go test ./cmd/api/config/...` en verde.

## 5. Bypass aplicado en el guard

- [ ] 5.1 Resolver el bypass **dentro de `isCalendarDayClosed`** (no en los 10 call sites):
  con el flag activo la función devuelve `false` y ninguna escritura se rechaza. Verificar
  que los callers que eligen entre dos caminos según ese bool (ej. no-tx vs tx,
  `:709`/`:736`) toman el camino esperado con el bypass activo.
- [ ] 5.2 Propagar el valor de config hasta el service por la vía de DI que ya usa
  `app.go`, sin agregar framework de inyección.
- [ ] 5.3 Test del bypass sobre día pasado con la variable activa: la escritura se acepta y
  el cambio queda persistido.
- [ ] 5.4 Test de que el bypass **no** cambia las lecturas: el calendario devuelto con el
  flag activo es idéntico al devuelto con el flag inactivo.
- [ ] 5.5 Test explícito para `BulkClear` y `Shift` con el bypass activo, para que el
  efecto sobre operaciones masivas sea intencional y no un efecto colateral (ver D4 en
  `design.md`).
- [ ] 5.6 `go test ./cmd/api/services/...` en verde.

## 6. Documentación y despliegue

- [ ] 6.1 Documentar `CALENDAR_EDITION_UNRESTRICTED` en `.env.example` con el valor `false`
  y una línea de qué hace exactamente (desactiva el bloqueo de días cerrados; **solo**
  escritura, no cambia lecturas).
- [ ] 6.2 Declarar la variable en `render.yaml` para los servicios de develop y producción,
  con `sync: false` y valor explícito `false`, siguiendo el patrón de las demás variables
  del archivo.
- [ ] 6.3 Anotar en el `.env.example` que el nombre pedido originalmente era
  `EDITION_IRRESTRICTED_DATE` y que **este es el nombre del contrato**: si algo quedó
  configurado con el anterior hay que renombrarlo.

## 7. Verificación final

- [ ] 7.1 `go build ./...`, `go vet ./...` y `go test ./...` completos en verde.
- [ ] 7.2 Recorrer a mano el escenario de aceptación de la propuesta: servidor con `TZ=UTC`,
  reloj en las 22:21 ART del 30/09, `PUT` del día de hoy y del día de mañana del mismo
  grupo → `200` en ambos (antes `422`).
- [ ] 7.3 Con la variable en `false`, confirmar que una fecha pasada sigue devolviendo 422
  con el mismo cuerpo de error que antes.
- [ ] 7.4 Registrar en el PR la consecuencia conocida: el bypass destraba la API pero el
  frontend sigue ocultando el botón de guardar, así que usarlo desde la UI requiere un
  change posterior en `utils/calendar-day-closed.js`.
