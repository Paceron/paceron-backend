package app

import (
	"github.com/gin-gonic/gin"
)

func mapUrls(r *gin.Engine, app *Application) {
	r.Use(SetRequestID())

	r.GET("/ping", app.pingController.Ping)
	r.GET("/user/:user_id", app.userController.GetUser)
	r.POST("/user", app.userController.CreateUser)
	r.POST("/api/v1/auth/register", app.authController.Register)
	r.POST("/api/v1/auth/login", app.authController.Login)
	r.GET("/api/v1/auth/user", app.authController.GetUser)
	r.PUT("/api/v1/users/:id", app.userController.Update)
	r.PATCH("/api/v1/users/:id/status", app.userController.ChangeStatus)
	r.GET("/example/weather", app.exampleWeatherController.GetWeather)
	r.GET("/user/:user_id/weather", app.userWeatherController.GetUserWithWeather)

	mapSwagger(r)
}
