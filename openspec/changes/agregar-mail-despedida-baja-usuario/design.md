## Context

La infraestructura de mailer (`infrastructure/mailer/`) fue construida en `agregar-envio-mails-smtp` y conectada únicamente al flujo de registro. `agregar-status-usuario-y-modificaciones` dejó explícitamente fuera de alcance "Notificaciones de cambio de estado". Este cambio cierra ese gap para la transición específica a `inactive`, reutilizando toda la infraestructura existente sin tocar el cliente SMTP ni la configuración.

## Goals / Non-Goals

**Goals:**
- Enviar un correo de despedida cuando `ChangeStatus` transiciona un usuario a `inactive`
- Reusar el mismo patrón de `authService.Register`: envío best-effort, nunca bloquea la respuesta
- Evitar reenvíos redundantes cuando se llama al endpoint repetidamente con `status=inactive` sobre un usuario ya inactivo

**Non-Goals:**
- Notificaciones para otras transiciones de estado (`pause`, `blocked`, `suspended`, reactivación a `active`)
- Corregir la falta de autenticación/autorización de `PATCH /api/v1/users/:id/status` (issue conocido, separado)
- Nuevo cliente SMTP o configuración (se reutiliza `config.MySMTP` y `mailer.Client` tal cual existen)

## Decisions

### 1. Guarda contra reenvíos redundantes: comparar estado previo antes de actualizar
- **Por qué**: sin guarda, invocar el endpoint repetidamente con `status=inactive` sobre un usuario ya inactivo reenviaría el correo cada vez. Se captura `previousStatus` del `FindByID` (antes de `UpdateStatus`) y solo se envía si `previousStatus != "inactive" && status == "inactive"`.
- **Alternativa**: no guardar y aceptar reenvíos — descartada, es spam evitable con una comparación trivial ya disponible en memoria.

### 2. Ubicación del hook: dentro de `userService.ChangeStatus`, no en el controller
- **Por qué**: el controller (`user_controller.go`) no tiene lógica de negocio, solo mapea errores a códigos HTTP; toda la lógica de transición de estado ya vive en el service. Mismo criterio usado para el welcome email en `authService.Register`.

### 3. Reordenar `app.go`: mover el bloque Mailer antes de User flow
- **Por qué**: `NewUserService` ahora necesita el `mailerClient` como segundo argumento; el bloque Mailer se construía después del bloque User flow en el código anterior. Se elige mover Mailer hacia arriba (en vez de mover User flow hacia abajo) porque minimiza el diff y no afecta el orden relativo de Auth flow, que ya depende de ambos.

### 4. Mismo cliente/logger de mailer compartido entre Auth y User flow
- **Por qué**: no hay razón para instanciar dos `mailer.Client` o dos loggers; se pasa la misma instancia `mailerClient` a `NewAuthService` y `NewUserService`.

### 4.b Una única instancia del cliente SMTP, construida en `New` (feedback de code review)
- **Por qué**: `Send` construía un `mail.Client` nuevo en cada envío, generando un cliente por correo. Ahora se construye una sola vez en `mailer.New` y se reutiliza. Es seguro compartirlo entre goroutines: `DialAndSendWithContext` abre y cierra su propia conexión por llamada (no la guarda en el struct) y go-mail protege el acceso a su configuración con un `RWMutex` interno.
- **Efecto secundario**: `New` ahora sí puede fallar (antes nunca retornaba error). En `app.go` se declara `mailerClient` como `mailer.MailerInterface` y solo se asigna si la construcción tuvo éxito, para que en caso de error quede una interfaz nil real y no un `*Client` nulo envuelto en interfaz no-nil (que haría pasar los chequeos `mailer != nil` de los services y terminaría en panic).

### 4.c Un único `SendEmail` parametrizado por tipo, en vez de un método por template (feedback de code review)
- **Por qué**: `SendWelcomeEmail` y `SendFarewellEmail` eran el mismo código con distinto template y asunto. Se reemplazan por `SendEmail(ctx, to, emailType, data)` más un registro `emailTemplates` (tipo → asunto + template parseado) en `render.go`. Agregar un tipo de correo nuevo ahora es sumar una entrada al registro, sin tocar `mailer.go` ni la interfaz.
- **Beneficio adicional**: los templates se parsean una sola vez al cargar el package (`template.Must`) en lugar de en cada envío.
- **Validado al mergear `develop`**: mientras esta rama estaba abierta, `develop` sumó dos tipos de correo más (recuperación de contraseña e invitación a equipo) repitiendo el patrón viejo — exactamente la duplicación que este refactor elimina. Al integrar, los cuatro tipos quedaron unificados bajo `SendEmail`.

### 4.d El asunto también es un template (`text/template`)
- **Por qué**: el correo de invitación necesita un asunto parametrizado (`Invitación a equipo {{.TeamName}} - Paceron`), así que el registro guarda el asunto como template además del cuerpo. Se usa `text/template` (no `html/template`) porque escapar entidades en una línea de asunto las mostraría literales en el cliente de correo (ej. `&` se vería como `&amp;`).
- **`EmailData` unificado**: un solo struct con `Name`, `Code` y `TeamName`; cada template usa los campos que necesita y el resto renderiza vacío. Evita un tipo de datos por cada tipo de correo.

### 5. Envío best-effort, nunca bloquea la respuesta (mismo criterio que Decisión 6 de `agregar-envio-mails-smtp`)
- **Por qué**: el correo de despedida es una notificación, no un requisito para que la baja se efectivice. Un fallo de SMTP no debe impedir que el usuario quede desactivado.

### 6. Mailer nil-safe en el service, con tests dedicados esta vez (a diferencia de `agregar-envio-mails-smtp`)
- **Por qué**: `userService.mailer` puede ser `nil`, chequeado antes de usarse, para no romper los 15 call-sites existentes de `NewUserService`. A diferencia de los dos cambios anteriores (que difirieron los tests del flujo conectado), acá sí se agregó un `mockMailer` y cobertura dedicada de `ChangeStatus`, porque la guarda anti-reenvío es lógica nueva real que amerita protección contra regresiones.

## Risks / Trade-offs

- **Endpoint sin autenticación** (conocido, fuera de alcance): el correo se disparará para cualquier caller de `PATCH /api/v1/users/:id/status`, no solo para el propio usuario. No se agrava el riesgo existente, pero tampoco se mitiga.
- **Reordenamiento de `app.go`**: bajo riesgo, cambio mecánico de orden de construcción sin cambiar comportamiento de los demás flujos.
