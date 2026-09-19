## ADDED Requirements

### Requirement: Sesión presencial exige horario de inicio y fin

El sistema SHALL exigir `presencial_time_from` y `presencial_time_to` (no un único horario) cuando `is_presencial = true`, y SHALL validar que `presencial_time_to` sea estrictamente posterior a `presencial_time_from`.

#### Scenario: Presencial sin ambos horarios
- **WHEN** se guarda un día con `is_presencial=true` y falta `presencial_time_from` o `presencial_time_to`
- **THEN** el sistema responde `422`

#### Scenario: time_to anterior o igual a time_from
- **WHEN** se guarda un día presencial con `presencial_time_to <= presencial_time_from`
- **THEN** el sistema responde `422`

#### Scenario: Lectura de horario presencial no se corrompe por timezone
- **WHEN** se guarda un día presencial con `presencial_time_from="10:15"` y se lo vuelve a leer (`GET`, o el chequeo de cierre de D13)
- **THEN** el sistema devuelve/usa exactamente `"10:15"`, sin desplazamiento por la zona horaria del proceso
