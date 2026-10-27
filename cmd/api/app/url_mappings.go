package app

import (
	"github.com/gin-gonic/gin"
)

func mapUrls(r *gin.Engine, app *Application) {
	// S9: tope global de body (10 MB) antes de cualquier otro middleware/route.
	r.Use(MaxBodySize())
	r.Use(CORSMiddleware())
	r.Use(SetRequestID())

	// Rutas públicas: no requieren Authorization. register/login/forgot/reset-password
	// son de por sí previas a tener sesión; refresh/logout usan el refresh token como
	// credencial propia (no un access token); GET /auth/user es lookup público a
	// propósito; las de weather quedan públicas sin discusión (demo, ver openspec
	// de protect-all-endpoints). Las rutas legacy /user y POST /user (duplicadas de
	// /api/v1/auth/user y /api/v1/auth/register) se eliminaron, ver openspec de
	// remove-legacy-user-routes.
	r.GET("/ping", app.pingController.Ping)
	r.GET("/callbackauth", app.pingController.CallbackTkn)

	// S10: rate limiter por IP SOLO en las rutas públicas de usuario (son las
	// ataquesables por fuerza bruta). Excluidos explícitamente: webhooks de MP
	// (MP reentiende y no debe recibir 429), ws handshake y lookup de weather.
	r.POST("/api/v1/auth/register", IPRateLimiter(), app.authController.Register)
	r.POST("/api/v1/auth/login", IPRateLimiter(), app.authController.Login)
	r.POST("/api/v1/auth/refresh", IPRateLimiter(), app.authController.Refresh)
	r.POST("/api/v1/auth/logout", app.authController.Logout)
	r.GET("/api/v1/auth/user", IPRateLimiter(), app.authController.GetUser)
	r.POST("/api/v1/auth/forgot-password", IPRateLimiter(), app.passwordResetController.ForgotPassword)
	r.POST("/api/v1/auth/reset-password", IPRateLimiter(), app.passwordResetController.ResetPassword)
	r.GET("/example/weather", app.exampleWeatherController.GetWeather)
	r.GET("/user/:user_id/weather", app.userWeatherController.GetUserWithWeather)

	// Payments - Webhook must be public (Mercado Pago sends notifications without auth)
	r.POST("/api/v1/payments/webhook", app.paymentController.HandleWebhook)

	// MP Connect deauthorization webhook - public (Mercado Pago sends it without auth)
	r.POST("/api/v1/mercadopago/webhook/connect", app.mpConnectController.HandleDeauthWebhook)

	// MP Connect OAuth callback - public a propósito: MP redirige el navegador a
	// esta ruta con code+state SIN header Authorization. La identidad se resuelve
	// del state (CSRF). GET /mercadopago/connect y /connect/status quedan detrás
	// del AuthMiddleware porque sí requieren sesión.
	r.GET("/api/v1/mercadopago/connect/callback", app.mpConnectController.HandleCallback)

	// Gateway WebSocket (ws-gateway-sesiones): pública a propósito, el handler
	// valida su propio token en el query param (no hay header Authorization en
	// el handshake) — D3. No entra al Swagger (no es OpenAPI-representable).
	r.GET("/api/v1/ws", wsUpgrade(app.realtimeGateway))

	mapSwagger(r)
	mapGuide(r)

	// A partir de acá, todas las rutas requieren Authorization: Bearer <access_token>.
	r.Use(AuthMiddleware())

	r.GET("/api/v1/auth/permissions", app.permissionsQueryController.GetUserPermissions)
	r.GET("/api/v1/users", app.userController.BatchLookup)
	r.GET("/api/v1/users/search", app.userController.Search)
	r.PUT("/api/v1/users/:id", app.userController.Update)
	r.PATCH("/api/v1/users/:id/status", app.userController.ChangeStatus)
	r.PATCH("/api/v1/users/:id/password", app.userController.ChangePassword)
	r.PUT("/api/v1/users/:id/photo", app.userController.UploadPhoto)
	r.DELETE("/api/v1/users/:id/photo", app.userController.DeletePhoto)
	r.POST("/api/v1/users/:id/roles", app.userRoleController.AssignRole)
	r.DELETE("/api/v1/users/:id/roles/:role_id", app.userRoleController.RemoveRole)
	r.POST("/api/v1/users/:id/trainer-role", app.userRoleController.ActivateEntrenador)
	r.DELETE("/api/v1/users/:id/trainer-role", app.userRoleController.DeactivateEntrenador)

	// Tier subscriptions
	r.PUT("/api/v1/users/:id/roles/:role_id/tier", app.tierSubscriptionController.ChangeTier)
	r.DELETE("/api/v1/users/:id/roles/:role_id/subscriptions/pending", app.tierSubscriptionController.CancelPendingSubscription)
	r.GET("/api/v1/users/:id/subscriptions/:period", app.tierSubscriptionController.GetCurrentSubscription)
	r.POST("/api/v1/push-tokens", app.pushTokenController.RegisterToken)
	r.GET("/api/v1/permissions", app.permissionController.GetAll)
	r.GET("/api/v1/permissions/by-name", app.permissionController.GetByName)
	r.GET("/api/v1/permissions/:id", app.permissionController.GetByID)
	r.POST("/api/v1/permissions", app.permissionController.Create)
	r.PUT("/api/v1/permissions/:id", app.permissionController.Update)
	r.DELETE("/api/v1/permissions/:id", app.permissionController.Delete)
	r.GET("/api/v1/tiers", app.tierController.GetAll)
	r.GET("/api/v1/tiers/by-name", app.tierController.GetByName)
	r.GET("/api/v1/tiers/:id", app.tierController.GetByID)
	r.POST("/api/v1/tiers", app.tierController.Create)
	r.PUT("/api/v1/tiers/:id", app.tierController.Update)
	r.DELETE("/api/v1/tiers/:id", app.tierController.Delete)
	r.POST("/api/v1/tiers/:id/permissions", app.tierPermissionController.Assign)
	r.GET("/api/v1/tiers/:id/permissions", app.tierPermissionController.List)
	r.DELETE("/api/v1/tiers/:id/permissions/:permission_id", app.tierPermissionController.Unassign)
	r.GET("/api/v1/roles", app.roleController.GetAll)
	r.GET("/api/v1/roles/by-name", app.roleController.GetByName)
	r.GET("/api/v1/roles/:id", app.roleController.GetByID)
	r.POST("/api/v1/roles", app.roleController.Create)
	r.PUT("/api/v1/roles/:id", app.roleController.Update)
	r.DELETE("/api/v1/roles/:id", app.roleController.Delete)

	// Teams
	r.POST("/api/v1/teams", app.teamController.Create)
	r.GET("/api/v1/teams", app.teamController.GetAll)
	r.GET("/api/v1/teams/search", app.teamController.Search)
	r.GET("/api/v1/teams/:id", app.teamController.GetByID)
	r.PUT("/api/v1/teams/:id", app.teamController.Update)
	r.DELETE("/api/v1/teams/:id", app.teamController.Delete)
	r.PUT("/api/v1/teams/:id/address", app.teamController.UpdateAddress)
	r.PUT("/api/v1/teams/:id/icon", app.teamController.UploadIcon)
	r.DELETE("/api/v1/teams/:id/icon", app.teamController.DeleteIcon)

	// Team Users
	r.POST("/api/v1/teams/:id/users", app.teamUserController.AddUser)
	r.GET("/api/v1/teams/:id/users", app.teamUserController.GetUsersByTeam)
	r.DELETE("/api/v1/teams/:id/users/:user_id", app.teamUserController.RemoveUser)

	// Groups
	r.POST("/api/v1/groups", app.groupController.Create)
	r.GET("/api/v1/groups", app.groupController.GetAll)
	r.GET("/api/v1/groups/:id", app.groupController.GetByID)
	r.PUT("/api/v1/groups/:id", app.groupController.Update)
	r.DELETE("/api/v1/groups/:id", app.groupController.Delete)

	// Group Users
	r.POST("/api/v1/teams/:id/groups/:group_id/users", app.groupUserController.AddUser)
	r.GET("/api/v1/groups/:id/users", app.groupUserController.GetUsersByGroup)
	r.DELETE("/api/v1/groups/:id/users/:user_id", app.groupUserController.RemoveUser)

	// Invitations
	r.POST("/api/v1/teams/:id/invite", app.invitationController.InviteRunner)
	r.GET("/api/v1/teams/:id/invitations", app.invitationController.ListPendingInvitations)
	r.GET("/api/v1/invitations", app.invitationController.ListMyInvitations)
	r.GET("/api/v1/invitations/:id", app.invitationController.GetInvitationByID)
	r.POST("/api/v1/invitations/:id/accept", app.invitationController.AcceptInvitation)
	r.POST("/api/v1/invitations/:id/reject", app.invitationController.RejectInvitation)

	// Join Requests
	r.GET("/api/v1/teams/:id/join-requests", app.joinRequestController.ListByTeam)
	r.POST("/api/v1/teams/:id/join-requests", app.joinRequestController.Create)
	r.GET("/api/v1/join-requests/mine", app.joinRequestController.ListMine)
	r.GET("/api/v1/join-requests/pending-count", app.joinRequestController.PendingCount)
	r.DELETE("/api/v1/join-requests/:id", app.joinRequestController.Cancel)
	r.POST("/api/v1/join-requests/:id/accept", app.joinRequestController.Accept)
	r.POST("/api/v1/join-requests/:id/reject", app.joinRequestController.Reject)

	// Exercise catalog
	r.POST("/api/v1/exercises", app.exerciseController.Create)
	r.GET("/api/v1/exercises", app.exerciseController.List)
	r.GET("/api/v1/exercises/:id", app.exerciseController.Get)
	r.PUT("/api/v1/exercises/:id", app.exerciseController.Update)
	r.DELETE("/api/v1/exercises/:id", app.exerciseController.Delete)
	r.POST("/api/v1/exercises/:id/clone", app.exerciseController.Clone)

	// Session catalog
	r.POST("/api/v1/sessions", app.sessionController.Create)
	r.GET("/api/v1/sessions", app.sessionController.List)
	r.GET("/api/v1/sessions/:id", app.sessionController.Get)
	r.PUT("/api/v1/sessions/:id", app.sessionController.Update)
	r.DELETE("/api/v1/sessions/:id", app.sessionController.Delete)
	r.POST("/api/v1/sessions/:id/clone", app.sessionController.Clone)

	// TrainingPlan catalog
	r.POST("/api/v1/training-plans", app.trainingPlanController.Create)
	r.GET("/api/v1/training-plans", app.trainingPlanController.List)
	r.GET("/api/v1/training-plans/:id", app.trainingPlanController.Get)
	r.PUT("/api/v1/training-plans/:id", app.trainingPlanController.Update)
	r.DELETE("/api/v1/training-plans/:id", app.trainingPlanController.Delete)
	r.POST("/api/v1/training-plans/:id/clone", app.trainingPlanController.Clone)

	// Group calendar
	r.GET("/api/v1/groups/:id/calendar", app.calendarController.GetRange)
	r.PUT("/api/v1/groups/:id/calendar/:date", app.calendarController.PutDay)
	r.DELETE("/api/v1/groups/:id/calendar/:date", app.calendarController.DeleteDay)
	r.POST("/api/v1/groups/:id/calendar/stamp", app.calendarController.Stamp)
	r.POST("/api/v1/groups/:id/calendar/bulk", app.calendarController.Bulk)
	r.POST("/api/v1/groups/:id/calendar/bulk-clear", app.calendarController.BulkClear)
	r.POST("/api/v1/groups/:id/calendar/shift", app.calendarController.Shift)

	// Runner calendar views
	r.GET("/api/v1/users/:id/next-session", app.calendarController.NextSession)
	r.GET("/api/v1/users/:id/next-presencial-session", app.calendarController.NextPresencialSession)
	r.GET("/api/v1/users/:id/member-calendar", app.calendarController.MemberCalendar)
	r.GET("/api/v1/users/:id/administered-calendar", app.calendarController.AdministeredCalendar)
	r.GET("/api/v1/users/:id/calendar-summary", app.calendarController.CalendarSummary)

	// Workout feedback history (workout-feedback-history Task 3)
	r.GET("/api/v1/users/:id/workout-feedback-history", app.workoutFeedbackController.AthleteHistory)
	r.GET("/api/v1/users/:id/administered-workout-feedback-history", app.workoutFeedbackController.AdministeredHistory)

	// Comparación planificado vs realizado (comparacion-planificado-realizado)
	r.GET("/api/v1/activities/comparison", app.activityComparisonController.ListComparisons)
	r.GET("/api/v1/activities/comparison/:sessionInstanceId", app.activityComparisonController.GetSessionComparison)

	// Gamificación: rachas (gamificacion-core E1), self-only.
	r.GET("/api/v1/users/:id/streaks", app.streakController.GetStreaks)
	// Escape del punto rojo (D3): POST seen, self-only.
	r.POST("/api/v1/users/:id/gamification/seen", app.gamificationSeenController.See)

	// Gamificación: catálogo + vitrina de logros y estampas (E2). El catálogo
	// es compartido (sin guard de recurso); la vitrina es self-only.
	r.GET("/api/v1/achievements/catalog", app.achievementsController.GetCatalog)
	r.GET("/api/v1/users/:id/achievements", app.achievementsController.GetUserAchievements)
	r.GET("/api/v1/users/:id/stamps", app.achievementsController.GetUserStamps)

	// Gamificación: leaderboard por equipo (E3). Miembro de algún grupo del
	// team O owner; solo corredores puntúan.
	r.GET("/api/v1/teams/:id/leaderboard", app.leaderboardController.GetLeaderboard)

	// Gamificación: competencias (E4.1). El creation va bajo el team con el
	// guard de caller; la moderación/cierre/detalle bajo el recurso.
	r.POST("/api/v1/teams/:id/competitions", app.competitionController.Create)
	r.GET("/api/v1/teams/:id/competitions", app.competitionController.ListByTeam)
	r.PATCH("/api/v1/competitions/:id/approve", app.competitionController.Approve)
	r.PATCH("/api/v1/competitions/:id/reject", app.competitionController.Reject)
	r.PATCH("/api/v1/competitions/:id/close", app.competitionController.Close)
	r.DELETE("/api/v1/competitions/:id/cancel", app.competitionController.Cancel)
	r.GET("/api/v1/competitions/:id", app.competitionController.GetDetail)
	// Intake de intentos de competencias de ruta (competencias-ruta-intake D2)
	// y listado de intentos (Gap 36 D4).
	r.POST("/api/v1/competitions/:id/attempts", app.competitionController.CreateAttempt)
	r.GET("/api/v1/competitions/:id/attempts", app.competitionController.ListAttempts)

	// Payments (authenticated)
	r.POST("/api/v1/payments/preference", app.paymentController.CreatePreference)
	r.POST("/api/v1/payments", app.paymentController.ProcessPayment)
	// Historial del usuario autenticado. Van antes de /payments/:id para que se
	// lea claro; Gin igual prioriza las rutas estáticas sobre el parámetro.
	r.GET("/api/v1/payments/received", app.paymentHistoryController.ListReceived)
	r.GET("/api/v1/payments/received/summary", app.paymentHistoryController.GetReceivedSummary)
	r.GET("/api/v1/payments/history", app.paymentHistoryController.ListHistory)
	r.GET("/api/v1/payments/:id", app.paymentController.GetPaymentStatus)
	r.GET("/api/v1/payments/mp/:id", app.paymentController.GetPaymentStatusFromMP)
	r.POST("/api/v1/payments/test-card-token", app.paymentController.GenerateTestCardToken)

	// MP Connect (suscripcion-teams-split D7)
	r.GET("/api/v1/mercadopago/connect", app.mpConnectController.GetAuthURL)
	r.GET("/api/v1/mercadopago/connect/status", app.mpConnectController.GetStatus)

	// Platform Settings (suscripcion-teams-split D8)
	r.GET("/api/v1/platform-settings/marketplace-fee", app.platformSettingController.GetMarketplaceFee)
	r.PUT("/api/v1/platform-settings/marketplace-fee", app.platformSettingController.UpdateMarketplaceFee)

	// Team Configuration (configuración por tier del entrenador)
	r.GET("/api/v1/team-configuration", app.teamConfigurationController.GetTeamConfiguration)

	// Team Subscription (suscripcion-teams-split D3)
	r.GET("/api/v1/users/:id/teams/:team_id/subscription", app.teamSubscriptionController.GetTeamSubscription)

	// Attendance (asistencia por QR + gestion de asistencia del entrenador)
	r.GET("/api/v1/attendance/qr", app.attendanceController.GenerateQR)
	r.POST("/api/v1/attendance/team/:team_id/session/:training_session_id", app.attendanceController.RegisterAttendance)
	r.GET("/api/v1/attendance/search", app.attendanceController.Search)
	r.GET("/api/v1/groups/:id/attendance-sessions", app.attendanceController.ListAttendanceSessions)
	r.GET("/api/v1/attendance/session/:session_instance_id", app.attendanceController.GetSessionAttendance)
	// Carga masiva y borrado. El `team_id` va en el body (bulk) o en el query
	// (delete) y no en el path de la asistencia: la asistencia se identifica por
	// su id, y el equipo es contra el que se autoriza, no parte de su identidad.
	r.POST("/api/v1/attendance/bulk", app.attendanceController.BulkSaveAttendance)
	r.DELETE("/api/v1/attendance/:attendance_id", app.attendanceController.DeleteAttendance)

	// Workout Feedback (feedback de entrenamiento)
	r.GET("/api/v1/workout-feedback/search", app.workoutFeedbackController.Search)
	r.GET("/api/v1/workout-feedback/:id/points", app.workoutFeedbackController.GetPoints)
	r.POST("/api/v1/workout-feedback/:id/points", app.workoutFeedbackController.CreatePoints)
	r.GET("/api/v1/workout-feedback/:id", app.workoutFeedbackController.GetByID)
	r.POST("/api/v1/workout-feedback", app.workoutFeedbackController.Create)
	r.PUT("/api/v1/workout-feedback/:id", app.workoutFeedbackController.Update)
	r.DELETE("/api/v1/workout-feedback/:id", app.workoutFeedbackController.Delete)

	// Dashboard analítico. team_id es obligatorio en los endpoints: el
	// acceso es por pestaña del detalle de equipo, así que el equipo siempre se
	// conoce y el scope no se deduce. Leaderboard y comparaciones por corredor/
	// período son del dueño; la comparación de sesiones también admite al corredor
	// para su propio historial y vuelve a validar los dos lados en backend.
	r.GET("/api/v1/dashboard/filters", app.dashboardController.Filters)
	r.GET("/api/v1/dashboard/metrics/summary", app.dashboardController.Summary)
	r.GET("/api/v1/dashboard/metrics/timeseries", app.dashboardController.TimeSeries)
	r.GET("/api/v1/dashboard/leaderboard", app.dashboardController.Leaderboard)
	r.GET("/api/v1/dashboard/exercises/:id/series", app.dashboardController.ExerciseSeries)
	r.POST("/api/v1/dashboard/comparisons", app.dashboardController.Comparisons)
	r.POST("/api/v1/dashboard/comparisons/periods", app.dashboardController.Periods)
	r.POST("/api/v1/dashboard/comparisons/sessions", app.dashboardController.SessionComparison)

	// Runner Session (estado de la sesión del corredor, wip -> finished) y
	// feedback de una sesión asignada. Sin ruta /session-instances/* existente
	// antes: este es el primer bloque bajo ese recurso.
	r.POST("/api/v1/session-instances/:id/runner", app.runnerSessionController.Create)
	r.PATCH("/api/v1/session-instances/:id/runner", app.runnerSessionController.Finish)
	r.GET("/api/v1/session-instances/:id/runner", app.runnerSessionController.Get)
	r.GET("/api/v1/session-instances/:id/feedback", app.workoutFeedbackController.GetBySession)
	// Mensajería de sesión (Gap 27), el chat de una instancia del calendario.
	r.POST("/api/v1/session-instances/:id/messages", app.sessionMessageController.Create)
	r.GET("/api/v1/session-instances/:id/messages", app.sessionMessageController.List)
	r.GET("/api/v1/session-instances/:id", app.calendarController.SessionInstanceDetail)
}
