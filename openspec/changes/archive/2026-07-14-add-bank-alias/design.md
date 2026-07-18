## Context

El endpoint `PUT /api/v1/users/:id` permite actualizar campos del perfil de usuario utilizando updates parciales (campos opcionales con punteros `*string`). El sistema actual maneja campos como nombre, email, teléfono, dirección, etc., pero no dispone de un campo para identificar cuentas bancarias mediante un alias personalizado.

El proyecto sigue una arquitectura en capas: Controllers → Services → DAOs, con GORM como ORM y PostgreSQL como base de datos. AutoMigrate gestiona los esquemas de la tabla `users`.

## Goals / Non-Goals

**Goals:**
- Agregar el campo `bank_alias` al modelo de usuario en la DB (columna nullable).
- Implementar la validación del campo: longitud 6-20 caracteres, solo letras, números, puntos (.) y guiones (-).
- Incluir el campo en el DTO de request y response del endpoint de actualización.
- Mantener la consistencia con el patrón existente de updates parciales.

**Non-Goals:**
- Modificar endpoints de consulta de usuario (GET) para incluir `bank_alias`.
- Agregar validación de unicidad del alias bancario (no es un campo único).
- Implementar búsqueda de usuarios por alias bancario.
- Crear migraciones manuales (AutoMigrate se encarga).

## Decisions

### 1. Campo nullable en la DB

**Decisión**: El campo `bank_alias` será `*string` en el struct GORM con tag `gorm:"column:bank_alias"` sin constraint `not null`.

**Razón**: El campo es opcional. Los usuarios existentes no tendrán este valor al momento de la migración, por lo que la columna debe permitir `NULL`.

**Alternativa descartada**: Usar un string vacío como valor por defecto — genera ambigüedad entre "no configurado" y "configurado como vacío".

### 2. Validación en la capa de servicio

**Decisión**: La validación del formato de `bank_alias` se implementa en la función `ValidateUserUpdateRequest()` dentro de `user_service.go`, siguiendo el patrón existente de validación de campos.

**Razón**: Consistencia con la arquitectura actual. Todas las validaciones de formato de campos del request están centralizadas en esta función.

### 3. Expresión regular para validación

**Decisión**: Usar la regex `^[a-zA-Z0-9.\-]{6,20}$` para validar el campo.

**Razón**: Cumple exactamente con los requisitos: letras (a-z, A-Z), números (0-9), puntos (.) y guiones (-), con longitud entre 6 y 20 caracteres. El patrón es simple y eficiente.

### 4. TrimSpace antes de persistir

**Decisión**: Aplicar `strings.TrimSpace(*req.BankAlias)` antes de persistir, igual que se hace con los demás campos string.

**Razón**: Consistencia con el manejo de otros campos. Evita persistir espacios en blanco al inicio o final del valor.

## Risks / Trade-offs

- **[Riesgo]** AutoMigrate podría fallar si hay una columna existente con el mismo nombre pero tipo diferente. → **Mitigación**: Verificar que no existe la columna antes de implementar. Si existe, crear una migración manual.
- **[Trade-off]** No se valida unicidad del alias bancario. → **Justificación**: El requisito no lo solicita. Si se necesita en el futuro, se puede agregar un índice y validación adicional.
- **[Riesgo]** El campo podría contener valores que parecen válidos pero no son útiles (ej: "------"). → **Mitigación**: Se podría agregar una validación de que al menos un carácter sea alfanumérico, pero esto escapa del alcance actual.
