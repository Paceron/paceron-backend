# Spec Delta — gamification-achievements

## Purpose

Define cómo se atribuyen los puntos GPS al evaluar logros de sesiones finalizadas, para evitar que la actividad GPS de un corredor desbloquee logros para otro en una sesión compartida.

## ADDED Requirements

### Requirement: Los logros GPS atribuyen puntos al atleta del feedback

El sistema DEBE (MUST) contar una instancia para `gps_N` solo si existen puntos GPS asociados a feedback cuyo `athlete_user_id` corresponde al atleta de la runner session finalizada. Los puntos de otro atleta en la misma instancia compartida no contribuyen al logro. La elegibilidad actual de feedback soft-deleted se mantiene.

#### Scenario: Puntos propios en instancia compartida

- **WHEN** un atleta tiene una runner session finalizada y puntos GPS asociados a su propio feedback en esa instancia
- **THEN** esa instancia cuenta una vez para sus logros `gps_N`.

#### Scenario: Solo otro atleta tiene puntos

- **WHEN** dos atletas comparten una instancia de calendario, la runner session de A está finalizada y solo B tiene puntos GPS asociados a su feedback
- **THEN** la instancia no cuenta para los logros `gps_N` de A.

#### Scenario: Feedback propio soft-deleted

- **WHEN** los puntos GPS pertenecen al feedback del mismo atleta pero el feedback fue soft-deleted, y existe una runner session finalizada del atleta en la instancia
- **THEN** se conserva la política vigente y la instancia cuenta para `gps_N`.
