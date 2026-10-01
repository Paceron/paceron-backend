# Proposal: workout-feedback-history

## Por qué

La pestaña "Historial" del frontend (`trainings-history-tab.jsx`) está en stub: no existe forma de listar los entrenamientos realizados (feedbacks) de un atleta ni de los atletas que administra un entrenador. El endpoint existente `GET /workout-feedback/search` devuelve el modelo crudo sin paginación, sin orden, sin nombres de grupo/sesión/ejercicio y sin los pools de atletas/ejercicios que la UI necesita, y su matriz de autorización (athlete-or-owner) no calza con la del historial (self-only / entrenador+equipo obligatorio).

Este es el change 2/3 del sub-proyecto de calendario que pide el frontend (`BACKEND_API_GAPS.md` Gap 13).

## Qué cambia

Dos endpoints nuevos, sin tocar `GET /workout-feedback/search` (queda vivo; el frontend indica que no lo usa hoy, se conserva por las dudas):

1. **`GET /users/{id}/workout-feedback-history`** (corredor): `{id}` debe ser el propio usuario (403 si no). Filtros: `team_id`, `group_id` (400 si falta `team_id`), `date_from`/`date_to` (400 si viene uno solo o from>to; iguales = un día), `exercise_id` (id de instancia), `set_number`. Orden: `sort` ∈ {`feedback_date`, `set_number`, `exercise_name`} (default `feedback_date`), `order` ∈ {`asc`,`desc`} (default `desc`). Paginación `page` (1) / `page_size` (20, tope 100). Sin filtros: historial completo del atleta en todos sus equipos.

2. **`GET /users/{id}/administered-workout-feedback-history`** (entrenador): `{id}` debe ser el propio usuario (403); `team_id` obligatorio (400) y debe administrarlo (404 si no existe, 403 si no es owner). Mismos filtros + `athlete_user_id`.

**Response (ambos):** `200 {items, total, page, page_size, available_athletes, available_exercises}` con ítems enriquecidos (athlete_name, team_name, group_name, session_name, exercise_name) y pools `available_*` = DISTINCT sobre los que matchean solo los filtros de primer nivel (equipo/grupo/rango de fechas), sin filtro de segundo nivel ni paginación.

## Decisiones ya confirmadas con el usuario

- `exercise_id` de los ítems = **id de instancia** (`assigned_exercise_id`); se suma además `catalog_exercise_id` (nullable, desde `exercise_instances.source_exercise_id`) para agrupar por familia.
- **Los feedbacks huérfanos se conservan** en el historial: `group_id`/`group_name` `null` indica día de calendario borrado o instancia nunca asignada a un día; `team_id`/`team_name` `null` indica feedback registrado sin equipo. No se agrega campo extra de razón (los nulls ya lo comunican).
- Sumarización server-side FUERA de alcance (mejora futura).

## Impacto

- **Aditivo**: 2 rutas nuevas, DAO/service/controller nuevos métodos. Ningún endpoint existente cambia de shape.
- Capability nueva: `workout-history`.
- Coverage gate hoy en 85%: el código nuevo entra testeado (DAO con Postgres real + service + controller).
