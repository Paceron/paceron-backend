package services

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/user"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/mailer"

	"github.com/gin-gonic/gin"
)

// bankAliasRegex valida el formato de bank_alias — compartido con user_role_service.go
// (activación de entrenador también exige un alias bancario válido).
var bankAliasRegex = regexp.MustCompile(`^[a-zA-Z0-9.\-]{6,20}$`)

const bankAliasFormatError = "bank_alias debe tener entre 6 y 20 caracteres (letras, números, puntos o guiones)"

// searchMinQueryLength y searchResultsLimit acotan /users/search: menos de 3 caracteres
// dispara demasiados resultados (y consultas ILIKE caras) para un autocompletar.
const searchMinQueryLength = 3
const searchResultsLimit = 5

// batchLookupMaxIDs acota /users?ids= — el caso de uso es resolver el roster de un
// equipo/grupo (decenas de miembros como mucho), no un lookup masivo arbitrario.
const batchLookupMaxIDs = 50

type UserServiceInterface interface {
	GetUser(ctx *gin.Context, userID int64) (user.User, error)
	Update(ctx *gin.Context, id int64, req *user.UserUpdateRequest, currentPassword string) (*user.UserUpdateResponse, error)
	ChangeStatus(ctx *gin.Context, id int64, status string) (*user.UserUpdateResponse, error)
	ChangePassword(ctx *gin.Context, id int64, currentPassword, newPassword string) error
	Search(ctx *gin.Context, query string) (*user.SearchResponse, error)
	BatchLookup(ctx *gin.Context, userIDs []int64) (*user.BatchLookupResponse, error)
}

type userService struct {
	userDao daos.UserDaoInterface
	mailer  mailer.MailerInterface
}

func NewUserService(userDao daos.UserDaoInterface, mailerClient mailer.MailerInterface) UserServiceInterface {
	return &userService{
		userDao: userDao,
		mailer:  mailerClient,
	}
}

func (s *userService) GetUser(ctx *gin.Context, userID int64) (user.User, error) {
	userDB, err := s.userDao.GetByID(ctx, userID)
	if err != nil {
		return user.User{}, fmt.Errorf("error getting user: %w", err)
	}
	if userDB == nil {
		return user.User{}, fmt.Errorf("user not found")
	}

	return user.User{
		ID:   userDB.ID,
		Name: userDB.Name,
	}, nil
}

func (s *userService) Search(ctx *gin.Context, query string) (*user.SearchResponse, error) {
	trimmed := strings.TrimSpace(query)
	if len(trimmed) < searchMinQueryLength {
		return nil, fmt.Errorf("la búsqueda requiere al menos %d caracteres", searchMinQueryLength)
	}

	usersDB, err := s.userDao.SearchActive(ctx, trimmed, searchResultsLimit)
	if err != nil {
		customlogger.Error(ctx, "error searching users", err,
			customlogger.Tag("query", trimmed))
		return nil, fmt.Errorf("error al buscar usuarios")
	}

	results := make([]user.SearchResultItem, 0, len(usersDB))
	for _, u := range usersDB {
		results = append(results, user.SearchResultItem{
			UserID:  u.ID,
			Name:    u.Name,
			Surname: u.Surname,
			Email:   u.Email,
		})
	}

	return &user.SearchResponse{Results: results}, nil
}

func (s *userService) BatchLookup(ctx *gin.Context, userIDs []int64) (*user.BatchLookupResponse, error) {
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("se requiere al menos un id de usuario")
	}
	if len(userIDs) > batchLookupMaxIDs {
		return nil, fmt.Errorf("no se pueden consultar más de %d usuarios a la vez", batchLookupMaxIDs)
	}

	usersDB, err := s.userDao.FindByIDs(ctx, userIDs)
	if err != nil {
		customlogger.Error(ctx, "error looking up users in batch", err)
		return nil, fmt.Errorf("error al consultar usuarios")
	}

	results := make([]user.SearchResultItem, 0, len(usersDB))
	for _, u := range usersDB {
		results = append(results, user.SearchResultItem{
			UserID:  u.ID,
			Name:    u.Name,
			Surname: u.Surname,
			Email:   u.Email,
		})
	}

	return &user.BatchLookupResponse{Results: results}, nil
}

