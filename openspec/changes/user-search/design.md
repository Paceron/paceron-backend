## Context

Gap 1 de `paceron-frontend/docs/BACKEND_API_GAPS.md`, desbloqueado por `feature/protect-all-endpoints` (ya no hace falta inventar autenticación ad-hoc para este endpoint).

## Goals / Non-Goals

**Goals:**
- Autocompletar usable al invitar: menos de un puñado de resultados, respuesta rápida, sin exponer más que lo necesario.

**Non-Goals:**
- Lookup en lote por ids (gap 2 del mismo doc) — endpoint distinto, no bloqueante, queda para otra iniciativa.
- Ranking/relevancia sofisticada (fuzzy match, scoring) — `ILIKE` simple alcanza para el volumen actual.
- Paginación — con el límite fijo de 5 resultados no hace falta.

## Decisions

### 1. Sin restricción de rol, solo login
**Por qué**: a diferencia de los patrones de la rama anterior (self-only, solo-entrenador, self-o-delegado), acá no hay todavía una relación entre quien busca y a quién encuentra — es exactamente el paso previo a que esa relación exista (la invitación). Restringir por rol no tiene sentido de negocio; alcanza con exigir sesión válida para que no sea anónimo.

### 2. Datos discretos, no el usuario completo
**Por qué**: `auth.UserResponse` (usado en `GET /auth/user`) expone DNI, teléfono, dirección — apropiado para el dueño de esos datos, no para que cualquier logueado lo vea de un tercero al tipear un autocompletar. `SearchResultItem` es un DTO nuevo, deliberadamente acotado a `user_id`/`name`/`surname`/`email` (lo que ya usa `InviteRunnerRequest.Email` para invitar). Si más adelante hace falta más dato para el flujo, se agrega explícitamente al DTO — no se reusa el DTO completo "por las dudas".

### 3. Mínimo 3 caracteres, límite 5 resultados
**Por qué**: un `ILIKE '%q%'` sin índice de texto completo escanea la tabla — con 1-2 caracteres el resultado es ruidoso y la consulta más cara sin necesidad (típico de un autocompletar). Ambos valores son constantes de paquete en `user_service.go`, fáciles de ajustar si hace falta.

### 4. Filtra por `status = active`
**Por qué**: no tiene sentido sugerir para invitar a un usuario inactivo/bloqueado/suspendido — la invitación fallaría más adelante en el flujo de todos modos (mismo criterio que usa el login).

## Risks / Trade-offs

- **`ILIKE` sin índice**: aceptable al volumen actual de usuarios del proyecto (tesis, no escala masiva). Si se vuelve un problema real, la solución es un índice `pg_trgm`/`gin`, no un rediseño del endpoint.
- **Sin rate limiting propio**: mismo criterio que el resto de la API hoy — no hay rate limiting en ningún endpoint, no se introduce acá como caso especial.
