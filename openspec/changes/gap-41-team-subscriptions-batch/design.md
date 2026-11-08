## Contexto

El estado de cuenta actual (`GetTeamSubscription`) resuelve un único corredor y exige que sea el usuario autenticado. El roster del equipo ya lista a los miembros, pero no trae el estado de cuota. La solución debe ser una sola consulta por equipo, sin N+1.

## Decisiones

### D1 — Autorización

El endpoint `GET /api/v1/teams/{id}/subscriptions` exige:

1. Equipo existente; si no, `404` con `equipo no encontrado`.
2. Caller igual a `team.owner_id`, o fila `team_users` activa con `role_in_team = 'entrenador'`.
3. Cualquier otro caso: `403 Forbidden` (`no autorizado`).

El owner no necesita fila `team_users` para autorizar. Un corredor del equipo, aunque sea miembro, recibe `403`.

### D2 — Datos y consulta batch

- `TeamUserDao.FindByTeamID(teamID)` ya devuelve las asociaciones activas (`deleted_at IS NULL`). Se filtran en memoria las filas con `role_in_team = 'corredor'`.
- Nueva consulta `InstallmentDao.FindPendingByTeam(ctx, teamID)`:
  `WHERE team_id = ? AND status = 'pending' ORDER BY user_id, installment_number ASC`.
  Devuelve todas las cuotas pendientes del equipo en una sola query. El service las agrupa por `user_id`.
- Para cada corredor, sin consultas adicionales:
  - Equipo gratis (`team.membership_fee == 0 && member.init_amount <= 0`) → `has_debt=false`, estado `active`.
  - Equipo pago: `has_debt = HasPendingDebt(pendingForUser) || membershipOverdue(member, now) > 0`.
  - `subscription_status` crudo: `member.subscription_status`; vacío legacy → `active`.
  - Estado de presentación `status`:
    - `has_debt` → `overdue`
    - si no, `subscription_status == first_payment_pending` → `first_payment_pending`
    - si no, `active`.
- Se reutiliza `HasPendingDebt` y `membershipOverdue` para mantener el mismo criterio que el estado de cuenta individual. No se usa `membershipDebt` (incluye cuotas que vencen hoy) porque el badge debe reflejar deuda vencida, igual que `has_debt` actual.

### D3 — Contrato

```json
{
  "team_id": 7,
  "membership_fee": 15000,
  "members": [
    {
      "user_id": 12,
      "status": "overdue",
      "subscription_status": "active",
      "has_debt": true,
      "init_amount": 15000,
      "paid_installments": 2
    }
  ]
}
```

- `members` siempre es un array, vacío si no hay corredores.
- Solo corredores; el owner y los entrenadores quedan fuera.
- Los valores de `status` son estables y documentados para que el frontend los mapee a sus badges (`active`→activo, `overdue`→vencido, `first_payment_pending`→en prueba).
- No se incluye `next_installment`; el frontend solo necesita el badge en la fila.

### D4 — Errores

- `400` team id inválido.
- `401` sin autenticación.
- `403` caller sin rol de entrenador/owner.
- `404` equipo inexistente.
- `500` error interno, sin filtrar detalles del DAO.

### D5 — Tests y verificación

- DAO contra Postgres real: cuotas pendientes de dos corredores, orden por usuario/número y aislamiento por equipo.
- Service con mocks: owner autorizado, entrenador autorizado, corredor 403, equipo inexistente, equipo gratis, primer pago pendiente, deuda vencida y agrupación batch.
- Controller: 200, 401, 400, 403, 404.
- `go build`, `go vet`, gofmt, suite completa con `paceron-test-db:5433`, coverage gate 85, `openspec validate --strict` y Swagger regenerado.
