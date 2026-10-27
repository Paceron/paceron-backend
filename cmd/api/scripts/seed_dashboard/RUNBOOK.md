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