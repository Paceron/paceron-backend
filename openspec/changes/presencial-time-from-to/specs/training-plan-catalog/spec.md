## ADDED Requirements

### Requirement: Día presencial de un plan exige horario de inicio y fin

El sistema SHALL exigir `default_time_from` y `default_time_to` (no un único horario) cuando `default_presencial = true`, y SHALL validar que `default_time_to` sea estrictamente posterior a `default_time_from`.

#### Scenario: Presencial sin ambos horarios
- **WHEN** se crea o edita un día de plan con `default_presencial=true` y falta `default_time_from` o `default_time_to`
- **THEN** el sistema responde `422`

#### Scenario: time_to anterior o igual a time_from
- **WHEN** se crea o edita un día de plan presencial con `default_time_to <= default_time_from`
- **THEN** el sistema responde `422`
