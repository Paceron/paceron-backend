# Gap 42 — tiers del rol corredor

## Por qué

El modelo actual exige un tier para todo `user_role`: `AssignRole` resuelve el tier base con `FindLowestByRole` y `GET /auth/permissions` falla si un rol no tiene tier base. A la vez, el producto decidió que el corredor no tenga tiers ni suscripciones de tier: su monetización ocurre por equipos. Purgar literalmente todos los tiers del corredor rompería la asignación del rol y la consulta de permisos.

## Qué

- Conservar un tier base interno para `corredor` como soporte del modelo de roles/permisos.
- Ocultar todos los tiers del corredor del catálogo público: `GET /tiers?role_id=<corredor>` devuelve `[]` y el listado sin filtro no los incluye.
- Impedir crear, actualizar o eliminar tiers del corredor por API; el tier base interno queda protegido.
- Rechazar la asignación de un tier explícito y el cambio de tier para el rol corredor. La asignación del rol sin `tier_id` sigue usando el base interno.
- Incluir un script SQL idempotente y manual para purgar tiers pagos y suscripciones del corredor en bases de desarrollo/testing, preservando el tier base y reasignando `user_roles.tier_id` al base antes de borrar.
- No cambiar contratos HTTP existentes ni agregar migraciones automáticas.

## Alcance

Solo backend. El frontend quita el badge y la acción “Mejorar tier” del corredor en un trabajo aparte.

## No-goals

- No se elimina el concepto de tier del modelo ni se hace nullable `user_roles.tier_id`.
- No se toca la monetización de equipos.
- No se ejecuta el script de purga contra bases locales/demo desde este change.
- No se agregan tiers pagos nuevos para corredor.

## Capabilities

- `tier-management` (nueva): catálogo y asignación de tiers por rol.
