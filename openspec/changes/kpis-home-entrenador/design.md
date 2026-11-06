# Diseño: KPIs del Home del entrenador

## Contrato

`GET /api/v1/dashboard/trainer-home-kpis?period=month|week|day&reference_date=YYYY-MM-DD`

El backend toma `trainer_user_id` exclusivamente del token. Respuesta estable:

```json
{
  "period": "month",
  "reference_date": "2026-11-05",
  "from": "2026-11-01",
  "to": "2026-11-30",
  "days_considered": 30,
  "sessions": {
    "scheduled": 18,
    "realized": 12,
    "presencial_scheduled": 8,
    "presencial_realized": 6
  },
  "structure": { "active_teams": 2, "active_runners": 17 },
  "collections": {
    "currency_id": "ARS",
    "net_collected": 120000,
    "pending_amount": 30000,
    "pending_count": 3
  }
}
```

Los valores de período son agregados por backend. `scheduled` cuenta filas
distintas de calendario `kind=training`, con `session_instance_id`, en grupos de
equipos activos del entrenador; canceladas, descanso y días sin instancia no
cuentan. `realized` cuenta esas sesiones con al menos una fila de asistencia;
las variantes presenciales aplican además `is_presencial=true`.

`active_teams` requiere equipo no eliminado y `status=active`. `active_runners`
es `COUNT(DISTINCT team_users.user_id)` con rol corredor, membresía activa y
equipo activo, evitando duplicar corredores de varios equipos.

`net_collected` suma solo cobros aprobados de membresías de equipo del vendedor,
con `payment_id` no vacío, creados en el período; neto = bruto − comisión de
Paceron, con la fórmula y default actuales de `PaymentHistoryDao`. Los límites
de timestamp usan `America/Argentina/Cordoba` y el rango `[from, to+1 día)`.
`pending_amount/count` suma obligaciones estimadas cuyo vencimiento está dentro
del mismo período, sin incluir cuotas ya pagadas.

## Decisiones

1. Servicio dedicado, DAO agregado y controller dedicado bajo el dominio
   dashboard; no hacer N+1 ni pedir listados completos de calendarios/pagos para
   agregarlos en memoria.
2. Reusar `dashboard.ResolveRunnerPeriod` para que los cortes y títulos sean
   idénticos a los KPIs del corredor.
3. Reusar las expresiones financieras canónicas del historial (pago aprobado,
   excluir preferencias sin `payment_id`, neto después de comisión) y los
   helpers puros de obligaciones de `services`.
4. El scope no acepta `owner_id`, `team_id`, `group_id` ni otros IDs del caller.
5. Todo contador/monto se devuelve aunque sea cero; no hay `null` ambiguo para
   estas métricas. Moneda `ARS`, igual que las cuotas de membresía.

## Errores

- `401` sin identidad autenticada.
- `400 INVALID_PERIOD` o `INVALID_DATE` para parámetros inválidos, siguiendo
  los códigos ya existentes del Home del corredor.
- `500` para fallo de lectura; nunca devolver agregados parciales.

## Tests

- período válido/default y errores de período/fecha;
- aislamiento por entrenador y exclusión de otros owners/equipos eliminados o
  inactivos;
- programadas vs realizadas (asistencia ausente/presente), presencial como
  subconjunto y canceladas excluidas;
- corredores únicos compartidos por más de un equipo y exclusión de roles no
  corredor/membresías inactivas;
- cobros aprobados, pagos sin `payment_id`, comisión/neto y obligaciones dentro
  del período;
- contrato con todas las claves y ceros; controller auth/status; `go test ./...`
  y `go vet ./...`.
