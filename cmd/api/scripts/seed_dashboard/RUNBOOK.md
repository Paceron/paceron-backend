# Seed del dashboard analítico

Sembrea el dataset que el dashboard de rendimiento necesita: 13 ejercicios de
catálogo, 4 plantillas de sesión, 2 planes de 7 días estampados por el mecanismo
de calendario, y el feedback de 7 corredores durante 2 semanas.

No es un `INSERT` masivo: **el calendario se siembra con `POST .../calendar/stamp`**
(vía `services.CalendarService.Stamp`), que es lo que crea las
`session_instances` / `exercise_instances` / `session_exercise_instances`. Sin esa
instanciación el dashboard no tiene nada que leer.

## Requisitos

1. `SEEDING_ENABLED=true` en `.env.local` (es lo que permite estampar fechas
   pasadas; sin la variable el calendario cierra toda fecha anterior a hoy).
2. Correrlo con `--stage=local`. `config.AssertSeedingStage()` aborta el proceso
   con un panic si la bandera está activa sin ese flag —es el mismo guard que
   protege a la app en arranque, no una copia.
3. Postgres con el schema migrado (`go run ./cmd/api --stage=local` una vez, o
   `make test-db-up` para la base descartable).

## Cómo se corre

```bash
# 1. Ver qué haría, sin escribir nada.
SEEDING_ENABLED=true go run ./cmd/api/scripts/seed_dashboard --stage=local

# 2. Sembrar.
SEEDING_ENABLED=true go run ./cmd/api/scripts/seed_dashboard --stage=local --seed

# 3. Ver qué borra la limpieza, sin borrar.
SEEDING_ENABLED=true go run ./cmd/api/scripts/seed_dashboard --stage=local --cleanup --dry-run

# 4. Limpiar (pide --yes explícito).
SEEDING_ENABLED=true go run ./cmd/api/scripts/seed_dashboard --stage=local --cleanup --yes
```

O con el target de Make (agrega `--stage=local` y deja pasar `SEED_FLAGS`):

```bash
make seed-dashboard                      # informa
make seed-dashboard SEED_FLAGS=--seed    # siembra
```

`--stage=local` está declarado como flag vacío en el script: `flag.Parse()`
aborta con cualquier flag que no conozca, y sin esa entrada no se podría pasar.

### Flags

| Flag | Default | Qué hace |
|---|---|---|
| `--seed` | off | Sin el flag solo informa el scope. Con el flag siembra. |
| `--dry-run` | off | No escribe nada, pase lo que pase. |
| `--cleanup` | off | Modo limpieza (incompatible con `--seed`). |
| `--yes` | off | Confirmación explícita, obligatoria para borrar. |
| `--start` | dos semanas completas anteriores | Primer día de la ventana, **tiene que ser un lunes** (`YYYY-MM-DD`). |
| `--force` | off | `StampRequest.Force`: sobrescribe días de calendario que ya existen. |
| `--bootstrap-scope` | off | Crea equipo, grupo, entrenador y corredores si el scope no existe. |
| `--db` | el del stage | DSN alternativo, para sembrar una base descartable. |
| `--team-id` / `--group-id` | `29` / `39` | Scope a sembrar, para reutilizar el seed en otro equipo. |

### Ventana

Por default son **las dos semanas completas que terminan la semana pasada**: el
lunes de hace dos semanas hasta el domingo de la semana pasada, con 4 sesiones
(martes y jueves de cada semana). Terminar la semana pasada y no la que viene es a
propósito: el stamp no puede escribir el día en curso (el flag de seeding no abre
esa rama) y el dashboard muestra `date_range`, así que un seed cuya última sesión
es de hace un mes se ve como un equipo abandonado.

Si el rango tiene días ocupados por otro motivo, el seed **aborta nombrando la
fecha** en vez de pisarlos: se elige otra ventana con `--start`, o se pasa `--force`.

