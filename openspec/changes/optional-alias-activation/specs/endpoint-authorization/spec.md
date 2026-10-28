# Spec Delta

## Purpose

Define las reglas de autorización y validación para los endpoints de roles del usuario.

## ADDED Requirements

### Requirement: La activación del rol entrenador SHALL permitir omitir el alias

Este requirement reemplaza el requisito anterior de alias obligatorio de `protect-all-endpoints`. El sistema SHALL exigir la contraseña actual correcta para activar el rol entrenador, pero no SHALL requerir `bank_alias`. Si se omite, es `null` o vacío, SHALL preservar el valor almacenado y continuar. Si se envía no vacío, SHALL validar su formato y persistirlo. Las demás protecciones SHALL permanecer sin cambios.

#### Scenario: Activación sin alias
- **WHEN** el usuario autenticado envía la contraseña actual correcta sin `bank_alias`
- **THEN** el sistema activa el rol entrenador y retorna HTTP 201 sin modificar el alias almacenado

#### Scenario: Activación con alias vacío
- **WHEN** el usuario autenticado envía la contraseña correcta y `bank_alias` ausente, `null` o vacío
- **THEN** el sistema activa el rol entrenador sin exigir ni borrar un alias existente

#### Scenario: Activación con alias válido
- **WHEN** el usuario autenticado envía la contraseña correcta y un alias no vacío con formato válido
- **THEN** el sistema activa el rol y persiste el alias enviado

#### Scenario: Activación con alias inválido
- **WHEN** el usuario autenticado envía un alias no vacío con formato inválido
- **THEN** el sistema retorna HTTP 400 y no activa el rol

#### Scenario: Contraseña incorrecta
- **WHEN** la contraseña enviada no coincide con la del usuario autenticado
- **THEN** el sistema retorna HTTP 401 sin activar el rol
