package app

import (
	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

func StartApp() {
	customlogger.CustomConfig(customlogger.DebugLevel, true, true, true)

	router := gin.Default()
	app := NewApplication()
	mapUrls(router, app)

	if err := router.Run(":8080"); err != nil {
		customlogger.Error(nil, "error when trying to start the application", err)
		panic(err)
	}
}
