# Tasks

## 1. Actualizar el comportamiento de activación

- [x] 1.1 Agregar tests de servicio para activar sin alias, aceptar alias vacío como omitido, preservar el alias existente y seguir rechazando un alias no vacío inválido.
- [x] 1.2 Cambiar `ActivateEntrenador` para no exigir alias; validar y persistirlo solo cuando se envíe un valor no vacío.

## 2. Actualizar el contrato documentado

- [x] 2.1 Actualizar comentarios Swagger del endpoint y el DTO para indicar que `bank_alias` es opcional; regenerar la documentación generada.
- [x] 2.2 Actualizar tests de controller para confirmar que una solicitud sin alias puede retornar éxito y que password incorrecta/formato de alias mantienen sus respuestas.

## 3. Verificación

- [x] 3.1 Ejecutar `go test ./cmd/api/services ./cmd/api/controllers` y `go test ./...`.
- [x] 3.2 Ejecutar `go vet ./...` y `openspec validate --strict optional-alias-activation`.