`--force` tiene un costo que conviene conocer: el stamp reemplaza el día por uno
nuevo con su propia sesión instancia, y **el feedback que tenía ese día deja de
estar visible en el dashboard** (la instancia vieja queda protegida del borrado,
pero el calendario ya no la apunta). Para sembrar la base de desarrollo es lo
correcto; para una base con datos reales, no.

## Qué produce

### Catálogo (13 ejercicios)

| Tipo | Ejercicios |
|---|---|
| `liviano` | Sparring (10 min / 1,2 km), Trote Vuelta a la Calma (15 min / 1,2 km) |
| `elongacion` | Elongación (10 min, con `muscle_group`) |
| `ritmo_continuo` | Distancia 4 / 6 / 8 / 10 / 21 km |
| `velocidad_distancia` | Pasadas 1000 (4 min), Pasadas 2000 (10 min), Velocidad Máxima 100 m y 200 m (sin minutos) |
| `velocidad_tiempo` | 1 min a máxima potencia (1 min, 400 m) |

"Calentamiento" se llama **Sparring**: el rol de un bloque lo define
`session_exercise_instances.role`, no el nombre del ejercicio.

Velocidad Máxima va como `velocidad_distancia` con `minutes` en `null` porque 100 m
son ~12 s = 0,2 minutos, que no es representable en un `INT` de minutos
(redondeado da 0). Por eso un sprint de 100/200 m no puede ser el objetivo de un
`velocidad_tiempo`: ese tipo exige minutos enteros. El seed agrega **1 min a
máxima potencia** (1 min / 400 m) como el único `velocidad_tiempo`, que es lo que
hace que el filtro por ese tipo tenga series y velocidad esperada (24 km/h).

### Planes y sesiones

Dos planes de 7 días ("Semilla dashboard · Semana 1/2"), con martes y jueves de
entrenamiento presencial y el resto de descanso (los días de descanso van en
`exclude_dates`). Cada sesión tiene la estructura Sparring → Elongación → bloque
principal → Vuelta a la Calma; desde 8 km el bloque principal es el único.

### Las seis cuentas de `data_quality`

El seed **rompe los datos a propósito**. Un dataset donde todo está limpio se ve
igual de lindo en la pantalla y no ejercita ninguna de las ramas que el change
especificó, así que el generador tiene un plan de rupturas explícito
(`defectPlan` en `generator.go`) y la corrida **compara** los contadores que
devuelve el dashboard contra los que el plan esperaba.

| Contador | Qué lo produce el seed | Valor |
|---|---|---|
| `invalid_series` | 2 `ritmo_continuo` sin `distance_meters` + 2 sin ningún tiempo | **4** |
| `degraded_time` | 5 series sin `active_duration_ms` (caen al fallback de `duration_ms`) **+ las 2 sin tiempo**, que también están degradadas | **7** |
| `unknown_status` | 3 series con `completion_status` fuera del vocabulario (o `null`) | **3** |
| `open_sessions` | 2 sesiones del último día con `runner_session.status = 'wip'` | **2** |
| `legacy_sessions` | 2 corredores sin fila `runner_session` (19 series válidas) | **19** |
| `insufficient_data` | el corredor "en progreso", con 1 sola sesión: se ve filtrando por él | `false` en el scope, `true` por corredor |

`legacy_sessions` es el único contador que **cambia después de la corrida**: el
seed deja sin `source_exercise_id` las instancias del día 0, así que su propio
verify (inmediatamente después de sembrar, sin reiniciar el backend) compara
**89/89** por el fallback de nombre. Al reiniciar, el backfill las enlaza y el
dashboard estable baja a **19**. La tabla de arriba muestra el valor estable
(post-backfill), que es el que se ve en la UI; no es el que imprime el verify
del seed en esa primera corrida.

Si la comparación falla, el seed sale con error en vez de terminar: un dataset
cuyo `data_quality` no es el que el plan dice es un bug, no una sorpresa.

### Corredores

