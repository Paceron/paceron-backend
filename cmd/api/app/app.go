package app

import (
	"fmt"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/controllers"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/delegates"
	"simple-arq-golang/cmd/api/restclients/exampleweatherclient"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/httpclient"
	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
	"simple-arq-golang/cmd/api/services"
)

type Application struct {
	pingController          controllers.PingController
	userController          controllers.UserController
	exampleWeatherController controllers.ExampleWeatherController
	userWeatherController    controllers.UserWeatherController
}

func NewApplication() *Application {
	// Database
	db, err := postgresdb.ConfigDB(config.MyDB)
	if err != nil {
		fmt.Println("error initializing DB:", err)
	}

	// User flow
	userDao := daos.NewUserDao(db)
	userService := services.NewUserService(userDao)
	userController := controllers.NewUserController(userService)

	// Example Weather flow
	restClientConfig := config.LoadRestClientConfig()
	customlogger.Info(nil, "initializing restclient",
		customlogger.Tag("base_url", restClientConfig.BaseURL),
		customlogger.Tag("timeout", restClientConfig.Timeout.String()),
		customlogger.Tag("max_retries", fmt.Sprintf("%d", restClientConfig.MaxRetries)),
	)

	loggerAdapter := customlogger.NewHTTPClientLogger()
	restClient := httpclient.New(
		httpclient.WithBaseURL(restClientConfig.BaseURL),
		httpclient.WithTimeout(restClientConfig.Timeout),
		httpclient.WithRetry(restClientConfig.MaxRetries, restClientConfig.RetryDelay),
		httpclient.WithLogger(loggerAdapter),
	)

	exampleWeatherClient := exampleweatherclient.New(restClient)
	exampleWeatherService := services.NewExampleWeatherService(exampleWeatherClient)
	exampleWeatherController := controllers.NewExampleWeatherController(exampleWeatherService)

	// Delegate: comunicación entre servicios
	// inyecta servicios en lugar de que se importen entre sí
	userWeatherDelegate := delegates.NewUserWeatherDelegate(userService, exampleWeatherService)
	userWeatherController := controllers.NewUserWeatherController(userWeatherDelegate)

	return &Application{
		pingController:           controllers.NewPingController(),
		userController:           userController,
		exampleWeatherController: exampleWeatherController,
		userWeatherController:    userWeatherController,
	}
}
