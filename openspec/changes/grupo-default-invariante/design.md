## Context

`assignInviteeToGroup` (feature de invitaciones, sesión previa) ya resuelve "grupo principal del equipo, no bloqueante" para el flujo de aceptar invitación. Este cambio aplica el mismo criterio a dos puntos más: creación de equipo (`TeamDelegate.CreateTeam`) y alta directa (`TeamUserService.AddUser`).

## Goals / Non-Goals

**Goals:**
- Todo equipo nuevo tiene grupo principal salvo exclusión explícita.
- Alta directa de usuario a equipo también entra a un grupo, igual que por invitación.

**Non-Goals:**
- Backfill de equipos existentes sin grupo — decisión explícita del usuario, queda como deuda.
- Unificar `assignInviteeToGroup` (invitation_service.go) y `assignToMainGroup` (team_user_service.go) en un helper compartido — son servicios distintos, y el repo no permite que un service importe a otro. Se acepta la duplicación menor (~20 líneas) en vez de forzar un helper cruzado.

## Decisions

### 1. `create_default_group` cambia de opt-in a opt-out
**Por qué**: decisión del usuario (2026-07-31) — "la única forma de que no se cree el grupo default sería mandando create_default_group=false, por default debe crearlo". Refleja el invariante real del dominio.
**Compatibilidad**: el frontend que ya manda `true` sigue igual. Rompe (en el buen sentido) a cualquier flujo que omitía el campo esperando "no crear grupo" — no había ningún flujo así documentado, se asume que era simplemente un caso no contemplado antes.

### 2. `AddUser` no recibe `group_id` explícito (a diferencia de `InviteRunnerRequest`)
**Por qué**: no fue pedido — el alta directa (`POST /teams/:id/users`) es un endpoint más simple/administrativo, siempre va al grupo principal. Si se necesita elegir grupo al agregar directamente, es una extensión futura sobre `AddTeamUserRequest`, no parte de este cambio.

### 3. Sin backfill de equipos existentes
**Por qué**: decisión explícita del usuario. Los equipos ya creados sin grupo (ej. el equipo de prueba creado en esta misma sesión sin `create_default_group`) quedan como están hasta que alguien cree un grupo manualmente para ellos, o se decida un backfill más adelante.

## Risks / Trade-offs

- Duplicación entre `assignInviteeToGroup` e `assignToMainGroup` — aceptado, ver Non-Goals.
- Equipos preexistentes sin grupo principal seguirán fallando silenciosamente la asignación de grupo en `AddUser`/`AcceptInvitation` (mismo comportamiento no bloqueante ya diseñado) hasta que se les cree un grupo.
