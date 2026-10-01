# Design — Gestión de asistencia desde el panel del entrenador

## Context

Estado actual relevante (el *why* está en `proposal.md`):

- `attendances(id, team_id, training_session_id, user_id, created_at, updated_at)`
  con `UNIQUE(team_id, training_session_id, user_id)`. `training_session_id` está
  documentada como **FK opaca** porque `training_sessions` no existe
  (`domains/dbs/attendance.go:6-8`).
- El identificador real de una **sesión realizada** es `session_instances(id)`, que
  se crea **una por día asignado** (`asignacion-por-instanciacion`, design D1) y
  por lo tanto es único por ocurrencia. `SessionInstance` es una copia congelada
  del catálogo: tiene `id, name, description, source_session_id, created_at` — **no**
  tiene `group_id`, `date` ni `is_presencial`.
- Eso último vive en `group_calendar_days`, que enlaza con
  `session_instance_id` y tiene `UNIQUE(group_id, date)`
  (`domains/dbs/group_calendar_day.go:10-11`). Un día es `kind` ∈
  `training|rest|cancelled|other`, con `is_presencial`, `presencial_time_from/to`
  y `presencial_location`.
- No existe ningún endpoint que liste sesiones presenciales pasadas de un grupo.
  `GET /groups/{id}/calendar` es range-scoped (`from`/`to`) y el front lo usa con
  granularidad mensual (`utils/calendar-month-range.js`).
- El roster de grupo (`group_users`) tiene `date_start`, `date_end` y `deleted_at`.
  El criterio de "activo" que usa hoy el sistema es **`deleted_at IS NULL`**
  (`group_user_dao.go:48-56` — `FindByGroupID` ni siquiera mira las fechas;
  `FindByUserID` sí mira `date_end` pero solo para los banners del home).
- Ya existe `attendance_dao.TeamExists` / `IsTeamOwner` / `GetTeamUserRole`
  delegando en `TeamMembershipDAO`, y `attendance_service.Search` ya resuelve la
  regla "owner **o** `team_users.role_in_team = 'entrenador'`". Esa lógica se
  **reutiliza**, no se reimplementa.
- Tests DAO/service de este módulo corren contra **Postgres real**
  (`docker start paceron-test-db`, :5433, env `TEST_DB_*`), con gate de coverage
  80.

## Goals / Non-Goals

**Goals:**

- Un solo JOIN (o una sola query con `LEFT JOIN`) para la grilla, para que el
  frontend no haga N+1 al pintar la tabla.
- Que todo endpoint que resuelve una sesión (los 4 nuevos más `GET /qr`) comparta
  **una** función de resolución, de modo que las reglas de "presencial / no cancelada / del equipo
  indicado" no puedan divergir entre endpoints.
- Reutilizar la matriz de autorización existente en lugar de escribir una nueva.
- Que la carga masiva sea una sola sentencia SQL, no un loop de inserts.

**Non-Goals (límites de diseño, no de producto):**

- No se modela un estado de asistencia (`present/absent/excused`). La presencia
  es binaria y se persiste como fila.
- No se agrega `deleted_at` a `attendances`: el borrado es físico (ver D4).
- No se cambia el contrato de `GET /attendance/search`; solo se le reutiliza la
  regla de rol.

## Decisions

### D1 — La clave de la asistencia es `session_instance_id`, y la validación se hace por JOIN

Se adopta literalmente lo pedido: `attendances.training_session_id` =
`session_instances.id`, y se agrega la FK real que faltaba
(`→ session_instances(id)`), que hoy es posible justamente porque
`session_instances` **sí** existe.

El problema que esto no resuelve solo: `SessionInstance` no sabe a qué grupo, ni
a qué fecha, ni si fue presencial. Por eso se agrega un resolver compartido
(`attendanceSessionContext`) que hace
`session_instances ⋈ group_calendar_days ON group_calendar_days.session_instance_id = session_instances.id`
y devuelve `(group_id, date, kind, is_presencial, team_id, ...)`. **Ese es el
único punto del código donde se consulta esa información**: los 3 endpoints
nuevos y el QR la usan a través del resolver, y ninguno vuelve a armar el JOIN
por su cuenta.