7 corredores, con arquetipo asignado por posición en el roster (ordenado por
`user_id`): 2 velocistas, 2 fondistas, 1 resistente, 1 regular y 1 "en progreso".
El generador aplica perfil, progresión semanal, fatiga dentro del ejercicio y
±3-7 % de ruido, con ~10 % de las series fuera de objetivo. La semilla del `rand`
es fija (`seedRandomSeed`), así que el dataset es reproducible.

Dos de los siete quedan **sin fila `runner_session`** (el feedback es anterior al
feature de registro de sesión) y uno aparece **en `interrupted`**.

### Instancias sin `source_exercise_id`

Al final el seed deja en `NULL` el `source_exercise_id` de las instancias del
**primer día** de la ventana, para que el fallback por nombre normalizado
(`/exercises/{id}/series`, tarea 4.1b) tenga algo que resolver.

Ojo con el ciclo de vida: el backfill del ítem 11 de `postgres.go` corre en **cada
arranque** del backend y llena esas filas por nombre —que es justamente para lo que
existe—. Después del primer arranque, esas instancias vuelven a tener su
referencia y `instancias_sin_source_exercise_id` da 0 hasta la próxima corrida del
seed. No es un bug del seed: es el backfill funcionando.

## Limpieza

`--cleanup` borra, en orden inverso de FK y dentro de una transacción (o se va
todo, o no se borra nada):

```
runner_session → workout_feedback_points → workout_feedback
→ session_exercise_instances → exercise_instances → session_instances
→ group_calendar_days → plan_days → training_plans
→ session_exercises → sessions → exercises
```

`runner_session` va **primero** a propósito: el seed crea esas filas en
`finished`, y borrarlas al final dejaría filas apuntando a `session_instances`
inexistentes.

Todo está acotado a **team + grupo + ventana temporal + marcador del seed**
(`description` con `[seed_dashboard]`). Dos protecciones extra: una sesión
instancia compartida con otro día, o un ejercicio de catálogo usado por una sesión
que no es del seed, **no se borran**.

### Lo que la limpieza NO toca: la membresía

`group_users` no se borra. El seed **solo mueve `date_start`** hacia atrás, para
que la primera sesión de la ventana esté dentro de la ventana de membresía (sin eso
el calendario del corredor sale vacío aunque el día exista). Restaurarlo a mano es
menos riesgoso que automatizarlo, y el valor anterior de cada corredor queda en el
log del seed cuando lo mueve:

```
seed dashboard: membresía movida para cubrir la ventana
  group_id=39 user_id=1570 date_start_previo=2026-10-02 date_start_nuevo=2026-09-28
```

Si querés el estado previo de la base: `make demo-restore`.

Con `--bootstrap-scope` además quedan los usuarios sintéticos y sus `team_users`
(`...@seed.local`, dominio reservado que no resuelve en DNS). Son scope, no
dataset: la limpieza no los borra.

## Verificación contra una base descartable

`--db` permite sembrar la base de tests sin tocar la de trabajo:

```bash
make test-db-up
SEEDING_ENABLED=true go run ./cmd/api/scripts/seed_dashboard --stage=local \
  --db "postgres://postgres:postgres@localhost:5433/paceron_test" --bootstrap-scope --seed
```

`--bootstrap-scope` crea el equipo, el grupo, un entrenador y 7 corredores
sintéticos. Los ids del scope (`29`/`39`) se insertan explícitos, así que una base
vacía funciona sin tocar una constante.

Los tests del paquete hacen exactamente ese camino (`seed_test.go`), así que
`make test-with-db` verifica que el dataset sigue produciendo los seis contadores.

**Ojo:** la base de test es compartida con la suite, y hay tests que cuentan filas
sin filtrar (`TestWorkoutFeedbackDao_History_NoScopeReturnsAllActive`, entre
otros). Después de sembrar a mano contra `paceron_test`, corré el `--cleanup --yes`
—o `make test-db-restart`— antes de `make test-with-db`, o la suite falla por datos
que el seed dejó, no por código.

## Perfil full (`--profile=full`)

