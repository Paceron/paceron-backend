## 1. API y dominio

- [x] 1.1 Definir DTOs de respuesta y query con `period`, fecha, sesiones, estructura y cobranzas.
- [x] 1.2 Crear DAO agregado scoped al owner para sesiones, equipos y corredores, sin N+1.
- [x] 1.3 Agregar agregación financiera neta y cuotas pendientes dentro del rango, reutilizando las reglas existentes.
- [x] 1.4 Crear servicio y controller, validar períodos/fechas y obtener el owner solo del token.
- [x] 1.5 Registrar `GET /api/v1/dashboard/trainer-home-kpis` y Swagger.

## 2. Verificación

- [x] 2.1 Tests de período, agregados, aislamiento y contrato.
- [x] 2.2 Ejecutar tests de paquetes afectados y `go test ./...`.
- [x] 2.3 Ejecutar `go vet ./...`.
