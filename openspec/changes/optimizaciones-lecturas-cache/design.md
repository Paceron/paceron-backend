## Design

### D1 — Calendario: índice único y join de links
En `sessionInstanceResponsesByIds` se cargan instancias, links y ejercicios. `NewSessionResponse` reconstruye hoy el mapa `id→ExerciseInstance` por cada sesión. Se construye el índice una vez y se pasa a un mapper indexado; el mapper de fila única se conserva. Se agrega un DAO que devuelve links con su ejercicio en una consulta (`JOIN exercise_instances`), preservando `ORDER BY session_instance_id, id` para que el orden de ejercicios no cambie. Quedan 2 consultas de detalle (instancias + links/ejercicios) en vez de 3.

### D2 — NextSession en batch
Se agrupan los dos kinds (`training`, `cancelled`) en una sola consulta con desempate determinista (`date`, `presencial_time_from`, `id`) y se resuelven las instancias de ambos banners con `FindByIDs`. Se preserva el filtro “hoy cuenta” y el shape actual. Si la query agrupada no es viable sin cambiar semántica, se mantiene la consulta por kind y solo se batchéan las instancias/nombres.

### D3 — Índices P1
Se agregan índices seguros, sin backfill: compuestos para `group_users` y `team_users`, `groups.team_id` y `exercise_instances.source_exercise_id`. Se declaran en los tags GORM y se verifican en `pg_indexes` con `daos_indexes_test.go`. No se eliminan índices existentes en esta rama.

### D4 — Push de cierre
`FindByUserIDs` se ejecuta en lotes acotados (500 IDs) y cada lote de tokens usa su propio timeout, con un presupuesto total acotado. El fan-out sigue async y best-effort; no cambia payload ni destinatarios.

### D5 — Equipos e invitaciones
Las invitaciones vencidas se excluyen en SQL (`expires_at > NOW()`), no en memoria. `filterByMember` resuelve la membresía del usuario una vez y filtra el conjunto del owner en memoria. El alta de equipo resuelve roles con `FindByIDs`. Se preservan orden, 404/403 y nombres vacíos cuando una referencia no existe.

### D6 — Caching
No se agrega caché en esta rama. Solo se considera seguro cachear datos inmutables con invalidación explícita; samples de ruta y otras entidades quedan documentados como deuda. No se cachean permisos, dashboard, rachas, calendario, membresías ni datos por usuario.

### D7 — Verificación
Tests de conteo de queries y paridad de respuesta; suite completa con Postgres real; build/vet/gofmt; `openspec validate --strict`; coverage gate 85. No se ejecutan `local-*`, `demo-*` ni `fullseed-*`.
