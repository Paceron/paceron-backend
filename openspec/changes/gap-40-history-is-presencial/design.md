# Diseño — Gap 40

## D1 — Fuente del dato

`HistorySearch` ya hace `LEFT JOIN group_calendar_days gcd ON gcd.session_instance_id = wf.assigned_session_id`. Se agrega al `SELECT`:

```sql
COALESCE(gcd.is_presencial, false) AS is_presencial
```

Así:
- día presencial → `true`;
- día asíncrono → `false`;
- ítem huérfano (sin día) → `false`, sin romper la fila.

`HistoryCount` y los pools de `available_*` no necesitan el campo y no se tocan.

## D2 — DTO y mapper

- `dbs.WorkoutFeedbackHistoryRow` gana `IsPresencial bool` con `gorm:"column:is_presencial"`.
- `WorkoutFeedbackHistoryItem` gana `IsPresencial bool json:"is_presencial"` sin `omitempty` (el contrato del historial ya devuelve todos los campos, nullable cuando corresponde).
- `historyItems` copia `row.IsPresencial` directo, sin lógica adicional.

## D3 — Compatibilidad y verificación

- Cambio aditivo: clientes viejos ignoran el campo.
- Tests DAO contra Postgres: día presencial → `true`; día async → `false`; huérfano → `false`; se mantienen los filtros y el orden existentes.
- Test de service del shape existente incorpora el campo para ambos endpoints.
- Swagger se regenera con el comando canónico porque el DTO cambia.
- Documentación: `docs/CATALOGO_Y_CALENDARIO.md` §8.9 y `docs/FRONTEND_IMPACTO_INSTANCIACION.md` §9.
