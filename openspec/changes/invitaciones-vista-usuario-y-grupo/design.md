## Context

`invitationService` ya depende de `teamDao`/`teamUserDao`/`invitationDao` (feature de invitaciones persistidas, sesión previa). El patrón de "identidad del actor vía `user_id` en query/body, sin middleware de auth" ya está establecido en todo el repo — se sigue igual acá.

## Goals / Non-Goals

**Goals:**
- El invitado puede ver sus invitaciones sin depender de la vista del dueño del equipo.
- El grupo destino de una invitación es explícito y realmente se aplica al aceptar.

**Non-Goals:**
- Autenticación real (JWT) para estos endpoints — mismo patrón `user_id` que el resto del repo.
- Notificar al invitado de nuevas invitaciones vía push/email adicional — el email de invitación ya existe, esto es solo la vista in-app.

## Decisions

### 1. `GET /api/v1/invitations/:id` no exige estado "pending"
**Por qué**: a diferencia de accept/reject (que sí exigen pendiente, porque son acciones), el detalle es de solo lectura — tiene sentido que el invitado pueda ver una invitación que ya aceptó o rechazó (historial), no solo las accionables.
**Validación que sí se mantiene**: `user_id` debe coincidir con `invitee_id` — sigue siendo "mis invitaciones", no un endpoint de admin.

### 2. Asignación de grupo al aceptar: no bloqueante
**Por qué**: la membresía del equipo (`team_users`) es la parte central de aceptar una invitación — ya funciona y está testeada desde la feature anterior. La asignación de grupo es una mejora sobre ese flujo, no lo reemplaza. Si falla (error de DAO) o no hay grupo destino (equipo sin grupo principal y sin `group_id` en la invitación), se loguea y la aceptación de todas formas se completa — el usuario queda en el equipo, sin grupo asignado, recuperable después vía `POST /teams/:id/groups/:group_id/users` (endpoint ya existente).
**Alternativa descartada**: fallar el accept completo si no se puede asignar grupo — se descarta porque dejaría al usuario sin poder aceptar una invitación válida solo porque el equipo nunca configuró un grupo principal, un caso de configuración del equipo, no un error del usuario.

### 3. Grupo principal resuelto vía `GetByTeamID` + filtro `is_main` en el service, no un método de DAO nuevo
**Por qué**: `groupDao.GetByTeamID` ya existe y devuelve los grupos activos del equipo (normalmente pocos) — filtrar `IsMain` en memoria es más simple que agregar un método `FindMainByTeamID` al DAO para un solo caller.
**Alternativa descartada**: nuevo método de DAO — se reevalúa si aparece un segundo caller que necesite lo mismo.

### 4. `group_id` se valida contra el equipo al invitar, no al aceptar
**Por qué**: falla rápido — si el entrenador elige un grupo que no es de su equipo, se entera al mandar la invitación (404), no cuando el invitado intenta aceptar semanas después.

## Risks / Trade-offs

- Si el grupo elegido al invitar se elimina antes de que el invitado acepte, `assignInviteeToGroup` no encuentra el grupo... en realidad `inv.GroupID` sigue apuntando a un ID que puede ya no existir (soft-deleted) — `groupUserDao.Create` igual insertaría la fila con ese `group_id`, apuntando a un grupo borrado. Caso de borde no cubierto explícitamente: no hay validación de que el grupo siga activo al momento de aceptar (solo se validó al invitar). Se acepta el riesgo — mismo nivel de rigor que el resto del repo (ej. `team_id` en muchos modelos tampoco se re-valida en cada operación posterior).
