# Proposal: gamificacion-core (Gaps 30/31/32/33/34)

## Why

El módulo de gamificación (doc fuente del frontend, `BACKEND_API_GAPS.md` Gaps 30-34) es la
próxima pieza de producto: rachas por corredor, logros con catálogo server-side + vitrina de
estampas, leaderboards por equipo, competencias con verificación de ruta y un mecanismo
transversal de "novedad por ver" (punto rojo). Nada de esto existe hoy en el backend; los datos
insumo (`runner_session`, `group_calendar_day`, `workout_feedback`, `workout_feedback_points`,
`attendance`) ya existen y están bien indexados para las lecturas nuevas.

## What

Un solo change en la rama `feature/gamificacion-core` con 4 etapas (stages) que se implementan
en orden y cada una deja la rama utilizable:

1. **E1 — Rachas (Gap 30) + novedades (Gap 34).** `GET /users/{id}/streaks`: racha estricta y
   flexible calculadas 100% bajo demanda en cada GET (sin tabla, sin recálculo en mutaciones),
   sobre días hasta AYER inclusive (el día de hoy nunca corta; aparece como `pending`).
   `?days=` default 42. Novedades: tabla `gamification_seen`, `has_unseen` embebido en los GETs
   de cada sección, `POST /users/{id}/gamification/seen` único.
2. **E2 — Logros (Gap 31).** Catálogo server-side: tabla `achievements` seeded de forma
   idempotente (15 logros `once` de la tabla definitiva del usuario; familia = metadata;
   `icon_key`). `GET /achievements/catalog`, `GET /users/{id}/achievements` (progreso +
   `has_unseen`), evaluación de desbloqueo bajo demanda vía hooks post-commit nil-safe, push
   best-effort al desbloquear. `GET /users/{id}/stamps` (vitrina; SOLO recibe emisiones de
   `type='competition'` — vacía hasta E4).
3. **E3 — Leaderboards (Gap 32).** `GET /teams/{id}/leaderboard?metric=&period=&from=&to=` con
   las 6 métricas cerradas en el consenso (distance/duration/pace/peak_speed/sessions_count/
   attendance_count), top 5 + `viewer_rank` + `record` solo en `period=all`. Desempate:
   menos entrenamientos que contribuyeron al valor; rank compartido a igual valor. Además,
   columna `peak_speed_kph` en `workout_feedback` (server-side al subir points; sin backfill).
4. **E4 — Competencias (Gap 33).** Tablas `competitions` + `competition_participants` +
   `competition_winners`; `runner_session.competition_id` nullable (FK opaca, mandada por el
   frontend en POST /runner al abrir el Play desde una competencia). CRUD + aprobación
   (`approve`/`reject` solo entrenador) + cancelación por el creador + cierre manual
   (`PATCH /competitions/{id}/close`; el auto-cierre programado queda DEFERIDO). Verificación
   de ruta server-side (Haversine con `acceptance_radius_meters`), empate = todos ganan
   (`winner_user_ids []`), sin umbral mínimo, ganador nulo si cero intentos. Trofeo
   (`stamps` `type='competition'`), logros de competencia, WS (`update:competition_progress` /
   `control:competition_finished`) y pushes. `has_unseen` por equipo embebido en
   `GET /teams/{id}/competitions`.

## Why 4 stages

El módulo es grande; dividido mantiene cada PR (o tramo de PR) utilizable y revisable. E1 no
tiene dependencias; E2 usa el cálculo de rachas de E1; E3 es lectura pura; E4 es la etapa con
más estado nuevo. Todas comparten el mecanismo de novedades de E1.

## Capabilities

Nuevas: `gamification-streaks`, `gamification-seen`, `gamification-achievements`,
`gamification-leaderboards`, `gamification-competitions`.
Modificadas: ninguna de forma breaking — el único endpoint existente que cambia es
`POST /session-instances/:id/runner`, que acepta un `competition_id` opcional (aditivo).

## No-goals

- Auto-cierre programado de competencias (regla aislada para el futuro).
- Niveles multi-threshold en el catálogo (Chispa→Fuego→...) — regla FUTURA documentada, no
  implementada; hoy todos los logros son `once` de un solo umbral.
- Familia `counter`, trofeos post-Leyenda y estampas distintas de la de competencia.
- Push notifications con contenido (solo avisos best-effort con `data.route`).
- Persistir `pace` (es una razón agregada del período, no un hecho persistible).
- Métricas nuevas v1 (volumen semanal, attendance_20, distance_250k/500k, early_10: fuera por
  decisión del usuario).
