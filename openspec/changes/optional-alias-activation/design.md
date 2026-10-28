# Design

## Context

`POST /api/v1/users/{id}/trainer-role` comprueba la contraseña y luego exige que exista un alias en el perfil o que llegue uno en la solicitud. El DTO ya representa `bank_alias` como puntero opcional; la obligatoriedad actual está en la lógica del servicio. El alias de perfil es nullable y no se requiere para el resto de las actualizaciones de usuario.

## Goals / Non-Goals

**Goals:**

- Dejar que la activación de entrenador funcione con contraseña correcta aunque no haya alias.
- Conservar validación y guardado si un cliente antiguo todavía envía un alias válido.
- No borrar silenciosamente alias guardados cuando el campo no se envía o se envía vacío.

**Non-Goals:**

- Borrar datos, schema o propiedades de perfil del backend.
- Cambiar la conexión o los requisitos de cobro de Mercado Pago.

## Decisions

### D1: La ausencia del alias no es un error de negocio

Se elimina el guard que devuelve "se requiere un alias bancario". `nil`, `null` y texto vacío se tratan como alias no provisto; el servicio continúa con la asignación del rol. Un alias no vacío sigue sujeto al regex actual y se persiste solo cuando fue proporcionado.

### D2: Mantener compatibilidad de clientes

No se elimina `BankAlias` del DTO. Clientes que todavía lo envían con un valor válido conservan el comportamiento actual; clientes nuevos pueden omitirlo. Un alias existente no se limpia por una activación que no envía el campo.

### D3: El cambio no altera las otras protecciones

Se conserva la verificación de contraseña, el alcance self-only, el manejo de rol ya asignado y el requisito de las reglas de negocio existentes. Solo cambia la dependencia del alias.

## Risks / Trade-offs

- La activación ya no garantiza que el perfil entrenador tenga un alias de pagos. Es intencional porque el producto deja de usar ese dato; la conexión de Mercado Pago permanece sin cambios.
- Los valores inválidos explícitos siguen rechazándose, para no aceptar datos con formato incorrecto de clientes antiguos.

## Migration Plan

No hay migración de datos. El despliegue actualiza el handler de negocio y su documentación; el campo nullable existente puede permanecer en la base y en los DTOs para compatibilidad.
