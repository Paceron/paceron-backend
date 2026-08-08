package app

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

const banner = `
 ____   _    ____ _____ ____   ___  _   _ 
|  _ \ / \  / ___| ____|  _ \ / _ \| \ | |
| |_) / _ \| |   |  _| | |_) | | | |  \| |
|  __/ ___ \ |___| |___|  _ <| |_| | |\  |
|_| /_/   \_\____|_____|_| \_\\___/|_| \_|
`

func StartApp() {
	fmt.Print(banner)
	customlogger.CustomConfig(customlogger.DebugLevel, true, true, true)

	stage := "testing"
	if config.IsProductionStage() {
		stage = "PRODUCTION"
	}
	customlogger.Info(nil, "supabase stage resolved", customlogger.Tag("stage", stage))

	router := gin.Default()
	app := NewApplication()
	mapUrls(router, app)

	for _, route := range router.Routes() {
		customlogger.Info(nil, "endpoint registered",
			customlogger.Tag("method", route.Method),
			customlogger.Tag("path", route.Path),
			customlogger.Tag("handler", route.Handler))
	}

	if err := router.Run(":8080"); err != nil {
		customlogger.Error(nil, "error when trying to start the application", err)
		panic(err)
	}
}