**Alternativa descartada — apuntar a `group_calendar_day_id`.** Era la opción más
limpia: la fila de calendario ya tiene `group_id`, `date` e `is_presencial`, y
`UNIQUE(group_id, date)` la hace direccionable sin índice nuevo. Se descartó
porque (a) la clave acordada con el equipo de producto es el
`session_instance_id`, y (b) `session_instances` es 1:1 con la ocurrencia (se crea
una instancia por día), así que no hay duplicidad de riesgo. Queda anotado como
candidata natural para el día que se quiera el histórico por fecha, junto con el
mismo cableado diferido que ya arrastra `workout_feedback.assigned_session_id`
(`asignacion-por-instanciacion` lo dejó explícitamente como non-goal).

### D2 — La lista de "sesiones presenciales ocurridas" es un endpoint propio, no un rango de calendario

`GET /groups/{id}/attendance-sessions?team_id=` con el filtro
`kind='training' AND is_presencial=true AND date <= CURRENT_DATE AND session_instance_id IS NOT NULL`,
`ORDER BY date DESC`.

Se evaluó reutilizar `GET /groups/{id}/calendar` con `from` muy temprano y
`to=hoy`. Descartado por dos razones: (a) el calendario devuelve **todas** las
filas del rango (descartables y `rest` incluidos), y el filtro `is_presencial`
tendría que hacerlo el cliente sobre un payload que incluye la locación y el
`session_instance` completo anidado; (b) no hay un rango "todo el pasado"
natural — `from` sería un hack y el payload crece con la historia del grupo.

**`CURRENT_DATE` del servidor**, no el del cliente: la regla "no se ven sesiones
futuras" es de negocio y no debe depender del reloj del dispositivo.

### D3 — Presencia binaria, sin columna de estado

Fila presente = asistió. Sin fila = no confirmed. No hay `status`.

El requerimiento "cargar la asistencia y confirmarla" se interpreta como *dar de
alta las filas y guardar*, no como un flujo de dos estados: el producto pidió además
poder **borrar** una asistencia cargada, y un modelo de dos estados con borrado
sería redundante. `not_confirmed` se **deriva** en la respuesta, nunca se
persiste.

### D4 — Borrado físico, y por qué la UNIQUE actual no estorba

`DELETE` físico, sin `deleted_at`. La preocupación de que
`UNIQUE(team_id, training_session_id, user_id)` bloquearía volver a marcar
después de borrar **no aplica**: un hard delete libera la fila del índice, así
que el re-marcado posterior funciona (hay scenario explícito en el spec). Lo que
no se puede es conservar el historial de "estuvo y ya no está" — ver R3.

Por qué físico y no soft-delete: una asistencia borrada es un **error del
entrenador que se está corrigiendo**, no un evento de negocio. Un soft-delete
obligaría a que toda lectura (incluida la grilla) filtrara `deleted_at IS NULL`,
a que la UNIQUE siguiera bloqueando el re-marcado (haría falta un índice único
parcial), y a que el "borrado" fuera indistinguible de "asistió pero fue
anulado" — distinción que hoy nadie necesita.

### D5 — La carga masiva es un `INSERT ... ON CONFLICT DO UPDATE`, no un loop

`POST /attendance/bulk` se resuelve con **una** sentencia:

```sql
INSERT INTO attendances (team_id, training_session_id, user_id, source, registered_by_user_id, created_at, updated_at)
SELECT $1, $2, unnest.user_id, 'manual', $3, NOW(), NOW()
FROM unnest($4::bigint[]) AS unnest(user_id)
ON CONFLICT (team_id, training_session_id, user_id) DO UPDATE
  SET updated_at = NOW(), source = 'manual', registered_by_user_id = $3
RETURNING (xmax = 0) AS inserted, user_id;
```

