# Proposal

## Why

El alias bancario dejó de ser un dato usado por el producto, pero el endpoint de activación del rol entrenador todavía lo exige cuando el usuario no tiene uno guardado. Eso impide activar el perfil aunque la contraseña y la conexión de Mercado Pago sean válidas.

### Objetivo

Permitir activar el rol entrenador sin `bank_alias`, manteniendo la confirmación de contraseña y la validación de Mercado Pago del flujo existente.

### Alcance

- Hacer opcional el alias en `POST /api/v1/users/{id}/trainer-role`.
- Si se omite, activar el rol sin modificar el alias que ya pudiera estar guardado.
- Si se envía un alias no vacío, conservar validación de formato y persistencia por compatibilidad.
- Actualizar documentación Swagger y pruebas del contrato.

### No alcance

- Eliminar la columna `users.bank_alias` ni migrar datos existentes.
- Cambiar las reglas de contraseña, autorización self-only, cobros o conexión con Mercado Pago.
- Eliminar los campos opcionales de lectura/actualización del endpoint de perfil en el backend.

### Métrica de éxito

Una solicitud de activación con solo la contraseña correcta puede activar el rol cuando se cumplen las demás reglas existentes. Las solicitudes con alias inválido continúan siendo rechazadas y las suites de pruebas pasan.

## What Changes

- Se elimina la condición de que el usuario tenga un alias guardado o envíe uno durante la activación.
- El alias enviado, si es no vacío, sigue validándose y guardándose como hasta ahora.
- La documentación del endpoint deja claro que solo la contraseña es requerida.

## Capabilities

### New Capabilities

- Ninguna.

### Modified Capabilities

- `endpoint-authorization`: la activación de entrenador conserva la confirmación de contraseña, pero deja de requerir un alias bancario.

## Impact

- `cmd/api/services/user_role_service.go` y pruebas co-ubicadas.
- Comentarios Swagger del controller y documentación generada.
- No requiere cambios de base de datos ni altera el endpoint genérico de edición de perfil.