func (s *userService) Update(ctx *gin.Context, id int64, req *user.UserUpdateRequest, currentPassword string) (*user.UserUpdateResponse, error) {
	userDB, err := s.userDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding user for update", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", id)),
			customlogger.Tag("step", "find_user"))
		return nil, fmt.Errorf("error al actualizar usuario")
	}
	if userDB == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}

	if req.Email != nil && *req.Email != userDB.Email {
		if currentPassword == "" {
			return nil, fmt.Errorf("para cambiar el email debe proporcionar la contraseña actual (header X-Current-Password)")
		}
		err := bcrypt.CompareHashAndPassword([]byte(userDB.Password), []byte(currentPassword))
		if err != nil {
			customlogger.Warn(ctx, "invalid current password for email change",
				customlogger.Tag("user_id", fmt.Sprintf("%d", id)),
				customlogger.Tag("field", "password"))
			return nil, fmt.Errorf("contraseña actual incorrecta")
		}
		existingEmail, err := s.userDao.FindByEmail(ctx, *req.Email)
		if err != nil {
			customlogger.Error(ctx, "error checking email uniqueness", err,
				customlogger.Tag("email", *req.Email),
				customlogger.Tag("step", "check_email"))
			return nil, fmt.Errorf("error al actualizar usuario")
		}
		if existingEmail != nil {
			customlogger.Warn(ctx, "email already taken",
				customlogger.Tag("email", *req.Email),
				customlogger.Tag("field", "email"))
			return nil, fmt.Errorf("el email ya está registrado")
		}
	}

	if req.Name != nil {
		userDB.Name = strings.TrimSpace(*req.Name)
	}
	if req.Surname != nil {
		userDB.Surname = strings.TrimSpace(*req.Surname)
	}
	if req.Email != nil {
		userDB.Email = strings.TrimSpace(*req.Email)
	}
	if req.Phone != nil {
		userDB.Phone = strings.TrimSpace(*req.Phone)
	}
	if req.PhoneContact != nil {
		userDB.PhoneContact = strings.TrimSpace(*req.PhoneContact)
	}
	if req.Country != nil {
		userDB.Country = strings.TrimSpace(*req.Country)
	}
	if req.Province != nil {
		userDB.Province = strings.TrimSpace(*req.Province)
	}
	if req.City != nil {
		userDB.City = strings.TrimSpace(*req.City)
	}
	if req.Street != nil {
		userDB.Street = strings.TrimSpace(*req.Street)
	}
	if req.Number != nil {
		userDB.Number = strings.TrimSpace(*req.Number)
	}
	if req.Dni != nil {
		userDB.DNI = strings.TrimSpace(*req.Dni)
	}
	if req.BirthDate != nil {
		parsed, err := time.Parse("02/01/2006", strings.TrimSpace(*req.BirthDate))
		if err != nil {
			return nil, fmt.Errorf("birth_date debe tener formato dd/mm/aaaa")
		}
		userDB.BirthDate = parsed
	}
	if req.BankAlias != nil {
		userDB.BankAlias = ptrString(strings.TrimSpace(*req.BankAlias))
	}

	err = s.userDao.Update(ctx, userDB)
	if err != nil {
		customlogger.Error(ctx, "error updating user", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", id)),
			customlogger.Tag("step", "update_user"))
		return nil, fmt.Errorf("error al actualizar usuario")
	}

	customlogger.Info(ctx, "user updated successfully",
		customlogger.Tag("email", userDB.Email),
		customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
		customlogger.TagMethod("Update"))

	return toUserUpdateResponse(userDB), nil
}

func (s *userService) ChangeStatus(ctx *gin.Context, id int64, status string) (*user.UserUpdateResponse, error) {
	userDB, err := s.userDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding user for status change", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", id)),
			customlogger.Tag("step", "find_user"))
		return nil, fmt.Errorf("error al cambiar estado")
	}
	if userDB == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}

	if !constants.IsValidUserStatus(status) {
		return nil, fmt.Errorf("estado inválido: %s. Estados permitidos: %v", status, constants.GetValidUserStatuses())
	}

	previousStatus := userDB.Status

	err = s.userDao.UpdateStatus(ctx, id, status)
	if err != nil {
		customlogger.Error(ctx, "error updating user status", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", id)),
			customlogger.Tag("status", status),
			customlogger.Tag("step", "update_status"))
		return nil, fmt.Errorf("error al cambiar estado")
	}

	userDB.Status = status

	customlogger.Info(ctx, "user status changed successfully",
		customlogger.Tag("email", userDB.Email),
		customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
		customlogger.Tag("status", status),
		customlogger.TagMethod("ChangeStatus"))

	if previousStatus != string(constants.UserStatusInactive) && status == string(constants.UserStatusInactive) {
		if s.mailer != nil {
			if err := s.mailer.SendEmail(ctx, userDB.Email, mailer.EmailTypeFarewell, mailer.EmailData{Name: userDB.Name}); err != nil {
				customlogger.Error(ctx, "error sending farewell email", err,
					customlogger.Tag("email", userDB.Email),
					customlogger.Tag("user_id", fmt.Sprintf("%d", userDB.ID)),
					customlogger.Tag("step", "send_farewell_email"))
			}
		}
	}

	return toUserUpdateResponse(userDB), nil
}

