# Gap 14 — GET /session-instances/{id} (detalle de instancia standalone)

## Por qué

El historial de entrenamientos (Gap 13) expone `session_instance_id` por ítem, pero no existe forma de pedir la instancia completa a partir de solo ese id. La pantalla "ver/editar registro" del frontend arma la lista de ejercicios a revisar a partir del objeto completo de la instancia; los demás puntos de entrada a esa pantalla traían la instancia embebida desde calendario. Este es el primer caso de entrada solo-id.

## Qué cambia

- Nuevo `GET /api/v1/session-instances/{id}`: devuelve el shape completo de `SessionInstanceResponse` (el mismo que el calendario embebe como `session_instance`): `{id, session_id, name, description, created_at, exercises: [...]}` con el detalle completo por ejercicio (kind, intensity, minutes, distance, speed, muscle_group, video_url, role, repeat_count, rest_minutes, exercise_id origen catálogo).
- Autorización dual (decisión aprobada): el caller accede si
  1. existe un `group_calendar_day` con esa instancia y es miembro activo del grupo (atleta) u owner del equipo del grupo (entrenador), **o**
  2. existe un `workout_feedback` activo sobre la instancia y es atleta del feedback, reportante u owner del equipo del feedback.
- `404` si la instancia no existe; `403` si existe sin acceso.

## Impacto

- Aditivo: no toca ningún endpoint ni shape existente (reutiliza el builder `sessionInstanceResponse` de `calendar_service`).
- Sin migración de DB.

## No-goals

- Sumarización de historial (deferida aparte desde Gap 13).
- Modificar `GET /session-instances/:id/runner` ni `/:id/feedback`.
