package app

import (
	"fmt"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/controllers"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/delegates"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/httpclient"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
	"simple-arq-golang/cmd/api/restclients/exampleweatherclient"
	"simple-arq-golang/cmd/api/services"
)

type Application struct {
	pingController             controllers.PingController
	userController             controllers.UserController
	authController             controllers.AuthController
	exampleWeatherController   controllers.ExampleWeatherController
	userWeatherController      controllers.UserWeatherController
	permissionController       controllers.PermissionController
	tierController             controllers.TierController
	roleController             controllers.RoleController
	tierPermissionController   controllers.TierPermissionController
	userRoleController         controllers.UserRoleController
	permissionsQueryController controllers.PermissionsQueryController
	passwordResetController    controllers.PasswordResetController
	teamController             controllers.TeamController
	groupController            controllers.GroupController
	teamUserController         controllers.TeamUserController
	groupUserController        controllers.GroupUserController
	invitationController       controllers.InvitationController
}

func NewApplication() *Application {
	// Database
	db, err := postgresdb.ConfigDB(config.MyDB)
	if err != nil {
		fmt.Println("error initializing DB:", err)
	}

	// Mailer: una única instancia compartida por todos los flujos que envían correo.
	// Si falla su construcción se deja la interfaz en nil (no un *Client nulo), para
	// que los chequeos `mailer != nil` de los services sigan funcionando.
	var mailerClient mailer.MailerInterface
	mailerLogger := customlogger.NewHTTPClientLogger()
	smtpClient, err := mailer.New(
		mailer.WithHost(config.MySMTP.Host),
		mailer.WithPort(config.MySMTP.Port),
		mailer.WithCredentials(config.MySMTP.User, config.MySMTP.AppPassword),
		mailer.WithLogger(mailerLogger),
	)
	if err != nil {
		customlogger.Error(nil, "error initializing mailer", err)
	} else {
		mailerClient = smtpClient
	}

	// User flow
	userDao := daos.NewUserDao(db)
	userService := services.NewUserService(userDao, mailerClient)
	userController := controllers.NewUserController(userService)

	// Auth flow
	authDao := daos.NewAuthDao(db)
	authService := services.NewAuthService(authDao, mailerClient)
	authController := controllers.NewAuthController(authService)

	// Password reset flow
	passwordResetDao := daos.NewPasswordResetDao(db)
	passwordResetService := services.NewPasswordResetService(authDao, userDao, passwordResetDao, mailerClient)
	passwordResetController := controllers.NewPasswordResetController(passwordResetService)

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

	// Permission flow
	permissionDao := daos.NewPermissionDao(db)
	permissionService := services.NewPermissionService(permissionDao)
	permissionController := controllers.NewPermissionController(permissionService)

	// Role flow
	roleDao := daos.NewRoleDao(db)
	roleService := services.NewRoleService(roleDao)
	roleController := controllers.NewRoleController(roleService)

	// Tier flow
	tierDao := daos.NewTierDao(db)
	tierService := services.NewTierService(tierDao, roleDao)
	tierController := controllers.NewTierController(tierService)

	// Tier Permission flow
	tierPermissionDao := daos.NewTierPermissionDao(db)
	tierPermissionService := services.NewTierPermissionService(tierPermissionDao, tierDao, permissionDao)
	tierPermissionController := controllers.NewTierPermissionController(tierPermissionService)

	// User Role flow
	userRoleDao := daos.NewUserRoleDao(db)
	userRoleService := services.NewUserRoleService(userRoleDao, roleDao, tierDao, userDao)
	userRoleController := controllers.NewUserRoleController(userRoleService)

	// Permissions Query flow
	permissionsQueryService := services.NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermissionDao, permissionDao)
	permissionsQueryController := controllers.NewPermissionsQueryController(permissionsQueryService)

	// Team User flow (needed by teamService and groupService)
	teamUserDao := daos.NewTeamUserDao(db)
	teamDao := daos.NewTeamDao(db)

	// Group flow (groupDao/groupUserDao también los necesita teamService para
	// cascadear el soft-delete al eliminar un equipo)
	groupDao := daos.NewGroupDao(db)
	groupUserDao := daos.NewGroupUserDao(db)
	groupService := services.NewGroupService(groupDao, teamDao, teamUserDao)

	// Invitation DAO (también lo necesita teamService para cascadear el soft-delete)
	invitationDao := daos.NewInvitationDao(db)

	// Team flow
	teamService := services.NewTeamService(teamDao, userDao, userRoleDao, roleDao, teamUserDao, groupDao, groupUserDao, invitationDao)

	// Team Delegate (coordina team + group)
	teamDelegate := delegates.NewTeamDelegate(teamService, groupService)
	teamController := controllers.NewTeamController(teamService, teamDelegate)
	groupController := controllers.NewGroupController(groupService)

	teamUserService := services.NewTeamUserService(teamUserDao, teamDao, userDao)
	teamUserController := controllers.NewTeamUserController(teamUserService)

	// Group User flow
	groupUserService := services.NewGroupUserService(groupUserDao, groupDao, userDao)
	groupUserController := controllers.NewGroupUserController(groupUserService)

	// Invitation flow
	invitationService := services.NewInvitationService(userDao, teamDao, invitationDao, teamUserDao, groupDao, groupUserDao, mailerClient)
	invitationController := controllers.NewInvitationController(invitationService)

	return &Application{
		pingController:             controllers.NewPingController(),
		userController:             userController,
		authController:             authController,
		exampleWeatherController:   exampleWeatherController,
		userWeatherController:      userWeatherController,
		permissionController:       permissionController,
		tierController:             tierController,
		roleController:             roleController,
		tierPermissionController:   tierPermissionController,
		userRoleController:         userRoleController,
		permissionsQueryController: permissionsQueryController,
		passwordResetController:    passwordResetController,
		teamController:             teamController,
		groupController:            groupController,
		teamUserController:         teamUserController,
		groupUserController:        groupUserController,
		invitationController:       invitationController,
	}
}
