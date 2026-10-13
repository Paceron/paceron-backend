# Spec Delta — gamification-seen

## ADDED Requirements

### Requirement: Persistencia de novedades no vistas

El sistema DEBE cumplir lo siguiente (MUST):

- Tabla `gamification_unseen`: id, user_id, section varchar(32) con valores `streaks`,
  `achievements` o `competitions`, team_id nullable (solo con sentido en `competitions`),
  created_at. Index `(user_id, section)`.
- El punto rojo del frontend se deriva de la existencia de filas: no hay estado adicional.
- Tabla `gamification_streak_state`: user_id PK, strict_current, flexible_current,
  updated_at. SOLO alimenta los triggers de notificación; el GET de rachas es siempre exacto
  (recalcula fresco) y nunca la consulta.

#### Scenario: Alta de unseen por reset de racha

- **WHEN** el trigger de racha detecta que la racha de un usuario pasa de mayor que 0 a 0
- **THEN** se inserta una fila en `gamification_unseen` con section `streaks`.

#### Scenario: El GET de rachas no usa el estado de trigger

- **WHEN** se consulta `GET /users/{id}/streaks`
- **THEN** el servicio recalcula las rachas con su lógica batch propia y no consulta
  `gamification_streak_state`.

#### Scenario: Racha intacta no genera novedad

- **WHEN** un write de runner session sucede y la racha del usuario no cambia de valor ni
  cruza un hito
- **THEN** no se inserta ninguna fila en `gamification_unseen`.

### Requirement: Confirmar novedades vistas (POST seen)

El sistema DEBE cumplir lo siguiente (MUST):

- `POST /users/{id}/gamification/seen` con body `{section, team_id?}` disponible; self-only
  (`{id}` distinto del caller responde `403`).
- `section` valida contra la whitelist `streaks|achievements|competitions`; otro valor
  responde `400`.
- `team_id` solo se admite con `section=competitions`; en otra sección responde `400`.
- El POST borra las filas de `gamification_unseen` del usuario para esa sección: con
  `team_id` filtra por team (solo competitions); sin `team_id` limpia todas las filas de la
  sección del usuario.
- Cero filas para borrar es un éxito no-op (`200`/`204`).

#### Scenario: Visto streaks

- **WHEN** el usuario marca visto `section=streaks`
- **THEN** se borran TODAS sus filas unseen de la sección streaks (de cualquier origen).

#### Scenario: Visto competitions con team

- **WHEN** el usuario marca visto `section=competitions` con `team_id` de su equipo
- **THEN** solo se borran sus filas unseen de competitions de ese team.

#### Scenario: Visto competitions sin team

- **WHEN** el usuario marca visto `section=competitions` sin `team_id`
- **THEN** se borran todas sus filas unseen de competitions (de cualquier team).

#### Scenario: Section inválida

- **WHEN** se envía `section=logros`
- **THEN** responde `400`.

#### Scenario: team_id en sección sin sentido

- **WHEN** se envía `section=achievements` junto con `team_id`
- **THEN** responde `400`.

#### Scenario: Visto ajeno

- **WHEN** el usuario A intenta marcar visto para `{id de B}`
- **THEN** responde `403` sin tocar el DAO.

### Requirement: Novedades por triggers de racha

El sistema DEBE cumplir lo siguiente (MUST):

- El write de runner session hacia `finished` o `interrupted` ejecuta post-commit un hook
  nil-safe: calcula la racha current del usuario antes y después del write con la misma
  lógica batch del GET, y actualiza `gamification_streak_state`.
- Si una racha (estricta o flexible) pasa de mayor que 0 a 0, inserta unseen de section
  streaks.
- Si una racha cruza un hito de 10, 20, 50 o 100 hacia arriba (valor anterior menor que el
  hito y nuevo mayor o igual), inserta unseen de section streaks.
- El create de sesión wip y los cambios del calendario NO disparan el trigger.
- La deriva del estado de trigger es la degradación aceptada: el GET exacto nunca depende de
  esta tabla (ver design D14).

#### Scenario: Racha rota genera unseen

- **WHEN** el corredor pasa su sesión de hoy a `interrupted` y eso corta su racha estricta
  con current mayor que 0
- **THEN** post-commit se inserta unseen section streaks.

#### Scenario: Hito alcanzado genera unseen

- **WHEN** el write sube la racha flexible de 9 a 10
- **THEN** post-commit se inserta unseen section streaks (hito 10).

#### Scenario: Cruza de varios hitos en un solo write

- **WHEN** una correccion masiva de sesiones pasadas sube el current de la racha de 15 a 55
- **THEN** se insertan unseen que reflejan la cruza de los hitos 20 y 50 hacia arriba; la
  regla general es old < hito y new >= hito para cada hito de 10/20/50/100.

#### Scenario: Cambio de estado wip no dispara

- **WHEN** se crea una runner session en `wip` para un intento chico
- **THEN** no corre el trigger ni actualiza el estado de rachas.

### Requirement: has_unseen embebido en los GETs de sección

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /users/{id}/streaks`, `GET /users/{id}/achievements` (E2) y
  `GET /teams/{id}/competitions` (E4) embeben `has_unseen: bool`.
- En `streaks` y `achievements` la marca es global del usuario para esa sección; en
  `competitions` filtra por el team_id de la consulta.
- El catálogo (`GET /achievements/catalog`) NO embebe has_unseen (es catálogo compartido, no
  estado por usuario).

#### Scenario: Con unseen pendiente

- **WHEN** existe una fila unseen de la sección para el usuario
- **THEN** el GET de esa sección responde `has_unseen: true`.

#### Scenario: Sin unseen pendiente

- **WHEN** el usuario ya marcó visto esa sección y no hubo novedades nuevas
- **THEN** el GET responde `has_unseen: false`.

#### Scenario: Marca por team en competencias

- **WHEN** existen filas unseen de competitions para el team A pero no para el team B
- **THEN** `GET /teams/A-id/competitions` responde `has_unseen: true` y el mismo endpoint
  para el team B responde `has_unseen: false`.
