# Proposal

## Why

Las operaciones de calendario, inicio/cierre de sesión presencial y guardado de feedback hacen lecturas redundantes y viajes de ida y vuelta innecesarios a la base. En endpoints sensibles a latencia, cada consulta extra aumenta el tiempo de respuesta y la carga de la DB. Ahora que el equipo está optimizando lecturas, conviene reducir esas consultas sin alterar el contrato ni el comportamiento visible.

## Objetivo

Reducir consultas redundantes y round trips en las operaciones seleccionadas, manteniendo exactamente los mismos payloads, autorización, transiciones de estado, mensajes de error y orden visible; sumar tests que prueben paridad y, cuando sea posible, menor cantidad de acceso a datos.

## Alcance

- Calendario: `NextSession`, `MemberCalendar`, `NextPresencialSession` y la lectura duplicada de `UpsertDay`.
- Inicio/cierre presencial: resolución de ownership, gate/open/close y creación/finalización de `runner_session`.
- Feedback: creación y actualización, incluyendo chequeos de acceso, derivación de equipo y relecturas posteriores al update.
- Tests de servicios/DAOs para cubrir paridad funcional y reducción de round trips en los caminos modificados.
- Trabajo sobre la rama actual `fix/team-removal-group-memberships`; sin cambios de schema ni índices.

## What Changes

- Las lecturas de calendario resolverán nombres de grupos/equipos solo para el conjunto devuelto y reutilizarán la membresía activa, en lugar de precargar todos los grupos del usuario.
- `UpsertDay` evitará la lectura previa fuera de la transacción, conservando el mismo comportamiento transaccional.
- El gate/open/close presencial usará una lectura conjunta de día+grupo+equipo y `UPDATE ... RETURNING` cuando aplique; la creación/finalización de `runner_session` agrupará los chequeos de acceso del caller y del atleta y evitará relecturas posteriores al update.
- La creación de feedback consolidará chequeos de acceso y derivará ejercicio/equipo en una sola consulta; la actualización usará `UPDATE ... RETURNING`.
- No cambian endpoints, DTOs, JSON, códigos de estado, reglas de autorización, contrato de frontend ni orden visible.

## No alcance

- No se modifican contratos de API, Swagger, DTOs, frontend, reglas de autorización, orden de respuestas, zona horaria, mensajes de error, semántica de soft-delete, schema, índices ni migraciones.
- No se agregan caches nuevos.
- No se limpian retroactivamente membresías viejas de `group_users` del incidente reportado; esa remediación de datos es separada.
- No se incluyen refactors no relacionados ni optimizaciones fuera de las operaciones listadas.

## Capabilities

### New Capabilities

- Ninguna. Es un cambio interno de performance/refactor sin cambios de comportamiento a nivel de specs; `.openspec.yaml` usa `skip_specs: true`.

### Modified Capabilities

- Ninguna.

## Métrica de éxito

- Todos los tests existentes pasan, más tests nuevos que cubren los caminos modificados.
- Tests de cantidad de consultas demuestran menos round trips en las operaciones objetivo, sin cambios en payloads ni transiciones de estado.
- Pasan `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, `openspec validate optimizaciones-calendario-sesion-feedback --strict` y `git diff --check`.
- La cobertura se mantiene en 85% o más; los paquetes modificados no reducen su cobertura.

## Impact

- `cmd/api/services/calendar_service.go`, `presencial_session_service.go`, `runner_session_service.go`, `workout_feedback_service.go`.
- DAOs relacionados: `group_user_dao.go`, `session_instance_dao.go`, `workout_feedback_dao.go`, `runner_session_dao.go` y sus tests.
- Sin impacto en API ni frontend; sin migración de base.
