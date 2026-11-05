## Contexto

`isCalendarDayClosed` (`cmd/api/services/calendar_service.go`) es el único guard de escritura del calendario y lo comparten `UpsertDay`, `DeleteDay`, `Bulk`, `BulkClear`, `Stamp` y `Shift`. Su regla actual:

| Fecha del día | Estado |
|---|---|
| `date < hoy` | cerrado, salvo `SEEDING_ENABLED` |
| `date > hoy` | abierto |
| `date == hoy`, presencial con horario | cerrado desde `presencial_time_from` |
| `date == hoy`, presencial sin horario | abierto |
| `date == hoy`, async | cerrado siempre |

## Decisiones

### D1 — Solo cambia la rama async de hoy

La rama `date == hoy && !is_presencial` pasa a devolver `false`. No se toca ninguna otra rama: pasado, futuro, presencial con/sin horario y la excepción de `cancelled` quedan exactamente igual.

### D2 — El cambio aplica a todas las escrituras

El guard es de nivel día y todos los endpoints lo comparten; partir el comportamiento por operación obligaría a duplicar la regla y generaría combinaciones incoherentes (poder asignar pero no borrar lo asignado). Habilitar el día async de hoy para todas las escrituras es la lectura coherente de "sin un bloqueo duro del sistema".

### D3 — Lectura y banners no cambian

`FindNextForGroupsByKind` y los criterios de "hoy cuenta" en los endpoints de lectura no usan `isCalendarDayClosed`; su regla es independiente (hoy presencial ya iniciado no cuenta; hoy async sí). Se conserva tal cual. Solo se corrigen comentarios que hoy afirman que espejan el guard de escritura.

### D4 — Sin migración ni cambios de contrato

No hay columnas nuevas ni DTOs nuevos. El request y la respuesta de todos los endpoints conservan su forma. `422 ErrCalendarDayClosed` sigue existiendo para los casos que continúan cerrados.

### D5 — Estrategia de tests

- Unitarios del guard: async hoy → `false`; pasado → `true` sin seeding; futuro → `false`; presencial hoy antes/después del horario → `false`/`true`.
- Regresión de servicio con Postgres real: `UpsertDay` de hoy async con `kind=training` crea el día y su instancia; presencial de hoy ya iniciada sigue devolviendo `422`.
- Rediseño de `TestCobertura_StampConflictMezclaCerradoGana`: el caso "422 gana sobre 409" se mantiene usando ayer como día cerrado y hoy como día con conflicto abierto.
- El test de `exclude_dates` conserva su comportamiento; se actualiza el comentario que marcaba hoy como cerrado.

## Verificación

`openspec validate gap-39-async-hoy-asignable --strict`, `gofmt`, `go build ./...`, `go vet ./...`, suite completa con Postgres real (`paceron-test-db:5433`) y coverage con cache limpio + analyzer contra el gate 85.
