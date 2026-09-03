package services

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"golang.org/x/crypto/bcrypt"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/auth"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
	"simple-arq-golang/cmd/api/utils"
)

type AuthServiceInterface interface {
	Register(ctx *gin.Context, req *auth.RegisterRequest, password string) (*auth.UserResponse, error)
	Login(ctx *gin.Context, email, password string) (*auth.LoginResponse, error)
	GetUser(ctx *gin.Context, id int64, email string) (*auth.UserResponse, error)
	Refresh(ctx *gin.Context, refreshToken string) (*auth.RefreshResponse, error)
	Logout(ctx *gin.Context, refreshToken string) error
}

type authService struct {
	authDao         daos.AuthDaoInterface
	userRoleDao     daos.UserRoleDaoInterface
	roleDao         daos.RoleDaoInterface
	refreshTokenDao daos.RefreshTokenDaoInterface
	mailer          mailer.MailerInterface
}

func NewAuthService(
	authDao daos.AuthDaoInterface,
	userRoleDao daos.UserRoleDaoInterface,
	roleDao daos.RoleDaoInterface,
	refreshTokenDao daos.RefreshTokenDaoInterface,
	mailerClient mailer.MailerInterface,
) AuthServiceInterface {
	return &authService{
		authDao:         authDao,
		userRoleDao:     userRoleDao,
		roleDao:         roleDao,
		refreshTokenDao: refreshTokenDao,
		mailer:          mailerClient,
	}
}

func (s *authService) Register(ctx *gin.Context, req *auth.RegisterRequest, password string) (*auth.UserResponse, error) {
	existingEmail, err := s.authDao.FindByEmail(ctx, req.Email)
	if err != nil {
		customlogger.Error(ctx, "error checking existing email", err,
			customlogger.Tag("email", req.Email),
			customlogger.Tag("step", "find_by_email"))
		return nil, fmt.Errorf("error al registrar usuario")
	}
	if existingEmail != nil {
		customlogger.Warn(ctx, "email already registered",
			customlogger.Tag("email", req.Email),
			customlogger.Tag("field", "email"))
		return nil, fmt.Errorf("el email ya está registrado")
	}

	existingDNI, err := s.authDao.FindByDNI(ctx, req.Dni)
	if err != nil {
		customlogger.Error(ctx, "error checking existing dni", err,
			customlogger.Tag("dni", req.Dni),
			customlogger.Tag("step", "find_by_dni"))
		return nil, fmt.Errorf("error al registrar usuario")
	}
	if existingDNI != nil {
		customlogger.Warn(ctx, "dni already registered",
			customlogger.Tag("dni", req.Dni),
			customlogger.Tag("field", "dni"))
		return nil, fmt.Errorf("el DNI ya está registrado")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		customlogger.Error(ctx, "error hashing password", err,
			customlogger.Tag("step", "hash_password"))
		return nil, fmt.Errorf("error al registrar usuario")
	}

	userDB := toDBModel(req, string(hashedPassword))
	createdUser, err := s.authDao.Create(ctx, userDB)
	if err != nil {
		customlogger.Error(ctx, "error creating user", err,
			customlogger.Tag("email", req.Email),
			customlogger.Tag("step", "create_user"))
		return nil, fmt.Errorf("error al registrar usuario")
	}

	customlogger.Info(ctx, "user registered successfully",
		customlogger.Tag("email", createdUser.Email),
		customlogger.TagMethod("Register"))

	if s.mailer != nil {
		if err := s.mailer.SendEmail(ctx, createdUser.Email, mailer.EmailTypeWelcome, mailer.EmailData{Name: createdUser.Name}); err != nil {
			customlogger.Error(ctx, "error sending welcome email", err,
				customlogger.Tag("email", createdUser.Email),
				customlogger.Tag("step", "send_welcome_email"))
		}
	}

	return toResponse(createdUser), nil
}

