# Proposal

## Why

La app ya graba datos reales de entrenamiento — `workout_feedback` tiene `duration_ms`, `active_duration_ms`, `distance_meters`, FC, cadencia, desnivel y RPE por serie — pero **nadie puede verlos agregados**. Verificado sobre el mapa completo de rutas: cero endpoints de `stats`/`metrics`/`dashboard`/`analytics`. Lo único cercano es `GET /users/{id}/workout-feedback-history`, y no sirve para un dashboard:

- Page size máximo 100, y **no expone** RPE, FC, cadencia ni desnivel (ver `workout_feedback_history.go:18-36`) — hay que hacer N requests por serie.
- La whitelist de sort es `feedback_date`/`set_number`/`exercise_name`: **no se puede ordenar por `duration_ms` ni por distancia** (`workout_feedback_service.go:652-663`).
- `workout_feedback` no tiene `group_id`; el grupo se deriva por `LEFT JOIN` a `group_calendar_days`.

El resultado es que el corredor no tiene forma de ver su evolución y el entrenador no tiene forma de ver al grupo. Los datos están, la lectura no.

## What Changes

- **7 endpoints nuevos bajo `/api/v1/dashboard`**: `filters`, `metrics/summary`, `metrics/timeseries`, `comparisons`, `comparisons/periods`, `leaderboard`, `exercises/{id}/series`.
- **Reglas de KPI por tipificación de ejercicio.** El eje del objetivo lo declara el `kind` (`liviano`, `elongacion`, `ritmo_continuo`, `velocidad_tiempo`, `velocidad_distancia`), no una columna nueva: el modelo lógico es inmutable.
- **Anti-IDOR por inyección de scope**: el scope del usuario se mete en el `WHERE` de la query, no se filtra en Go después de traer las filas.
- **2 índices nuevos**, idempotentes y sin downtime. Ninguna tabla, columna, tipo ni FK se toca.
- **`SEEDING_ENABLED`**: flag de entorno, **solo local**, que relaja el guard de día cerrado para poder sembrar histórico.
- **Scripts de seed y limpieza**, parametrizables por equipo/grupo, transaccionales y con dry-run.

### Objetivo

Que corredor y entrenador puedan responder, desde la app y sin exportar a planillas: *¿cómo vengo?*, *¿mejoré o retrocedí?*, *¿quién necesita atención?* — con números derivados de lo que efectivamente se ejecutó, no de lo que estaba prescrito.

### Alcance

Vista de corredor (sus propias métricas y evolución), vista de entrenador (agregados del equipo, comparación entre corredores, leaderboard), y el dataset sintético para validar todo eso contra datos realistas.

## Capabilities

### New Capabilities
- `dashboard-analytics`: endpoints de analítica de rendimiento, reglas de KPI por tipificación de ejercicio, autorización por scope, calidad de datos y comparación entre períodos.

### Modified Capabilities
- `workout-feedback`: se agrega el índice `idx_feedback_session_date (assigned_session_id, session_date) WHERE deleted_at IS NULL`. Es la única modificación: no cambia ningún requisito de comportamiento de la API actual.
- `group-calendar`: `SEEDING_ENABLED=true` relaja el guard de día cerrado **solo en entorno local**, para permitir el stamping de fechas pasadas que necesita el seed.

## Non-Goals

- **No se agrega ninguna métrica nueva al modelo.** No hay campo de objetivo, ni de ritmo, ni de calorías. El objetivo se *deriva* de `minutes`/`distance_m` según el `kind`.
- **No se toca `intensity`.** Sigue en el modelo (el frontend ya no lo usa) pero este change no lo borra — es otro trabajo.
- **No hay rate limiter.** `FASE 2.1` del pedido original lo pedía, pero el backend no tiene ninguno y sumarlo es dependencia nueva. Se acota la query en su lugar (ver `design.md` D12).
- **No hay exportación en el backend.** CSV/JSON se arman en el front desde la respuesta que ya existe; un endpoint de export duplicaría la lógica de columnas.
- **No se implementa actualización en tiempo real por WebSocket.** El WS actual publica por sesión, no por scope de dashboard. Se resuelve con polling.
- **No se arreglan los índices faltantes del catálogo** (`exercises` no tiene ninguno). Es un problema del catálogo, no del dashboard.
- **No se valida la matriz de campos en la API.** Es validación del cliente, decisión ya tomada en el cambio de tipificación de ejercicios.

## Impact

- **APIs**: 7 endpoints nuevos en `/api/v1/dashboard`. Ninguna ruta existente cambia de comportamiento, salvo el guard de día cerrado **solo** cuando `SEEDING_ENABLED` está activo en local.
- **Datos**: 2 índices nuevos. Cero cambios de schema.
- **Dependencias**: ninguna. El contrato sigue siendo code-first — las anotaciones `swag` de los controllers generan el OpenAPI canónico, como el resto del repo.
- **Frontend**: pantalla nueva de dashboard, que se especifica en su propio change (`dashboard-analitico-rendimiento` en `paceron-frontend`), consumiendo estos endpoints.
- **Configuración**: variable de entorno `SEEDING_ENABLED`, que solo corre con `--stage=local`, y que debe rechazarse en cualquier otro stage (incluido staging).

### Métrica de éxito

- Un corredor con ≥3 sesiones en el período ve ritmo y cumplimiento con valores que un entrenador puede reproducir a mano desde las mismas series.
- El leaderboard muestra **variación real** entre corredores — si el top 5 queda dentro del 2%, la métrica no sirve.
- Un `ritmo_continuo` muestra delta y cumplimiento. Un ejercicio **sin objetivo cargado** muestra solo mejor tiempo, sin delta ni cumplimiento — y eso lo decide `has_target` (el dato), no el `kind`.
- Los objetivos del seed aparecen bajo el nombre de los tipos nuevos, nunca bajo los cinco tipos legacy.
