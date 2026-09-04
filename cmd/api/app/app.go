package app

import (
	"context"
	"fmt"
	"time"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/controllers"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/delegates"
	"simple-arq-golang/cmd/api/infrastructure/crypto"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/httpclient"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
	"simple-arq-golang/cmd/api/restclients/exampleweatherclient"
	"simple-arq-golang/cmd/api/restclients/expopushclient"
	"simple-arq-golang/cmd/api/restclients/mercadopagoclient"
	"simple-arq-golang/cmd/api/restclients/storageclient"
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
	pushTokenController        controllers.PushTokenController
	paymentController          controllers.PaymentController
	tierSubscriptionController controllers.TierSubscriptionController
	// suscripcion-teams-split
	mpConnectController       controllers.MPConnectControllerInterface
	platformSettingController controllers.PlatformSettingControllerInterface
	teamSubscriptionController controllers.TeamSubscriptionControllerInterface
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
	resendClient, err := mailer.New(
		mailer.WithAPIKey(config.MyMailer.APIKey),
		mailer.WithFrom(config.MyMailer.From),
		mailer.WithLogger(mailerLogger),
	)
	if err != nil {
		customlogger.Error(nil, "error initializing mailer", err)
	} else {
		mailerClient = resendClient
	}

	// Push notifications: cliente HTTP plano contra la API pública de Expo, sin SDK
	// (mismo patrón que exampleWeatherClient). pushTokenDao se comparte entre todos
	// los services que disparan notificaciones (user, team_user, invitation).
	pushTokenDao := daos.NewPushTokenDao(db)
	expoPushHTTPClient := httpclient.New(
		httpclient.WithBaseURL("https://exp.host"),
		httpclient.WithTimeout(8*time.Second),
		httpclient.WithRetry(2, 500*time.Millisecond),
		httpclient.WithLogger(customlogger.NewHTTPClientLogger()),
	)
	expoPushClient := expopushclient.New(expoPushHTTPClient)

	// Storage: fotos de perfil de usuario e ícono de equipo, S3-compatible contra
	// Supabase Storage (testing/producción resuelto por config.IsProductionStage()).
	storageClientInstance, err := storageclient.New(context.Background(), storageclient.Options{
		Endpoint:        config.MyStorage.Endpoint,
		Region:          config.MyStorage.Region,
		AccessKeyID:     config.MyStorage.AccessKeyID,
		SecretAccessKey: config.MyStorage.SecretAccessKey,
		Bucket:          config.MyStorage.Bucket,
	})
	if err != nil {
		customlogger.Error(nil, "error initializing storage client", err)
	}

	// User flow
	userDao := daos.NewUserDao(db)
	userService := services.NewUserService(userDao, mailerClient, pushTokenDao, expoPushClient, storageClientInstance)
	userController := controllers.NewUserController(userService)

	// Role flow (roleDao/userRoleDao también los necesita authService para los claims
	// de roles del access token)
	roleDao := daos.NewRoleDao(db)
	roleService := services.NewRoleService(roleDao)
	roleController := controllers.NewRoleController(roleService)

	userRoleDao := daos.NewUserRoleDao(db)

	// Auth flow
	authDao := daos.NewAuthDao(db)
	refreshTokenDao := daos.NewRefreshTokenDao(db)
	authService := services.NewAuthService(authDao, userRoleDao, roleDao, refreshTokenDao, mailerClient)
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

	// Tier flow
	tierDao := daos.NewTierDao(db)
	tierService := services.NewTierService(tierDao, roleDao)
	tierController := controllers.NewTierController(tierService)

	// Tier Permission flow
	tierPermissionDao := daos.NewTierPermissionDao(db)
	tierPermissionService := services.NewTierPermissionService(tierPermissionDao, tierDao, permissionDao)
	tierPermissionController := controllers.NewTierPermissionController(tierPermissionService)

	// Team User flow (needed by teamService, groupService, and userRoleService's
	// entrenador activation/deactivation)
	teamUserDao := daos.NewTeamUserDao(db)
	teamDao := daos.NewTeamDao(db)
	installmentDao := daos.NewInstallmentDao(db) // ledger compartido tier/equipo (change suscripcion-teams-split)

	// User Role flow
	userRoleService := services.NewUserRoleService(userRoleDao, roleDao, tierDao, userDao, teamUserDao, db)
	userRoleController := controllers.NewUserRoleController(userRoleService)

	// Permissions Query flow
	permissionsQueryService := services.NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermissionDao, permissionDao)
	permissionsQueryController := controllers.NewPermissionsQueryController(permissionsQueryService)

	// Group flow (groupDao/groupUserDao también los necesita teamService para
	// cascadear el soft-delete al eliminar un equipo)
	groupDao := daos.NewGroupDao(db)
	groupUserDao := daos.NewGroupUserDao(db)
	groupService := services.NewGroupService(groupDao, teamDao, teamUserDao)

	// Invitation DAO (también lo necesita teamService para cascadear el soft-delete)
	invitationDao := daos.NewInvitationDao(db)

	// Team flow
	teamService := services.NewTeamService(teamDao, userDao, userRoleDao, roleDao, teamUserDao, groupDao, groupUserDao, invitationDao, storageClientInstance)

	// Team Delegate (coordina team + group)
	teamDelegate := delegates.NewTeamDelegate(teamService, groupService)
	teamController := controllers.NewTeamController(teamService, teamDelegate)
	groupController := controllers.NewGroupController(groupService)

	teamUserService := services.NewTeamUserService(teamUserDao, teamDao, userDao, groupDao, groupUserDao, mailerClient, pushTokenDao, expoPushClient, installmentDao, db)
	teamUserController := controllers.NewTeamUserController(teamUserService)

	// Group User flow
	groupUserService := services.NewGroupUserService(groupUserDao, groupDao, userDao, teamUserDao)
	groupUserController := controllers.NewGroupUserController(groupUserService)

	// Invitation flow
	invitationService := services.NewInvitationService(userDao, teamDao, invitationDao, teamUserDao, groupDao, groupUserDao, mailerClient, pushTokenDao, expoPushClient, installmentDao, db)
	invitationController := controllers.NewInvitationController(invitationService)

	// Push token flow
	pushTokenService := services.NewPushTokenService(pushTokenDao)
	pushTokenController := controllers.NewPushTokenController(pushTokenService)

	// DAOs para split de equipos
	sellerConnDao := daos.NewSellerConnectionDao(db)
	settingDao := daos.NewPlatformSettingDao(db)
	encryptor := crypto.NewAESGCMEncryptor(config.TokenEncryptionKey)
	mpClient := mercadopagoclient.New()

	// MP Connect flow
	mpConnectService := services.NewMPConnectService(sellerConnDao, mpClient, encryptor,
		config.MyMP.OAuthClientID, config.MyMP.OAuthClientSecret, config.MyMP.OAuthRedirectURI)
	mpConnectController := controllers.NewMPConnectController(mpConnectService)

	// Platform Settings flow
	platformSettingService := services.NewPlatformSettingService(settingDao, userDao)
	platformSettingController := controllers.NewPlatformSettingController(platformSettingService)

	// Payment flow (con dependencias para split de equipos)
	paymentDao := daos.NewPaymentDao(db)
	paymentService := services.NewPaymentService(paymentDao, mpClient, db,
		sellerConnDao, teamDao, teamUserDao, settingDao, installmentDao, encryptor)
	paymentController := controllers.NewPaymentController(paymentService)

	// Tier subscription flow (ledger de suscripciones de tier por usuario/rol)
	tierSubscriptionDao := daos.NewTierSubscriptionDao(db)
	tierSubscriptionService := services.NewTierSubscriptionService(db, userRoleDao, roleDao, tierDao, tierSubscriptionDao, installmentDao)
	tierSubscriptionController := controllers.NewTierSubscriptionController(tierSubscriptionService)

	// Team Subscription flow (D3: GET /api/v1/users/:id/teams/:team_id/subscription)
	teamSubscriptionService := services.NewTeamSubscriptionService(teamDao, teamUserDao, installmentDao)
	teamSubscriptionController := controllers.NewTeamSubscriptionController(teamSubscriptionService)

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
		pushTokenController:        pushTokenController,
		paymentController:          paymentController,
		tierSubscriptionController: tierSubscriptionController,
		mpConnectController:        mpConnectController,
		platformSettingController:  platformSettingController,
		teamSubscriptionController: teamSubscriptionController,
	}
}
