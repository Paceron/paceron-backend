package services

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/auth"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/utils"
)

type AuthServiceInterface interface {
	Register(ctx *gin.Context, req *auth.RegisterRequest, password string) (*auth.RegisterResponse, error)
	Login(ctx *gin.Context, email, password string) (*auth.LoginResponse, error)
	GetUser(ctx *gin.Context, id int64, email string) (*auth.RegisterResponse, error)
}

type authService struct {
	authDao daos.AuthDaoInterface
}

func NewAuthService(authDao daos.AuthDaoInterface) AuthServiceInterface {
	return &authService{
		authDao: authDao,
	}
}

func (s *authService) Register(ctx *gin.Context, req *auth.RegisterRequest, password string) (*auth.RegisterResponse, error) {
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

	accessToken, err := utils.GenerateAccessToken(userDB.ID, userDB.Email)
	if err != nil {
		customlogger.Error(ctx, "error generating access token", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "login_generate_access"))
		return nil, fmt.Errorf("No se pudo autenticar")
	}

	refreshToken, err := utils.GenerateRefreshToken(userDB.ID)
	if err != nil {
		customlogger.Error(ctx, "error generating refresh token", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
			customlogger.Tag("step", "login_generate_refresh"))
		return nil, fmt.Errorf("No se pudo autenticar")
	}

	customlogger.Info(ctx, "user logged in successfully",
		customlogger.Tag("email", userDB.Email),
		customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
		customlogger.TagMethod("Login"))

	userResponse := toResponse(userDB)

	return &auth.LoginResponse{
		Authorization: auth.AuthorizationData{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    3600,
		},
		User: *userResponse,
	}, nil
}

func (s *authService) GetUser(ctx *gin.Context, id int64, email string) (*auth.RegisterResponse, error) {
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

func toResponse(userDB *dbs.User) *auth.RegisterResponse {
	return &auth.RegisterResponse{
		UserID:       userDB.ID,
		Name:         userDB.Name,
		Surname:      userDB.Surname,
		Email:        userDB.Email,
		Phone:        userDB.Phone,
		PhoneContact: userDB.PhoneContact,
		Country:      userDB.Country,
		Province:     userDB.Province,
		City:         userDB.City,
		Street:       userDB.Street,
		Number:       userDB.Number,
		Dni:          userDB.DNI,
		BirthDate:    userDB.BirthDate.Format("02/01/2006"),
		Status:       userDB.Status,
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