func (s *authService) Login(ctx *gin.Context, email, password string) (*auth.LoginResponse, error) {
	userDB, err := s.authDao.FindByEmail(ctx, email)
	if err != nil {
		customlogger.Error(ctx, "error finding user by email", err,
			customlogger.Tag("email", email),
			customlogger.Tag("step", "login_find_user"))
		return nil, fmt.Errorf("No se pudo autenticar")
	}
	if userDB == nil {
		customlogger.Warn(ctx, "login attempt with non-existent email",
			customlogger.Tag("email", email),
			customlogger.Tag("step", "login_user_not_found"))
		return nil, fmt.Errorf("No se pudo autenticar")
	}

	if userDB.Status != string(constants.UserStatusActive) {
		customlogger.Warn(ctx, "login attempt with inactive user",
			customlogger.Tag("email", email),
			customlogger.Tag("status", userDB.Status),
			customlogger.Tag("step", "login_inactive"))
		return nil, fmt.Errorf("No se pudo autenticar")
	}

	err = bcrypt.CompareHashAndPassword([]byte(userDB.Password), []byte(password))
	if err != nil {
		customlogger.Warn(ctx, "login attempt with wrong password",
			customlogger.Tag("email", email),
			customlogger.Tag("step", "login_wrong_password"))
		return nil, fmt.Errorf("No se pudo autenticar")
	}

	roles, err := s.roleNamesForUser(ctx, userDB.ID)
	if err != nil {
		customlogger.Error(ctx, "error loading roles for login", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "login_load_roles"))
		return nil, fmt.Errorf("No se pudo autenticar")
	}

	sessionUUID, err := uuid.NewV4()
	if err != nil {
		customlogger.Error(ctx, "error generating session id", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "login_generate_session"))
		return nil, fmt.Errorf("No se pudo autenticar")
	}
	sessionID := sessionUUID.String()

	accessToken, err := utils.GenerateAccessToken(userDB.ID, sessionID, roles)
	if err != nil {
		customlogger.Error(ctx, "error generating access token", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "login_generate_access"))
		return nil, fmt.Errorf("No se pudo autenticar")
	}

	refreshToken, err := s.issueRefreshToken(ctx, userDB.ID, sessionID, "Login")
	if err != nil {
		return nil, fmt.Errorf("No se pudo autenticar")
	}

	customlogger.Info(ctx, "user logged in successfully",
		customlogger.Tag("email", userDB.Email),
		customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
		customlogger.TagMethod("Login"))

	userResponse := toResponse(userDB)

	return &auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(config.AccessTokenDuration.Seconds()),
		User:         *userResponse,
	}, nil
}

// Refresh valida un refresh token activo, lo rota (revoca el usado, emite uno nuevo con
// la misma sesión) y emite un access token nuevo.
func (s *authService) Refresh(ctx *gin.Context, refreshToken string) (*auth.RefreshResponse, error) {
	genericErr := fmt.Errorf("refresh token inválido o expirado")

	tokenHash := utils.HashToken(refreshToken)
	existing, err := s.refreshTokenDao.FindActiveByHash(ctx, tokenHash)
	if err != nil {
		customlogger.Error(ctx, "error finding refresh token", err,
			customlogger.Tag("step", "refresh_find_token"))
		return nil, fmt.Errorf("error al renovar la sesión")
	}
	if existing == nil {
		customlogger.Warn(ctx, "refresh attempt with invalid or expired token",
			customlogger.Tag("step", "refresh_invalid_token"))
		return nil, genericErr
	}

	userDB, err := s.authDao.FindByID(ctx, existing.UserID)
	if err != nil {
		customlogger.Error(ctx, "error finding user for refresh", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", existing.UserID)),
			customlogger.Tag("step", "refresh_find_user"))
		return nil, fmt.Errorf("error al renovar la sesión")
	}
	if userDB == nil || userDB.Status != string(constants.UserStatusActive) {
		customlogger.Warn(ctx, "refresh attempt for missing or inactive user",
			customlogger.Tag("user_id", fmt.Sprintf("%d", existing.UserID)),
			customlogger.Tag("step", "refresh_inactive_user"))
		return nil, genericErr
	}

	roles, err := s.roleNamesForUser(ctx, userDB.ID)
	if err != nil {
		customlogger.Error(ctx, "error loading roles for refresh", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "refresh_load_roles"))
		return nil, fmt.Errorf("error al renovar la sesión")
	}

	newRefreshToken, newTokenID, err := s.rotateRefreshToken(ctx, existing)
	if err != nil {
		return nil, fmt.Errorf("error al renovar la sesión")
	}

	accessToken, err := utils.GenerateAccessToken(userDB.ID, existing.SessionID, roles)
	if err != nil {
		customlogger.Error(ctx, "error generating access token on refresh", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "refresh_generate_access"))
		return nil, fmt.Errorf("error al renovar la sesión")
	}

	customlogger.Info(ctx, "refresh token rotated successfully",
		customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
		customlogger.Tag("session_id", existing.SessionID),
		customlogger.Tag("new_token_id", fmt.Sprintf("%d", newTokenID)),
		customlogger.TagMethod("Refresh"))

	return &auth.RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int(config.AccessTokenDuration.Seconds()),
	}, nil
}

