package app

import (
	"github.com/gin-gonic/gin"
)

func mapUrls(r *gin.Engine, app *Application) {
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
	r.POST("/api/v1/auth/register", app.authController.Register)
	r.POST("/api/v1/auth/login", app.authController.Login)
	r.POST("/api/v1/auth/refresh", app.authController.Refresh)
	r.POST("/api/v1/auth/logout", app.authController.Logout)
	r.GET("/api/v1/auth/user", app.authController.GetUser)
	r.POST("/api/v1/auth/forgot-password", app.passwordResetController.ForgotPassword)
	r.POST("/api/v1/auth/reset-password", app.passwordResetController.ResetPassword)
	r.GET("/example/weather", app.exampleWeatherController.GetWeather)
	r.GET("/user/:user_id/weather", app.userWeatherController.GetUserWithWeather)

	// Payments - Webhook must be public (Mercado Pago sends notifications without auth)
	r.POST("/api/v1/payments/webhook", app.paymentController.HandleWebhook)

	// MP Connect deauthorization webhook - public (Mercado Pago sends it without auth)
	r.POST("/api/v1/mercadopago/webhook/connect", app.mpConnectController.HandleDeauthWebhook)

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
	r.POST("/api/v1/users/:id/roles", app.userRoleController.AssignRole)
	r.DELETE("/api/v1/users/:id/roles/:role_id", app.userRoleController.RemoveRole)
	r.POST("/api/v1/users/:id/trainer-role", app.userRoleController.ActivateEntrenador)
	r.DELETE("/api/v1/users/:id/trainer-role", app.userRoleController.DeactivateEntrenador)

	// Tier subscriptions
	r.PUT("/api/v1/users/:id/roles/:role_id/tier", app.tierSubscriptionController.ChangeTier)
	r.GET("/api/v1/users/:id/subscriptions/current", app.tierSubscriptionController.GetCurrentSubscription)
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
	r.GET("/api/v1/teams/:id", app.teamController.GetByID)
	r.PUT("/api/v1/teams/:id", app.teamController.Update)
	r.DELETE("/api/v1/teams/:id", app.teamController.Delete)
	r.PUT("/api/v1/teams/:id/address", app.teamController.UpdateAddress)

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

	// Payments (authenticated)
	r.POST("/api/v1/payments/preference", app.paymentController.CreatePreference)
	r.POST("/api/v1/payments", app.paymentController.ProcessPayment)
	r.GET("/api/v1/payments/:id", app.paymentController.GetPaymentStatus)
	r.GET("/api/v1/payments/mp/:id", app.paymentController.GetPaymentStatusFromMP)
	r.POST("/api/v1/payments/test-card-token", app.paymentController.GenerateTestCardToken)

	// MP Connect (suscripcion-teams-split D7)
	r.GET("/api/v1/mercadopago/connect", app.mpConnectController.GetAuthURL)
	r.GET("/api/v1/mercadopago/connect/callback", app.mpConnectController.HandleCallback)
	r.GET("/api/v1/mercadopago/connect/status", app.mpConnectController.GetStatus)

	// Platform Settings (suscripcion-teams-split D8)
	r.GET("/api/v1/platform-settings/marketplace-fee", app.platformSettingController.GetMarketplaceFee)
	r.PUT("/api/v1/platform-settings/marketplace-fee", app.platformSettingController.UpdateMarketplaceFee)

	// Team Subscription (suscripcion-teams-split D3)
	r.GET("/api/v1/users/:id/teams/:team_id/subscription", app.teamSubscriptionController.GetTeamSubscription)
}