`xmax = 0` distingue insert de update en una sola pasada y da los contadores
`created`/`updated` que pide el spec **sin** un `SELECT` previo (mismo criterio
anti-race que ya usa `attendance_dao.Create` para el alta del corredor). El
`unnest` evita ir y volver por fila.

**Todo-o-nada**: la validación de que todos los `user_id` sean miembros activos
del grupo se hace **antes** de abrir la transacción, con un `WHERE user_id = ANY($n)
AND group_id = $g AND deleted_at IS NULL` que devuelve el subconjunto faltante.
Si falta alguno → `422` con la lista, sin escribir nada.

### D6 — La grilla: roster y asistencia en una query con `LEFT JOIN`

```
FROM group_users gu
JOIN groups g ON g.id = gu.group_id
LEFT JOIN attendances a
       ON a.training_session_id = $session_instance_id
      AND a.team_id = $team_id
      AND a.user_id = gu.user_id
WHERE gu.group_id = $group_id
  AND gu.deleted_at IS NULL
  AND gu.date_start <= $session_date
  AND (gu.date_end IS NULL OR gu.date_end >= $session_date)
```

`$session_date` es la `date` de la fila de `group_calendar_days` que resolvió
`resolveAttendanceSession` — **no** `CURRENT_DATE`. Es el criterio de D7.

Con `a.*` proyectado, `status` se deriva en el service de
`a.id IS NOT NULL`. Los nombres salen del `BatchLookup` de usuarios ya existente
(`GET /users?ids=`, usado por el front) — **una** llamada extra al
`UserBatchLookup` para toda la grilla, no una por fila.

**El `name` de la fila se compone en el backend con la misma regla que el
roster que la app ya muestra** (`hooks/use-team-roster.js:60`), porque `users`
separa el nombre en dos columnas:

```go
name := strings.TrimSpace(user.Name + " " + user.Surname)
if name == "" {
    name = user.Email
}
```

Sin ese `TrimSpace` + fallback, la grilla mostraría `"Ana "` con espacio
sobrante y ordenaría distinto que el roster.

El **orden alfabético del spec se resuelve en Go, después del batch lookup**, no
en SQL: `group_users` no tiene columna de nombre, así que un `ORDER BY` por
nombre en la query de la grilla no es posible sin un `JOIN` extra a `users` que
duplicaría datos que el batch lookup ya trae. El service arma el map de nombres
y ordena el slice con `sort.Slice` por `strings.ToLower(name)`, con desempate
estable por `user_id` para que el orden no dependa del plan de la query.

`summary` se calcula **en Go**, aritmética sobre los conteos ya disponibles en la
fila (`COUNT(*)`, `COUNT(a.id)`). No hace falta una query de agregados aparte: la
grilla ya son todas las filas. `attendance_rate_pct` se redondea a 1 decimal;
cuando `roster_size == 0` se fuerza `0` para no dividir por cero.

### D7 — Miembro activo = ventana de membresía evaluada **en la fecha de la sesión**

Una membresía en `group_users` tiene tres estados distinguibles, y esta decisión
define cuáles entran a la grilla:

| Estado | `deleted_at` | `date_end` |
| --- | --- | --- |
| Activo | `NULL` | `NULL` o futuro |
| Dejó el grupo | `NULL` | pasado |
| Borrado | pasado | — |

**El criterio es la ventana de membresía contra la fecha de la sesión**, no
contra la fecha de hoy:

```
deleted_at IS NULL
AND date_start <= session_date
AND (date_end IS NULL OR date_end >= session_date)
```

Se descarta deliberadamente la alternativa de `date_end > NOW()`. La sesión que
se está reviendo **ya ocurrió**, así que la pregunta correcta no es "quién está
hoy en el grupo" sino "quién era miembro cuando la sesión pasó". Con
`> NOW()`, un corredor que dejó el grupo la semana pasada desaparecería de la
grilla de una sesión de hace tres semanas, aunque era miembro activo el día que
se recibió — y el entrenador no podría registrar esa asistencia, que es
precisamente el dato que se está intentando recuperar.