// Logout revoca la sesión de refresh asociada al token. Idempotente: si el token no
// existe o ya estaba revocado, igual responde éxito (no filtra información).
func (s *authService) Logout(ctx *gin.Context, refreshToken string) error {
	tokenHash := utils.HashToken(refreshToken)
	existing, err := s.refreshTokenDao.FindActiveByHash(ctx, tokenHash)
	if err != nil {
		customlogger.Error(ctx, "error finding refresh token for logout", err,
			customlogger.Tag("step", "logout_find_token"))
		return fmt.Errorf("error al cerrar sesión")
	}
	if existing == nil {
		return nil
	}

	if err := s.refreshTokenDao.Revoke(ctx, existing.ID, nil); err != nil {
		customlogger.Error(ctx, "error revoking refresh token on logout", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", existing.UserID)),
			customlogger.Tag("step", "logout_revoke_token"))
		return fmt.Errorf("error al cerrar sesión")
	}

	customlogger.Info(ctx, "user logged out successfully",
		customlogger.Tag("user_id", fmt.Sprintf("%d", existing.UserID)),
		customlogger.Tag("session_id", existing.SessionID),
		customlogger.TagMethod("Logout"))

	return nil
}

// issueRefreshToken genera, persiste y devuelve un refresh token opaco nuevo para una sesión.
func (s *authService) issueRefreshToken(ctx *gin.Context, userID int64, sessionID string, method string) (string, error) {
	refreshToken, err := utils.GenerateOpaqueToken()
	if err != nil {
		customlogger.Error(ctx, "error generating refresh token", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod(method))
		return "", err
	}

	tokenDB := &dbs.RefreshToken{
		UserID:    userID,
		SessionID: sessionID,
		TokenHash: utils.HashToken(refreshToken),
		ExpiresAt: time.Now().Add(config.RefreshTokenDuration),
	}
	if err := s.refreshTokenDao.Create(ctx, tokenDB); err != nil {
		customlogger.Error(ctx, "error persisting refresh token", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod(method))
		return "", err
	}

	return refreshToken, nil
}

// rotateRefreshToken emite un refresh token nuevo para la misma sesión y revoca el
// anterior, dejando el rastro de reemplazo (replaced_by).
func (s *authService) rotateRefreshToken(ctx *gin.Context, existing *dbs.RefreshToken) (string, int64, error) {
	newRefreshToken, err := utils.GenerateOpaqueToken()
	if err != nil {
		customlogger.Error(ctx, "error generating rotated refresh token", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", existing.UserID)),
			customlogger.TagMethod("Refresh"))
		return "", 0, err
	}

	newTokenDB := &dbs.RefreshToken{
		UserID:    existing.UserID,
		SessionID: existing.SessionID,
		TokenHash: utils.HashToken(newRefreshToken),
		ExpiresAt: time.Now().Add(config.RefreshTokenDuration),
	}
	if err := s.refreshTokenDao.Create(ctx, newTokenDB); err != nil {
		customlogger.Error(ctx, "error persisting rotated refresh token", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", existing.UserID)),
			customlogger.TagMethod("Refresh"))
		return "", 0, err
	}

	if err := s.refreshTokenDao.Revoke(ctx, existing.ID, &newTokenDB.ID); err != nil {
		customlogger.Error(ctx, "error revoking previous refresh token", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", existing.UserID)),
			customlogger.Tag("old_token_id", fmt.Sprintf("%d", existing.ID)),
			customlogger.TagMethod("Refresh"))
		return "", 0, err
	}

	return newRefreshToken, newTokenDB.ID, nil
}

// roleNamesForUser resuelve los nombres de los roles globales de un usuario (el sistema
// de roles ya existente vía user_role/role — no confundir con TeamUser.RoleInTeam, que
// es un rol por equipo, no global).
func (s *authService) roleNamesForUser(ctx *gin.Context, userID int64) ([]string, error) {
	userRoles, err := s.userRoleDao.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(userRoles))
	for _, ur := range userRoles {
		role, err := s.roleDao.FindByID(ctx, ur.RoleID)
		if err != nil {
			continue
		}
		if role != nil {
			names = append(names, role.Name)
		}
	}
	return names, nil
}

func (s *authService) GetUser(ctx *gin.Context, id int64, email string) (*auth.UserResponse, error) {
	var user *dbs.User
	var err error

	if id > 0 && email != "" {
		return nil, fmt.Errorf("debe proporcionar solo id o email, no ambos")
	}
	if id <= 0 && email == "" {
		return nil, fmt.Errorf("debe proporcionar id o email")
	}

	if email != "" {
		user, err = s.authDao.FindByEmail(ctx, email)
	} else {
		user, err = s.authDao.FindByID(ctx, id)
	}

	if err != nil {
		customlogger.Error(ctx, "error getting user", err,
			customlogger.Tag("step", "get_user"))
		return nil, fmt.Errorf("error al obtener usuario")
	}
	if user == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}

	customlogger.Info(ctx, "user retrieved successfully",
		customlogger.Tag("email", user.Email),
		customlogger.Tag("user_id", fmt.Sprintf("%d", user.ID)),
		customlogger.TagMethod("GetUser"))

	return toResponse(user), nil
}

