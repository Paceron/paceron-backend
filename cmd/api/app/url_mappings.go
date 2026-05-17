package app

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "simple-arq-golang/cmd/api/docs"
)

func mapUrls(r *gin.Engine, app *Application) {
	r.Use(SetRequestID())

	r.GET("/ping", app.pingController.Ping)
	r.GET("/user/:user_id", app.userController.GetUser)
	r.POST("/user", app.userController.CreateUser)
	r.GET("/example/weather", app.exampleWeatherController.GetWeather)
	r.GET("/user/:user_id/weather", app.userWeatherController.GetUserWithWeather)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
