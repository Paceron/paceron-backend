## REMOVED Requirements

### Requirement: Clonado por divergencia al editar una Session con asignaciones activas

**Motivo de remoción:** reemplazado enteramente por instanciación en el momento de asignar (ver capability `group-calendar` en este mismo change). Ya no existe una referencia viva entre `GroupCalendarDay` y el catálogo que pueda divergir — `PUT /sessions/{id}` y `PUT /exercises/{id}` vuelven a ser ediciones simples de catálogo, sin `exclude_group_ids`/`clone_name`/`clone_description` ni ningún chequeo de calendario.

Todo el contenido de `calendario-asignacion-grupos` (D8/D13) y `congelar-ejercicio-en-clon` (deep-clone de `Exercise`) queda sin efecto — no se re-lista escenario por escenario acá, el `proposal.md`/`design.md` de este change documentan qué reemplaza a cada pieza.

`GET /sessions/{id}/assigned-groups` SHALL eliminarse (sin reemplazo) — nació exclusivamente como el paso 1 del flujo D8 (checklist de grupos a excluir antes de editar una `Session`), y ese flujo desaparece entero. Ver `design.md` D7.
