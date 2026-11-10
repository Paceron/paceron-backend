# Proposal — optimizaciones de lecturas y cache 2

## Why

La rama `optimizaciones-lecturas-cache` redujo round-trips en calendario, banners, equipos, invitaciones y push. Quedan lecturas internas que siguen trayendo más datos de los necesarios o repiten trabajo por request, sin cambios de contrato: agregaciones de logros, historial de feedback, resolución de instancias legacy del dashboard, nombres de calendario y algunos índices.

Además, el coverage actual (86.3%) puede mejorarse con tests de los caminos nuevos y de ramas hoy no ejercitadas, acercándonos a la meta gradual de 90% sin bajar el gate de 85.

## What

1. Resolver `early_N` de logros con una agregación SQL en timezone Córdoba, en lugar de traer todo el histórico de `runner_session.start_date` y contar en Go.
2. Reducir las lecturas del historial de feedback: total con `COUNT(*) OVER()` (fallback cuando la página viene vacía) y nombre del atleta en el join, evitando el batch separado de usuarios.
3. Acotar `UnlinkedExerciseInstances` al rango de fechas de la consulta del dashboard, para no escanear todo el histórico de instancias legacy.
4. Resolver `CalendarSummary` y los nombres de equipo de calendarios agregados en una sola consulta cada uno, sin cambiar shapes.
5. Limpiar índices duplicados/redundantes y agregar índices faltantes para borrados/consultas internas.
6. Sumar tests de cobertura en las áreas tocadas y en ramas internas de bajo costo, sin cambiar comportamiento.

## Alcance

- Solo backend; sin cambios de endpoints, DTOs, shapes, autorización ni mecanismos del frontend.
- Sin caché nuevo en esta rama; las mejoras son de consultas y trabajo redundante.
- Tests contra Postgres de test (`paceron-test-db:5433`). Prohibido local/demo/fullseed.
- No se toca `.testcoverage.yml`; el gate sigue en 85 y la meta es acercarse a 90.

## No-goals

- Paginación o límites de búsquedas existentes.
- Cota de rango de calendario.
- `WithContext` masivo, email async, lock de `CreateAttemptSession`, rachas incrementales.
- Agregación SQL completa de `FindSeries` o cache de datos mutables/por usuario.
- Cambios de contrato HTTP.
