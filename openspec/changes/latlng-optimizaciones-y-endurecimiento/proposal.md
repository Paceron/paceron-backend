# Cambio: latlng-optimizaciones-y-endurecimiento

## Por qué este cambio
El frontend reemplaza los selects anidados de país/provincia/localidad por un
`LocationPicker` (mapa + geocoder) en registro, perfil y equipos (Gap 29). Eso
exige persistir el punto exacto (`lat`/`lng`) y aceptar los nombres que devuelve
el geocoder (hoy rechazados por el charset de registro). Al mismo tiempo, las
operaciones bulk del calendario (`stamp`, `bulk`, `shift`, `bulk-clear`) y varias
lecturas priorean N+1 que se traducen en latencia visible (un stamp de 30 días
training hace ~250 round trips en una sola transacción). Se cierra el gap y se
optimizan esas rutas, más una ronda de tests de seguridad sobre las consultas y
los errores 500.

## Qué hace falta
- Gap 29 (opción (a), acordada con el frontend): el frontend sigue mandando
  códigos internos (`data/locations.js`) para `country`/`province`/`city`; el
  backend solo suma `lat`/`lng` nullable en `users` y `teams` (sin backfill,
  aceptados y devueltos igual que los campos de dirección actuales). Se relaja
  el charset de los campos de dirección del registro y del update de usuario
  (agregar `-`, `.`, `'`, `,` — nombres con guiones/puntos/apóstrofes hoy dan
  400) con mensajes de error actualizados para ser veraces. Los 4 forms del
  frontend ya mandan el código resuelto; nada más cambia.
- Rendimiento (internos, sin cambio de contrato):
  1. Escritura del calendario en batches: `stamp`/`bulk`/`shift` hacen hoy
     SELECT+INSERT/UPDATE por día (DAO `Upsert` con lógica Gap-26 adentro) y
     instancian/borran instancia por día (no deduplicado, D4). Pasa a: lectura de catálogo con `IN`,
     instanciación por lote (n+1 inserts de instancias con RETURNING, uno por
     día — D4 sin dedup), upsert multi-fila con `ON CONFLICT (group_id, date)`
     y la limpieza presencial en SQL (CASE con `EXCLUDED`), y borrado batch de
     instancias superadas (feedback check con `IN` + deletes con `IN`,
     preservando D10). `UpsertDay` individual y el path mock `db==nil` quedan
     intactos.
  2. `bulk-clear`: hoy N × `FindByGroupAndDate` + supersede per-day. Pasa a:
     1 query `date IN`, guards en memoria, 1 delete, supersede batch.
  3. `administered-calendar`: 1 query de colisiones por fecha con ≥2
     presenciales. Pasa a: 1 query para todo el rango + clasificación/dedup
     en memoria (mismas reglas cross/same del guard).
  4. `GET /invitations` (y remitentes): 3N+1 (team + invitee + inviter por
     invitación). Pasa a: 3 queries totales con los `FindByIDs` existentes.
- Seguridad (tests): pins anti-inyección sobre filtros/supuestos de texto
  (search, history, sort whitelists), asertar que los 500 genéricos no
  filtran `err.Error()` del driver (con el helper `FailingDB`), y auditar/cerrar
  endpoints con chequeo de autorización sin test 403/404.

## Constraint transversal (del usuario)
- Cero cambios de contrato para el frontend fuera de: campos `lat`/`lng` nuevos
  (aditivos) y los mensajes de charset, que se actualizan para listar los
  caracteres admitidos (antes mentían). Respuestas del calendario byte-idénticas
  (mismo wrapper/shape/errores de la PR #97/#93): "de forma que no cambie el
  consumo desde el front, solo mejoras y performance".

## Impacto
- Frontend: Gap 29 desbloquea el reemplazo de los 4 forms (pendiente su lado);
  nada más consume distinto.
- Backfill: no hay (columnas nuevas nullable, filas viejas en NULL).
- Pestañas README de docs: nota de gap 29 + deuda técnica actualizada
  (`§Resueltos` suma las rutas optimizadas de este cambio).