**Consecuencia asumida, y es deliberada:** la grilla puede mostrar menos gente
que la pestaña de roster del mismo grupo, porque el roster de la pantalla de
equipo usa `FindByGroupID` (`group_user_dao.go:54`), que solo mira
`deleted_at IS NULL` e **ignora `date_end`**. No se unifica con ese criterio:
el roster de la pantalla de equipo responde "¿quién integra este grupo hoy?",
mientras que la grilla responde "¿quién podía asistir a esta sesión?".
Unificarlos obligaría a elegir entre marcar a gente que ya no está (roster) o
borrar el registro histórico de una asistencia real (grilla).

El repo ya es inconsistente en esto y esta decisión no lo empeora: el calendario
sí excluye membresías vencidas (`FindByUserID`, `group_user_dao.go:66`, con
tests en `calendar_service_test.go:501,855`). La grilla se alinea con el
calendario, no con el roster.

Un beneficio del criterio por fecha de sesión: `roster_size` de una sesión dada
**queda congelado en el tiempo**. Si se usara `NOW()`, el mismo endpoint
devolvería denominadores distintos conforme vencen membresías, y las
asistencias ya registradas empezarían a sumar más de 100 %.

Este mismo criterio es el que usa `MissingGroupMembers` para validar el lote de
`POST /attendance/bulk` (tasks 3.2): un `user_id` que no era miembro del grupo
**en la fecha de esa sesión** invalida todo el lote con `422`.

### D8 — "Ser entrenador" reutiliza la lógica existente, sin endpoint nuevo para el rol

No se agrega un endpoint tipo "soy entrenador de este equipo". El service llama
a los mismos tres helpers que ya usa `Search` (`TeamExists` → `IsTeamOwner` →
`GetTeamUserRole`), extraídos a un método privado
`resolveTrainerRole(ctx, teamID, userID) (bool, error)` para que los 6 handlers
compartan exactamente el mismo criterio. Es el criterio que el usuario pidió
(owner del equipo) **más** el rol `entrenador` del roster que `Search` ya
aceptaba — no se restringe respecto de hoy, se unifica.

### D9 — Dos brechas de autorización se cierran en este change

`GenerateQR` y `Register` verifican **solo** que haya un usuario autenticado.
Consecuencias reales, hoy explotables: cualquiera autenticado puede pedir el QR
de cualquier equipo (y el QR es la URL que registra asistencia), y cualquiera
autenticado puede insertar asistencias en nombre de cualquier `user_id` para
cualquier equipo y sesión.

Se cierran acá y no en un change aparte porque son la mitad de la superficie del
módulo y el frontend es el primer consumidor real de esos endpoints: publicar la
pantalla de entrenador sin cerrarlos dejaría el agujero abierto y con tráfico.
`Register` pasa a exigir **membro activo del grupo** (no "entrenador"): es el
corredor el que se registra, y exigirle ser entrenador lo dejaría sin poder
usar su propio flujo.

### D10 — Provenance: dos columnas, no una tabla de auditoría

`source` (`'qr' | 'manual'`) y `registered_by_user_id` (nullable). Se descartaron
las alternativas más ricas: una tabla de auditoría de cambios daría historial de
"quién cambió qué y cuándo", que no se pidió y agrega un modelo de concurrencia
para un recurso que el entrenador edita en una sentada; y `status` +
`deleted_at` se descartan por D3/D4.

`registered_by_user_id` queda **nullable** y sin FK: para filas por QR el valor
natural es el propio corredor (y se guarda, es dato útil), pero se deja nullable
para el backfill de filas preexistentes, donde no hay dato de origen. La
integridad referencial no se agrega porque el actor de un registro histórico
puede haber sido dado de baja y borrado de la tabla de usuarios; una FK con
`ON DELETE` restrictivo rompería borrados de usuario que hoy funcionan.

### D11 — La fecha es una condición del filtro, no un dato que se compute