func toDBModel(req *auth.RegisterRequest, hashedPassword string) *dbs.User {
	birthDate, _ := time.Parse("02/01/2006", req.BirthDate)

	return &dbs.User{
		Name:         strings.TrimSpace(req.Name),
		Surname:      strings.TrimSpace(req.Surname),
		Email:        strings.TrimSpace(req.Email),
		Phone:        strings.TrimSpace(req.Phone),
		PhoneContact: strings.TrimSpace(req.PhoneContact),
		Country:      strings.TrimSpace(req.Country),
		Province:     strings.TrimSpace(req.Province),
		City:         strings.TrimSpace(req.City),
		Street:       strings.TrimSpace(req.Street),
		Number:       strings.TrimSpace(req.Number),
		DNI:          strings.TrimSpace(req.Dni),
		BirthDate:    birthDate,
		Password:     hashedPassword,
		Status:       string(constants.UserStatusActive),
	}
}

func toResponse(userDB *dbs.User) *auth.UserResponse {
	return &auth.UserResponse{
		UserID:       userDB.ID,
		Name:         userDB.Name,
		Surname:      userDB.Surname,
		Email:        userDB.Email,
		Phone:        userDB.Phone,
		PhoneContact: userDB.PhoneContact,
		Country:      userDB.Country,
		BankAlias:    ptrStringDeref(userDB.BankAlias),
		Province:     userDB.Province,
		City:         userDB.City,
		Street:       userDB.Street,
		Number:       userDB.Number,
		Dni:          userDB.DNI,
		BirthDate:    userDB.BirthDate.Format("02/01/2006"),
		Status:       userDB.Status,
		PhotoURL:     buildMediaURL(userDB.PhotoKey, userDB.PhotoUpdatedAt),
	}
}

func ValidateRegisterRequest(req *auth.RegisterRequest) string {
	if req.Name == "" {
		return "name es requerido"
	}
	if req.Surname == "" {
		return "surname es requerido"
	}
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(req.Email) {
		return "email no tiene un formato válido"
	}
	if req.Phone != "" {
		match, _ := regexp.MatchString(`^[0-9]+$`, req.Phone)
		if !match {
			return "phone solo acepta números"
		}
	}
	if req.PhoneContact != "" {
		match, _ := regexp.MatchString(`^[0-9]+$`, req.PhoneContact)
		if !match {
			return "phone_contact solo acepta números"
		}
	}
	if req.Country != "" {
		match, _ := regexp.MatchString(`^[a-zA-ZáéíóúÁÉÍÓÚñÑ ]+$`, req.Country)
		if !match {
			return "country solo acepta letras y espacios"
		}
	}
	if req.City != "" {
		match, _ := regexp.MatchString(`^[a-zA-Z0-9áéíóúÁÉÍÓÚñÑ ]+$`, req.City)
		if !match {
			return "city solo acepta letras, números y espacios"
		}
	}
	if req.Street != "" {
		match, _ := regexp.MatchString(`^[a-zA-Z0-9áéíóúÁÉÍÓÚñÑ ]+$`, req.Street)
		if !match {
			return "street solo acepta letras, números y espacios"
		}
	}
	if req.Number != "" {
		match, _ := regexp.MatchString(`^[a-zA-Z0-9áéíóúÁÉÍÓÚñÑ ]+$`, req.Number)
		if !match {
			return "number solo acepta letras, números y espacios"
		}
	}
	if req.Dni == "" {
		return "dni es requerido"
	}
	match, _ := regexp.MatchString(`^[0-9]+$`, req.Dni)
	if !match {
		return "dni solo acepta números"
	}
	if req.BirthDate == "" {
		return "birth_date es requerido"
	}
	_, err := time.Parse("02/01/2006", req.BirthDate)
	if err != nil {
		return "birth_date debe tener formato dd/mm/aaaa"
	}
	return ""
}

func ValidatePassword(password string) string {
	if password == "" {
		return "la contraseña es requerida"
	}
	if len(password) < 8 {
		return "la contraseña debe tener al menos 8 caracteres"
	}
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	if !hasUpper {
		return "la contraseña debe contener al menos una letra mayúscula"
	}
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	if !hasLower {
		return "la contraseña debe contener al menos una letra minúscula"
	}
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
	if !hasDigit {
		return "la contraseña debe contener al menos un número"
	}
	sensitiveRegex := regexp.MustCompile(`[<>"'&;%]`)
	if sensitiveRegex.MatchString(password) {
		return "la contraseña contiene caracteres no permitidos"
	}
	return ""
}

func ptrStringDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