func (s *userService) ChangePassword(ctx *gin.Context, id int64, currentPassword, newPassword string) error {
	userDB, err := s.userDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding user for password change", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", id)),
			customlogger.Tag("step", "find_user"))
		return fmt.Errorf("error al cambiar la contraseña")
	}
	if userDB == nil {
		return fmt.Errorf("usuario no encontrado")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(userDB.Password), []byte(currentPassword)); err != nil {
		customlogger.Warn(ctx, "invalid current password for password change",
			customlogger.Tag("user_id", fmt.Sprintf("%d", id)))
		return fmt.Errorf("contraseña actual incorrecta")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(userDB.Password), []byte(newPassword)); err == nil {
		return fmt.Errorf("la nueva contraseña debe ser distinta a la actual")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		customlogger.Error(ctx, "error hashing new password", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", id)),
			customlogger.Tag("step", "hash_password"))
		return fmt.Errorf("error al cambiar la contraseña")
	}

	now := time.Now()
	userDB.Password = string(hashedPassword)
	userDB.PasswordChangedAt = &now

	if err := s.userDao.Update(ctx, userDB); err != nil {
		customlogger.Error(ctx, "error updating user password", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", id)),
			customlogger.Tag("step", "update_user"))
		return fmt.Errorf("error al cambiar la contraseña")
	}

	customlogger.Info(ctx, "password changed successfully",
		customlogger.Tag("user_id", fmt.Sprintf("%d", id)),
		customlogger.TagMethod("ChangePassword"))

	return nil
}

func toUserUpdateResponse(userDB *dbs.User) *user.UserUpdateResponse {
	return &user.UserUpdateResponse{
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
		BankAlias:    userDB.BankAlias,
	}
}

func ValidateUserUpdateRequest(req *user.UserUpdateRequest) string {
	if req.Name != nil && *req.Name == "" {
		return "name no puede estar vacío"
	}
	if req.Surname != nil && *req.Surname == "" {
		return "surname no puede estar vacío"
	}
	if req.Email != nil && *req.Email != "" {
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		if !emailRegex.MatchString(*req.Email) {
			return "email no tiene un formato válido"
		}
	}
	if req.Phone != nil && *req.Phone != "" {
		match, _ := regexp.MatchString(`^[0-9]+$`, *req.Phone)
		if !match {
			return "phone solo acepta números"
		}
	}
	if req.PhoneContact != nil && *req.PhoneContact != "" {
		match, _ := regexp.MatchString(`^[0-9]+$`, *req.PhoneContact)
		if !match {
			return "phone_contact solo acepta números"
		}
	}
	if req.Country != nil && *req.Country != "" {
		match, _ := regexp.MatchString(`^[a-zA-ZáéíóúÁÉÍÓÚñÑ ]+$`, *req.Country)
		if !match {
			return "country solo acepta letras y espacios"
		}
	}
	if req.City != nil && *req.City != "" {
		match, _ := regexp.MatchString(`^[a-zA-Z0-9áéíóúÁÉÍÓÚñÑ ]+$`, *req.City)
		if !match {
			return "city solo acepta letras, números y espacios"
		}
	}
	if req.Province != nil && *req.Province != "" {
		match, _ := regexp.MatchString(`^[a-zA-Z0-9áéíóúÁÉÍÓÚñÑ ]+$`, *req.Province)
		if !match {
			return "province solo acepta letras, números y espacios"
		}
	}
	if req.Street != nil && *req.Street != "" {
		match, _ := regexp.MatchString(`^[a-zA-Z0-9áéíóúÁÉÍÓÚñÑ ]+$`, *req.Street)
		if !match {
			return "street solo acepta letras, números y espacios"
		}
	}
	if req.Number != nil && *req.Number != "" {
		match, _ := regexp.MatchString(`^[a-zA-Z0-9áéíóúÁÉÍÓÚñÑ ]+$`, *req.Number)
		if !match {
			return "number solo acepta letras, números y espacios"
		}
	}
	if req.Dni != nil && *req.Dni != "" {
		match, _ := regexp.MatchString(`^[0-9]+$`, *req.Dni)
		if !match {
			return "dni solo acepta números"
		}
	}
	if req.BirthDate != nil && *req.BirthDate != "" {
		_, err := time.Parse("02/01/2006", *req.BirthDate)
		if err != nil {
			return "birth_date debe tener formato dd/mm/aaaa"
		}
	}
	if req.BankAlias != nil && *req.BankAlias != "" {
		if !bankAliasRegex.MatchString(*req.BankAlias) {
			return bankAliasFormatError
		}
	}
	return ""
}

func ptrString(s string) *string {
	return &s
}
