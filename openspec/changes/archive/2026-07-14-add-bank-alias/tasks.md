## 1. Modelo de datos (GORM)

- [x] 1.1 Agregar campo `BankAlias *string` al struct `User` en `domains/dbs/user.go` con tag `gorm:"column:bank_alias"`

## 2. DTOs de dominio user

- [x] 2.1 Agregar campo `BankAlias *string` al struct `UserUpdateRequest` en `domains/user/update_request.go` con tag `json:"bank_alias,omitempty"`
- [x] 2.2 Agregar campo `BankAlias string` al struct `UserUpdateResponse` en `domains/user/update_response.go` con tag `json:"bank_alias,omitempty"`

## 3. Capa de negocio (Service)

- [x] 3.1 Agregar validación de `bank_alias` en `ValidateUserUpdateRequest` en `services/user_service.go`: longitud entre 6 y 20 caracteres, regex `^[a-zA-Z0-9.\-]{6,20}$`
- [x] 3.2 Agregar mapeo de `bank_alias` en el método `Update` de `services/user_service.go`: si `req.BankAlias != nil`, aplicar `strings.TrimSpace` y asignar a `userDB.BankAlias`
- [x] 3.3 Agregar campo `BankAlias` en la función `toUserUpdateResponse` en `services/user_service.go`

## 4. Tests

- [x] 4.1 Actualizar tests de `ValidateUserUpdateRequest` en `services/user_service_test.go` para cubrir validación de `bank_alias` (longitud mínima, longitud máxima, caracteres no permitidos, valor válido, campo nulo)
- [x] 4.2 Actualizar tests del método `Update` en `services/user_service_test.go` para verificar persistencia de `bank_alias`
