## 1. Configuración de entorno

- [ ] 1.1 Agregar soporte `DATABASE_URL` en `config/config.go` (parsing de connection string)
- [ ] 1.2 Crear `.env.example` con variables documentadas
- [ ] 1.3 Agregar dependencia `golang.org/x/crypto` para bcrypt

## 2. Modelo de datos

- [ ] 2.1 Actualizar `domains/dbs/user.go` con todos los campos (surname, email, phone, phone_contact, country, city, street, number, dni, birth_date)

## 3. Cliente DB

- [ ] 3.1 Mejorar `infrastructure/postgresdb/postgres.go` con auto-migrate y pool configurable

## 4. DTOs de dominio

- [ ] 4.1 Crear `domains/auth/register_request.go` con DTO de entrada
- [ ] 4.2 Crear `domains/auth/register_response.go` con DTO de salida

## 5. Capa de datos (DAO)

- [ ] 5.1 Crear `daos/auth_dao.go` con interfaz + implementación (FindByEmail, FindByDNI, Create)

## 6. Capa de negocio (Service)

- [ ] 6.1 Crear `services/auth_service.go` con interfaz + implementación (validación, bcrypt, transformers)

## 7. Capa de presentación (Controller)

- [ ] 7.1 Crear `controllers/auth_controller.go` con handler POST /api/v1/auth/register
- [ ] 7.2 Agregar ruta en `app/url_mappings.go`
- [ ] 7.3 Agregar wiring en `app/app.go`

## 8. Tests

- [ ] 8.1 Escribir tests para auth_service
- [ ] 8.2 Escribir tests para auth_controller
- [ ] 8.3 Escribir tests para auth_dao