Perfil aparte del dataset del dashboard: un universo sintético de todo el producto
(identidad y suscripciones, equipos y grupos, calendario, actividad, gamificación,
competencias, pagos y mensajes), con fechas relativas a hoy en
`America/Argentina/Cordoba`. El perfil `dashboard` de arriba no cambia. Diseño y
contrato: `openspec/changes/seed-perfil-full/` (D12–D14).

Es datos **100% sintéticos**: no llama a Mercado Pago, Expo ni storage.

### Requisitos

En `.env.local` (con `--stage=local` pisa a `.env`):

| Variable | Valor | Para qué |
|---|---|---|
| `SEEDING_ENABLED` | `true` | Sin ella el stamp rechaza fechas pasadas y el seed no arranca. |
| `MP_OAUTH_CLIENT_ID` | client_id de la app MP (cualquiera sirve, no se llama a MP) | Queda como `client_id` de la conexión de seller del entrenador 1. Sin valor, `seedTeams` aborta. |
| `TOKEN_ENCRYPTION_KEY` | 16, 24 o 32 bytes; p. ej. `openssl rand -hex 16` | Cifra los tokens sintéticos de `seller_connections`. Sin valor, `seedTeams` aborta. |

Además, Postgres local arriba (`make local-up`). El seed migra el schema al
conectarse, así que no hace falta correr el backend antes.

### Comandos

Son los targets de `Makefile` (`fullseed-*`); los `seed-dashboard*` no se tocan.

| Target | Equivale a | Qué hace |
|---|---|---|
| `make fullseed-seed` | `go run ./cmd/api/scripts/seed_dashboard --stage=local --profile=full --seed --bootstrap-scope` | Siembra el perfil y al final corre la verificación. |
| `make fullseed-verify` | `... --profile=full --verify` | Verifica el universo: sale con error si algo no es coherente. |
| `make fullseed-cleanup` | `... --profile=full --cleanup --yes` | Borra solo lo marcado `[seed_full]`, en una transacción. |
| `make fullseed-reset` | `make local-reset` + `make fullseed-seed` | Ver la advertencia de abajo: hoy no deja la base lista. |

`--bootstrap-scope` está en el target pero el perfil full no lo usa (la creación
del scope mínimo es del dashboard); es inocuo.

Flujo típico:

```bash
make local-up
make fullseed-seed        # siembra y verifica
make fullseed-verify      # re-verificar cuando quieras
make fullseed-cleanup     # vuelve a dejar el perfil fuera de la base
```

### Advertencia: base reseteada

`make local-reset` ejecuta `docker compose down -v`: baja los contenedores **y borra
los volúmenes** (base y bucket). Se pierde todo lo local, incluido lo clonado de
Supabase; si lo querés conservar, `make local-dump` antes.

`local-reset` no vuelve a levantar la base, así que `make fullseed-reset` termina con
Postgres apagado y el seed falla al conectar. Mientras el target no se corrija, el
flujo que funciona es:

```bash
make local-reset
make local-up             # esperar a que db esté healthy: make local-ps
make fullseed-seed
```

Sembrar sobre una base que ya tiene el perfil es esperado (las fases están pensadas
para re-ejecutarse), pero las fechas se recalculan y el scope ya está; para un estado
limpio, `make fullseed-cleanup` antes o el reset de arriba.

### Seguridad

- Solo corre con `--stage=local` y `SEEDING_ENABLED=true`
  (`config.AssertSeedingStage`).
- Los IDs de scope están reservados: equipos `1001`–`1006`, grupos `2001`–`2007`. Si
  esos IDs existen sin el marker `[seed_full]`, aborta antes de tocar nada.
- Cleanup exige `--yes` y solo borra filas del scope con marker.
- Las credenciales son sintéticas y conocidas (ver abajo): no usar con una base que
  tenga usuarios reales.

### Usuarios

Contraseña **de todos**: `Paceron.123`. Email: `<clave>.seed-full@seed-full.local`.