`date <= CURRENT_DATE` va en el WHERE (SARGable, usa el índice) y **no** se
convierte a `time.Time` en Go para comparar. La validación de sesión no presencial
/ cancelada, en cambio, sí se hace en el service sobre la fila ya resuelta,
porque no es filtrado sino rechazo: una sesión no presencial debe dar `422` con un
mensaje que lo explique, no desaparecer del resultado.

## Risks / Trade-offs

- **[R1] La UNIQUE y el `ON CONFLICT` dependen de que el índice exista con esos
  tres columnas en ese orden y NO sea parcial.** El plan asumía que el índice ya
  estaba (`uq_att_team_session_user`, declarado en el modelo GORM), pero **eso no
  está verificado**: un tag `uniqueIndex` del struct describe lo que GORM crearía
  con `AutoMigrate` sobre una base nueva, y no dice nada sobre la base real. La
  inferencia "el índice existe porque la idempotencia de `Register` funciona" tiene
  dos agujeros: (a) solo vale si alguien alguna vez escaneó dos veces la misma
  sesión, y (b) **un índice PARCIAL también dispararía el `23505` que `Create`
  atrapa**, así que `Register` funcionaría igual mientras el `ON CONFLICT` del
  camino nuevo fallaría — el caso donde el código viejo está bien y el nuevo se
  rompe. → Mitigación en tres capas: la migración verifica y crea el índice si
  falta (paso 5), `TestAttendanceDao_UniqueIndexExists` falla ruidosamente si no
  matchea, y el DAO envuelve el error de Postgres con el SQL a correr.

- **[R2] `CURRENT_DATE` depende del timezone del servidor Postgres**, no del de
  la app. Si la DB corre en UTC y el usuario espera su hora local, la sesión de
  "hoy" puede aparecer o no según la hora. → Se documenta; el servicio no lo
  compensa. Es el mismo criterio que ya usa el resto del módulo de calendario
  (`CalendarSummary`, `NextSession`), así que no introduce una inconsistencia
  nueva.

- **[R3] El borrado físico pierde el historial.** Si más adelante se quiere
  auditar que "este corredor justifica que fue marcado por error y se corrigió",
  esa información ya no existe. → Aceptado explícitamente (D4). Si el requirement
  aparece, la
  migración es agregar `deleted_at` + índice único parcial, y el `ON CONFLICT`
  pasa a `WHERE deleted_at IS NULL`.

- **[R4] El roster no es date-aware (D7).** Un corredor que se fue del grupo hace
  6 meses sigue apareciendo en las sesiones viejas, y cuenta para
  `roster_size` (y baja el porcentaje). → Aceptado por consistencia con el roster
  existente. Se deja anotado como follow-up natural, que además necesitaría
  definir el mismo criterio en las dos pantallas a la vez para no reintroducir la
  inconsistencia que D7 evita.

- **[R5] `registered_by_user_id` sin FK** (D10): nada impide que apunte a un
  `user_id` inexistente. → Aceptado a propósito; el actor puede haber sido
  eliminado y no se quiere bloquear ese borrado.

- **[R6] La limpieza previa de datos de prueba es manual.** Las filas de
  `attendances` cargadas a mano en dev apuntan a `training_session_id` que puede
  no existir en `session_instances`; la FK nueva las rechazaría. → El plan de
  migración incluye un `SELECT` de preview + `DELETE` acotado, y el `DELETE` está
  respaldado por confirmación explícita del equipo (datos de prueba, se pueden
  perder).

- **[R7] Endurecer `GET /attendance/qr` rompe cualquier consumidor actual.** Hoy
  no hay ninguno (el frontend no tiene código de asistencia), pero si alguien lo
  implementó por fuera del repo, pasa a recibir `403`. → Verificado contra el
  código del front: cero referencias. Se deja en el changelog del `README.md`.

## Migration Plan

1. **Preview** (sin escritura):
   ```sql
   SELECT a.id, a.team_id, a.training_session_id
   FROM attendances a
   LEFT JOIN session_instances si ON si.id = a.training_session_id
   WHERE si.id IS NULL;
   ```
   Esperado: solo filas del set de pruebas manuales. Si apareciera una fila de
   un `session_instance` que sí existe pero de un equipo distinto, **parar** y
   revisar con el equipo antes de seguir.
