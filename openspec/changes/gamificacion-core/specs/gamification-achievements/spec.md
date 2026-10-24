# Spec Delta — gamification-achievements

## ADDED Requirements

### Requirement: Catálogo de logros versionado en constants

El sistema DEBE cumplir lo siguiente (MUST):

- El catálogo de logros vive como lista versionada en constants Go (NO en una tabla). Cada
  fila es un logro `once` de un solo threshold; la familia es metadata de agrupación (la
  escalera multi-threshold es regla futura, fuera de v1).
- Campos por fila: `{id, family, name, description, icon_key, threshold, unit}`.
- Lista completa de 21 filas: la tabla definitiva del frontend define 15 (consistency 6 +
  volume 4 + sessions 5) y los bloques de familia restantes suman su resto (gps 3 + early 1
  + team 2):
  - consistency (6): first_session(1), streak_3(3), streak_7(7), streak_14(14),
    streak_30(30), streak_60(60).
  - volume (4): distance_10k(10 km), distance_50k(50 km), distance_100k(100 km),
    distance_42k_single(42.2 km en una sesión).
  - sessions (5): sessions_5, sessions_25, sessions_50, sessions_100, sessions_200.
  - gps (3): gps_first(1), gps_10(10), gps_50(50).
  - early_bird (1): early_5(5).
  - team (2): team_first(1), team_10(10).
- Los extras (early_10, distance_250k/500k, attendance_20) quedan FUERA de v1.
- `GET /achievements/catalog` disponible para cualquier usuario autenticado (sin ownership).
- Respuesta: las 21 filas con todos sus campos.

#### Scenario: Catálogo completo

- **WHEN** un usuario autenticado llama `GET /achievements/catalog`
- **THEN** responde `200` con 21 filas, cada una con id/family/name/description/icon_key/
  threshold/unit.

#### Scenario: Catálogo sin ownership

- **WHEN** cualquier usuario autenticado llama al catálogo (sin importar a qué equipos
  pertenece)
- **THEN** responde `200` igual (no hay guard de recurso porque el catálogo es compartido).

#### Scenario: Un logro un umbral

- **WHEN** se revisa una fila del catálogo (por ejemplo streak_7)
- **THEN** es un logro único de un solo threshold (7), sin niveles previos ni posteriores
  modelados.

#### Scenario: Familia como metadata

- **WHEN** el front agrupa la vitrina por familia
- **THEN** puede hacerlo leyendo `family` de cada fila; el backend no modela jerarquía por
  familia en la evaluación.

### Requirement: Progreso y evaluación de desbloqueo

El sistema DEBE cumplir lo siguiente (MUST):

- Tabla `user_achievement_progress`: user_id, achievement_id varchar(64), unlocked_at,
  created_at. PK compuesta `(user_id, achievement_id)`; index user_id.
- Motor de evaluación: hooks post-commit en controllers, nil-safe y asíncronos; cada trigger
  re-evalúa los logros de su mapa de criterios.
- Evaluación idempotente por unique `(user_id, achievement_id)`: re-evaluar un logro ya
  desbloqueado no inserta ni actualiza nada.
- Criterios:
  - first_session y sessions_N: COUNT histórico de runner sessions finished.
  - distance_10k/50k/100k: SUM distance_meters de feedbacks.
  - distance_42k_single: MAX(SUM distance_meters por session_instance_id) >= 42.2 km (una
    query GROUP BY sobre las sesiones de corredor).
  - streak_N: current O best de racha estricta (design D2) >= N.
  - gps_N: COUNT de runner sessions finished con puntos GPS subidos.
  - early_N: COUNT de runner sessions finished con started_at antes de las 08:00
    America/Argentina/Cordoba.
  - team_N: COUNT de días distintos con alguna asistencia confirmada registrada (criterio A).

#### Scenario: Primer desbloqueo por sesiones

- **WHEN** el corredor termina su quinta sesión finished de la historia
- **THEN** post-commit el evaluador inserta `(su user_id, sessions_5)` con unlocked_at.

#### Scenario: Re-evaluación idempotente

- **WHEN** el evaluador corre de nuevo para un logro ya desbloqueado
- **THEN** no inserta una segunda fila (PK compuesta) ni altera unlocked_at.

#### Scenario: Umbral no alcanzado

- **WHEN** el corredor lleva 3 sesiones finished
- **THEN** `sessions_5` no se desbloquea: no hay fila en user_achievement_progress para ese
  logro.

#### Scenario: Distance por sesión única

- **WHEN** el corredor termina una sesión cuyas series acumulan 43 km (por feedback)
- **THEN** `distance_42k_single` se desbloquea aunque su total histórico sea menor.

#### Scenario: Rachas alimentan consistency

- **WHEN** la racha estricta del corredor llega a 3 (current o best histórico)
- **THEN** `streak_3` se desbloquea reusando el cálculo de rachas del design D2.

#### Scenario: Sesiones tempranas

- **WHEN** el corredor empieza (started_at) su quinta sesión antes de las 08:00 de Cordoba
- **THEN** `early_5` se desbloquea.

### Requirement: Endpoints de logros y estampas

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /users/{id}/achievements` self-only (`403` para otro caller): merge de catálogo +
  progreso; shape `{achievement_id, current_level?(null en v1), count?(null en v1),
  unlocked_at, last_unlocked_at}`; current_level y count llegan en `null` en v1; los logros
  no desbloqueados incluyen `unlocked_at: null`. Con `has_unseen` de la sección achievements
  embebido.
- `GET /users/{id}/stamps` (vitrina): lista de estampas del usuario; en v1 SOLO recibe
  emisiones de `type=competition`, así que la tabla `gamification_stamp` queda vacía hasta
  E4 y el GET responde lista vacía (mismo guard self-only).

#### Scenario: Vitrina propia con mix de desbloqueos

- **WHEN** el corredor con 2 logros desbloqueados consulta su lista de achievements
- **THEN** responde `200` con las 21 filas, 2 con unlocked_at seteado y current_level/count
  en null, y 19 con unlocked_at null.

#### Scenario: Vitrina ajena

- **WHEN** el usuario A consulta `GET /users/{id de B}/achievements`
- **THEN** responde `403` sin tocar el DAO.

#### Scenario: Estampas vacías hasta E4

- **WHEN** el corredor consulta `GET /users/su-id/stamps` en E2 (antes de que existan
  competencias)
- **THEN** responde `200` con lista vacía.

### Requirement: Push best-effort al desbloquear

El sistema DEBE cumplir lo siguiente (MUST):

- Al desbloquear un logro, el backend envía un push vía `expopushclient` best-effort con
  `data.route "/achievements"` (placeholder).
- El fallo del proveedor de push se loguea y se ignora: nunca rompe el flujo de respuesta ni
  la transacción del desbloqueo.
- Sin push en otros eventos de gamificación en E2.

#### Scenario: Desbloqueo con push

- **WHEN** el evaluador desbloquea `sessions_5`
- **THEN** se envía un push best-effort al usuario con `data.route "/achievements"`.

#### Scenario: Fallo silencioso del proveedor

- **WHEN** el proveedor de push rechaza o falla el envío
- **THEN** se loguea el error y el desbloqueo y la respuesta del endpoint original quedan
  intactos.
