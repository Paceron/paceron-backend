## Context

`Team` ya tiene precedente de agregar columnas aditivas sin migración manual (`AutoMigrate` las agrega solas) — mismo patrón que `password_changed_at` en `User` (Feature D, sesión previa). `CreateTeamRequest` ya tiene un campo opcional similar (`CreateDefaultGroup *bool`) que fluye directo por el bind de JSON sin lógica de controller dedicada.

## Goals / Non-Goals

**Goals:**
- Persistir la preferencia sin romper el contrato existente.
- Default `false` si no se envía.

**Non-Goals:**
- Enforcement de la visibilidad en otros endpoints — ver "No alcance" en `proposal.md`. Es una decisión de scope: hoy no hay ningún consumidor real de "ocultar grupo a un corredor" del lado del backend (los endpoints de grupo no filtran por rol del que consulta), agregar ese enforcement ahora sería sobre-ingeniería sin caso de uso.

## Decisions

### 1. Campo bool simple, no un modelo de "configuración de equipo" aparte
**Por qué**: es un único flag hoy. Un modelo de settings separado (tabla `team_settings`) sería sobre-ingeniería para un campo — se agrega directo a `Team`, mismo criterio que otros flags existentes (`Status`).
**Alternativa descartada**: tabla de configuración aparte — se reevalúa si aparecen más preferencias de equipo en el futuro.

### 2. Sin cambios en el controller
**Por qué**: `CreateTeamRequest`/`UpdateTeamRequest` ya se bindean completos desde el body (`c.BindJSON(&req)`), sin allowlist de campos — el campo nuevo llega solo, mismo patrón que `create_default_group`. Agregar lógica de controller sería redundante.

## Risks / Trade-offs

- Ninguno relevante — cambio aditivo, sin impacto en comportamiento existente.