2. **Limpieza** — `DELETE FROM attendances WHERE training_session_id NOT IN (SELECT id FROM session_instances);`
   (el equipo ya confirmó que son datos de prueba descartables).
3. **Columnas nuevas** — `ADD COLUMN source text NULL`, `ADD COLUMN
   registered_by_user_id bigint NULL` (sin FK, ver D10).
4. **Backfill** — `UPDATE attendances SET source = 'qr' WHERE source IS NULL;`
   + `ALTER COLUMN source SET NOT NULL`. Esto asegura que `source` nunca devuelva
   un valor fuera del dominio.

   El valor del backfill es **`'qr'`, no `'manual'`**: el único escritor de
   `attendances` hoy es `attendance_service.go:82`, el registro del corredor al
   escanear el QR. No existe alta manual todavía, así que **toda** fila
   preexistente proviene necesariamente de un QR. Backfillear a `'manual'`
   etiquetaría como "cargada por el entrenador" la asistencia histórica
   entera — y `source` es exactamente el campo que la grilla muestra como
   procedencia, así que el error sería visible para el usuario, no invisible.

5. **Chequeo de índice** — confirmar que hay un índice UNIQUE sobre
   `(team_id, training_session_id, user_id)`. La verificación:

   ```sql
   SELECT indexname, indexdef FROM pg_indexes
   WHERE tablename = 'attendances' AND indexdef ILIKE '%UNIQUE%';
   ```

   Si no hay ninguno sobre esas tres columnas **en ese orden**, crearlo. La guarda
   se hace **por columnas y no por nombre** a propósito: si ya existe con otro
   nombre, `CREATE INDEX IF NOT EXISTS` crearía un índice redundante con el mismo
   contenido (doble costo de escritura) en lugar de notar que el que importa ya
   estaba:

   ```sql
   DO $$
   BEGIN
     IF NOT EXISTS (
       SELECT 1 FROM pg_indexes
       WHERE tablename = 'attendances'
         AND indexdef ILIKE '%UNIQUE%'
         AND indexdef ~ 'team_id.*training_session_id.*user_id'
         AND indexdef NOT LIKE '%WHERE%'
     ) THEN
       CREATE UNIQUE INDEX uq_att_team_session_user
         ON attendances (team_id, training_session_id, user_id);
     END IF;
   END $$;
   ```

   Las dos cosas que hacen fallar el `ON CONFLICT` y que el `IF NOT EXISTS` por
   nombre NO detectan: **orden distinto** de las columnas, y **índice parcial** (con
   `WHERE`). Por eso la guarda mira `indexdef` y no el catálogo de constraints.
### Migración manual, no AutoMigrate (decisión, 2026-09-28)

El SQL de esta migración quedó versionado en **`scripts/migrate_attendance_source_provenance.sql`**, y es **manual y obligatorio en cada base nueva**.

Motivo: los modelos GORM de `cmd/api/domains/dbs/` no declaran asociaciones `constraint:` —`grep "constraint:" cmd/api/domains/dbs/*.go` no devuelve nada—, así que `AutoMigrate` crea tablas, columnas e índices pero **ninguna foreign key**. La FK del paso 6 existe en la base de desarrollo solo porque el SQL se corrió a mano.

Consecuencias, aceptadas a propósito:

- CI, `make test-db-up` y la máquina de un compañero levantan bases **sin** la FK. Por eso la tarea 0.8 no puede ser un test de comportamiento: correría contra una base sin la restricción y fallaría siempre. Lo que se automatiza es que el script no se pierda ni se corrompa (`cmd/api/daos/attendance_migration_test.go`).
- `docs/TESTING.md` decía que el `AutoMigrate` de test nunca diverge del de producción. Era falso para FKs; corregido.
- La verificación real de la FK es manual, contra una base migrada: es el paso 5.7 del change.

