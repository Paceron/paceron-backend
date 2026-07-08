## ADDED Requirements

### Requirement: Status incluido en RegisterResponse
El sistema SHALL incluir el campo `status` en el response del registro de usuario con valor `"active"`.

```go
type RegisterResponse struct {
    // ... campos existentes
    Status       string `json:"status"`
}
```

#### Scenario: Status en response de registro
- **WHEN** un usuario se registra exitosamente
- **THEN** `RegisterResponse` incluye el campo `status` con valor `"active"`
- **AND** el password nunca aparece en el response

### Requirement: Status por defecto al crear usuario en BD
El sistema SHALL establecer `status = UserStatusActive` al crear un usuario en la BD mediante `authService.Register`.

#### Scenario: Valor por defecto en BD
- **WHEN** se crea un usuario exitosamente
- **THEN** el registro en BD tiene `status = "active"`
- **AND** el modelo `dbs.User` tiene el campo `status` mapeado a columna `status` con default `'active'`

### Requirement: Constantes de status accesibles desde dominio auth
El sistema SHALL importar `domains/constants` desde `services/auth_service.go` para setear el status por defecto y desde `controllers/auth_controller.go` según sea necesario.

#### Scenario: Uso de constantes en auth
- **WHEN** `authService.Register` crea un usuario
- **THEN** usa la constante `constants.UserStatusActive` como valor de status