| Usuario | Email | Rol / tier | Estado |
|---|---|---|---|
| Lucía Entrenadora (`trainer_1`) | `trainer1` | entrenador base + corredor base | active |
| Mateo Entrenador (`trainer_2`) | `trainer2` | entrenador base + corredor base | active |
| Corredor 1 (`r1`) | `runner1` | corredor premium | active |
| Corredor 2 (`r2`) | `runner2` | corredor premium | active |
| Corredor 3 (`r3`) | `runner3` | corredor medium | active |
| Corredores 4–12 (`r4`–`r12`) | `runner4`…`runner12` | corredor base | active |
| Corredor Bloqueado | `blocked` | corredor base | blocked (no puede loguear) |
| Corredor Inactivo | `inactive` | corredor base | inactive (no puede loguear) |
| Sin Rol | `no-role` | sin rol | active |
| Invitaciones Desactivadas | `invitations-off` | corredor base | active, `allow_team_invitations=false` |

Los emails van con guiones, p. ej. `no-role.seed-full@seed-full.local`.

### Equipos y grupos

| Equipo (id) | Grupos | Entrenador | Particularidad | Corredores |
|---|---|---|---|---|
| A Normal (1001) | principal 2001, secundario 2002 | Lucía | Sin cuota, cupo 20 | r1–r9 |
| B Pago (1002) | 2003 | Mateo | Cuota 5000 | r1, r2, r6, r7, r8, r11 |
| C Lleno (1003) | 2004 | Lucía | Cupo 2, ya completo | r10 |
| D Oculto (1004) | 2005 | Mateo | No aparece en búsqueda | r11 |
| E Cerrado (1005) | 2006 | Mateo | No acepta solicitudes | r12 |
| F Colisión (1006) | 2007 | Lucía | Cuota 4500, colisiona con A | r10 |

Dentro de A: en el secundario r4 y r5 activos, r8 con membresía vencida, r9 con alta
futura. r10 y r11 pertenecen a dos equipos cada uno (C y F; B y D).

### Casos que cubre

- **Identidad**: login de protagonistas; `blocked` e `inactive` rechazados;
  `GET /auth/permissions` completo por tier.
- **Suscripciones y cuotas**: R1 premium activa con 2 cuotas pagas y la 3.ª pendiente;
  R2 premium en deuda (bloquea la baja del equipo); R3 medium activa y una premium en
  `first_payment_pending`.
- **Solicitudes e invitaciones**: vigente, expirada, aceptada y rechazada; invitación a
  un usuario con invitaciones desactivadas; join requests pendientes a A, E y C (cupo
  lleno) y una rechazada en B.
- **Calendario**: pasado (-21 a -1) con stamp real, futuro (+1 a +14) con training
  presencial y asincrónico, descanso y cancelado con motivo. Hoy: presencial abierto
  en A, presencial no abierto en el secundario, presencial cerrado en F y asincrónico
  en B. Colisión presencial A vs F.
- **Actividad**: runner sessions finished, wip de hoy e interrumpida; feedback con
  todos los `completion_status` (uno soft-deleted); puntos GPS para pace/peak;
  asistencias `qr` y `manual` con no-confirmados.
- **Gamificación**: racha de 10 días para R4 (strict y flexible), racha cortada en 4
  para R5 y un corredor con días skipped; logros desbloqueados; estampas y punto
  rojo de `unseen`; leaderboard de las 6 métricas.
- **Competencias**: pendiente de aprobación, activa por métrica, activa por ruta
  (con intentos parciales y completos), terminada con empate, terminada con ganador
  por duración, terminada sin intentos, rechazada con motivo y una con límite de
  intentos agotado.
- **Pagos y mensajes**: cuotas, pagos rechazados, cobros del entrenador con split
  (`marketplace_fee`, `net_received_amount`), mensajes de sesión por modo y push tokens
  sintéticos.

`fullseed-verify` comprueba estos casos contra los servicios reales: si algo no
coincide, sale con error en lugar de dejar un dataset incoherente.