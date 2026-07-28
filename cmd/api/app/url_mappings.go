package app

import (
	"github.com/gin-gonic/gin"
)

func mapUrls(r *gin.Engine, app *Application) {
	r.Use(CORSMiddleware())
	r.Use(SetRequestID())

	r.GET("/ping", app.pingController.Ping)
	r.GET("/user/:user_id", app.userController.GetUser)
	r.POST("/user", app.userController.CreateUser)
	r.POST("/api/v1/auth/register", app.authController.Register)
	r.POST("/api/v1/auth/login", app.authController.Login)
	r.GET("/api/v1/auth/user", app.authController.GetUser)
	r.POST("/api/v1/auth/forgot-password", app.passwordResetController.ForgotPassword)
	r.POST("/api/v1/auth/reset-password", app.passwordResetController.ResetPassword)
	r.GET("/api/v1/auth/permissions", app.permissionsQueryController.GetUserPermissions)
	r.PUT("/api/v1/users/:id", app.userController.Update)
	r.PATCH("/api/v1/users/:id/status", app.userController.ChangeStatus)
	r.PATCH("/api/v1/users/:id/password", app.userController.ChangePassword)
	r.POST("/api/v1/users/:id/roles", app.userRoleController.AssignRole)
	r.DELETE("/api/v1/users/:id/roles/:role_id", app.userRoleController.RemoveRole)
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
	r.GET("/example/weather", app.exampleWeatherController.GetWeather)
	r.GET("/user/:user_id/weather", app.userWeatherController.GetUserWithWeather)

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

	mapSwagger(r)
	mapGuide(r)
}
