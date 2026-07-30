## Context

`team_service.GetAll` hoy no acepta parámetros, `teamDao.GetAll` trae todo sin filtro. `group_controller.GetAll` ya tiene precedente de parseo de query params opcionales (`team_id`/`user_id`) en este mismo repo — se sigue el mismo patrón.

## Goals / Non-Goals

**Goals:**
- Filtrar por `owner_id` y/o `member_id` sin romper el uso actual sin filtros.
- Reusar el patrón de parseo de query opcionales ya establecido (`group_controller.GetAll`).

**Non-Goals:**
- Endpoint nuevo bajo `/users/:id/teams` — los query params sobre `/teams` alcanzan.
- Optimizar la combinación de ambos filtros con una query SQL dedicada — es un caso raro, no justifica una tercera query en el DAO.

## Decisions

### 1. Dos métodos DAO separados (`GetAllByOwnerID`, `GetAllByMemberID`) en vez de un único método con filtros opcionales
**Por qué**: cada uno resuelve con una query distinta (`WHERE owner_id = ?` vs. `JOIN team_users`). Un método combinado con parámetros opcionales terminaría construyendo la query condicionalmente igual, sin ahorrar código real, y complica el mock en tests.

### 2. Combinación de ambos filtros (`owner_id` + `member_id`) resuelta en el service, filtrando en memoria
**Por qué**: es un caso de uso raro (¿"equipos que administro donde también participo como corredor"? no tiene mucho sentido de negocio hoy, pero no hay razón para prohibirlo). En vez de agregar una tercera query SQL combinada al DAO para un caso que probablemente nunca se use, se resuelve trayendo por `owner_id` y filtrando en memoria contra `teamUserDao.FindByTeamAndUser` por cada equipo — mismo criterio N+1 aceptado ya en `invitation_service.ListPendingInvitations` (enriquecimiento de invitado), volumen de equipos por owner es bajo.
**Alternativa descartada**: query SQL con doble join — más eficiente, pero sobre-ingeniería para un caso de borde sin caso de uso real conocido hoy.

### 3. Sin filtros, `GetAll` no cambia de comportamiento
**Por qué**: el contrato existente (sin query params) debe seguir devolviendo todos los equipos activos — no es una capability nueva, es una extensión opcional de la existente.

## Risks / Trade-offs

- El filtro combinado (`owner_id`+`member_id`) hace N queries adicionales (una por equipo del owner) — aceptable dado el volumen esperado de equipos por entrenador.