La alternativa descartada era declarar la asociación GORM con `constraint:OnDelete:RESTRICT` en el modelo, que haría la FK reproducible por código y dejaría de hacer falta el script. Se descarta para no ser el primer modelo del repo con una decisión de schema que los demás no siguen; si algún día se toma, `TestAttendanceMigration_ElModeloNoDeclaraLaForeignKey` falla a propósito para avisar que el script quedó obsoleto.

6. **FK** — `ALTER TABLE attendances ADD CONSTRAINT fk_attendances_session_instance
   FOREIGN KEY (training_session_id) REFERENCES session_instances(id);`

   Sin cláusula `ON DELETE`, o sea **`RESTRICT`**. Es deliberado y **no** es
   `CASCADE`: si algún día se expone un borrado de sesión, `CASCADE` borraría
   en silencio el registro histórico de las asistencias, que es justo lo que
   este change existe para preservar.

   `RESTRICT` parece que va a interrumper el borrado de sesiones que ya tiene el
   calendario, pero **no lo hace**: no hay ningún camino alcanzable
   que borre una `session_instance` con asistencias debajo. La cadena de guardas
   que lo garantiza, verificada contra el código:

   - **No existe** ruta `DELETE /api/v1/session-instances/:id`. El único
     `DELETE` de calendario es `/groups/:id/calendar/:date`.
   - `calendarService.DeleteDay` (línea 796) llama a `isCalendarDayClosed` y
     **rechaza** con `newCalendarClosedDaysError` cualquier día cerrado.
   - `isCalendarDayClosed` (línea 378) devuelve `true` para **todo** día con
     fecha anterior a hoy, así que ninguna sesión ya ocurrida se puede borrar.
   - El otro camino a `sessionInstanceDao.Delete` es `deleteSupersededInstance`,
     alcanzable solo desde `UpsertDay`, y la línea 755 lo **excluye**
     explícitamente cuando el nuevo `kind` es `cancelled` — en ese caso la
     línea 749 además **conserva** el `SessionInstanceID` en el día. Cancelar no
     borra la instancia.
   - Queda un solo caso límite: superscribir un día presencial **futuro**.
     No puede tener asistencias, porque las asistencias solo se crean para
     sesiones ya ocurridas (`is_presencial AND date <= CURRENT_DATE`).

   O sea: la FK es una red de seguridad, no una restricción que estorbe.

   **Acoplamiento a registrar:** la integridad de los datos de asistencia
   **depende** de que `isCalendarDayClosed` siga cerrando los días pasados. Si
   en el futuro se relaja ese invariante, o se expone un `DELETE` de
   `session_instance`, o se permite superscribir sesiones ya ocurridas, esta FK
   pasa a fallar en runtime y habría que revisarla. No es una constante: es una
   dependencia viva de este change con el módulo de calendario.
7. **Swagger** — `swag init -g cmd/api/main.go -d . -o cmd/api/docs --parseDependency --parseInternal`.

**Rollback.** Los pasos 3-6 son reversibles: `DROP CONSTRAINT`,
`ALTER TABLE ... DROP COLUMN` (perdiendo el backfill, que es regenerable). El
paso 2 **no es reversible** — es la única acción destructiva, y por eso va
primero y con preview. Como la API nueva no se despliega en el mismo paso, el
rollback de la app (quitar las rutas nuevas) no depende de nada de esto.

**Orden de despliegue**: migración primero, app después. La app nueva exige las
columnas `source`/`registered_by_user_id` (las lee en la grilla), así que
desplegarla antes rompería la lectura de la grilla.

## Open Questions

Ninguna que bloquee. Dos cosas que se pueden decidir en `/opsx-apply` sin tocar
specs ni el enfoque:

- Si conviene exponer también `roster_size` en la respuesta de
  `GET /attendance/search` (hoy solo devuelve filas crudas). Es un añadido
  opcional y ortogonal a este change.
- Si el endpoint de listado de sesiones debería aceptar un `from` opcional para
  paginar de a meses. Con `date <= hoy` y orden descendente, el caso de uso real
  (una sesión que el entrenador acaba de terminar) siempre cae en las primeras
  páginas.
