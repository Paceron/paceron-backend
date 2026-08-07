package services

import (
	"context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
	"github.com/stretchr/testify/assert"
)

// mockMailer es el doble de prueba compartido por todos los tests de services
// que dependen de mailer.MailerInterface. Registra la última invocación para
// poder assertar sin tener que definir un closure en cada test.
type mockMailer struct {
	sendEmailCalled bool
	lastTo          string
	lastEmailType   mailer.EmailType
	lastData        mailer.EmailData

	mockSend      func(ctx context.Context, to, subject, htmlBody string) error
	mockSendEmail func(ctx context.Context, to string, emailType mailer.EmailType, data mailer.EmailData) error
}

func (m *mockMailer) Send(ctx context.Context, to, subject, htmlBody string) error {
	if m.mockSend != nil {
		return m.mockSend(ctx, to, subject, htmlBody)
	}
	return nil
}

func (m *mockMailer) SendEmail(ctx context.Context, to string, emailType mailer.EmailType, data mailer.EmailData) error {
	m.sendEmailCalled = true
	m.lastTo = to
	m.lastEmailType = emailType
	m.lastData = data

	if m.mockSendEmail != nil {
		return m.mockSendEmail(ctx, to, emailType, data)
	}
	return nil
}

type mockUserDao struct {
	mockGetByID      func(ctx *gin.Context, userID int64) (*dbs.User, error)
	mockFindByID     func(ctx *gin.Context, userID int64) (*dbs.User, error)
	mockFindByEmail  func(ctx *gin.Context, email string) (*dbs.User, error)
	mockUpdate       func(ctx *gin.Context, user *dbs.User) error
	mockUpdateStatus func(ctx *gin.Context, userID int64, status string) error
	mockSearchActive func(ctx *gin.Context, query string, limit int) ([]*dbs.User, error)
	mockFindByIDs    func(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error)
}

func (m mockUserDao) GetByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	return m.mockGetByID(ctx, userID)
}

func (m mockUserDao) FindByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	return m.mockFindByID(ctx, userID)
}

func (m mockUserDao) FindByEmail(ctx *gin.Context, email string) (*dbs.User, error) {
	return m.mockFindByEmail(ctx, email)
}

func (m mockUserDao) Update(ctx *gin.Context, user *dbs.User) error {
	return m.mockUpdate(ctx, user)
}

func (m mockUserDao) UpdateStatus(ctx *gin.Context, userID int64, status string) error {
	return m.mockUpdateStatus(ctx, userID, status)
}

func (m mockUserDao) SearchActive(ctx *gin.Context, query string, limit int) ([]*dbs.User, error) {
	return m.mockSearchActive(ctx, query, limit)
}

func (m mockUserDao) FindByIDs(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error) {
	return m.mockFindByIDs(ctx, userIDs)
}

func TestGetUser_Success(t *testing.T) {
	expectedUser := &dbs.User{ID: 1, Name: "test"}

	mockDao := mockUserDao{
		mockGetByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return expectedUser, nil
		},
	}

	service := NewUserService(mockDao, nil)
	result, err := service.GetUser(nil, 1)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, "test", result.Name)
}

func TestGetUser_NotFound(t *testing.T) {
	mockDao := mockUserDao{
		mockGetByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	service := NewUserService(mockDao, nil)
	_, err := service.GetUser(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetUser_DaoError(t *testing.T) {
	mockDao := mockUserDao{
		mockGetByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, errors.New("dao error")
		},
	}

	service := NewUserService(mockDao, nil)
	_, err := service.GetUser(nil, 1)

	assert.Error(t, err)
}

func TestUserService_Search_Success(t *testing.T) {
	mockDao := mockUserDao{
		mockSearchActive: func(ctx *gin.Context, query string, limit int) ([]*dbs.User, error) {
			assert.Equal(t, "ana", query)
			assert.Equal(t, searchResultsLimit, limit)
			return []*dbs.User{
				{ID: 1, Name: "Ana", Surname: "Gomez", Email: "ana@test.com"},
			}, nil
		},
	}

	service := NewUserService(mockDao, nil)
	result, err := service.Search(nil, "ana")

	assert.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, int64(1), result.Results[0].UserID)
	assert.Equal(t, "Ana", result.Results[0].Name)
	assert.Equal(t, "Gomez", result.Results[0].Surname)
	assert.Equal(t, "ana@test.com", result.Results[0].Email)
}

func TestUserService_Search_TrimsQuery(t *testing.T) {
	mockDao := mockUserDao{
		mockSearchActive: func(ctx *gin.Context, query string, limit int) ([]*dbs.User, error) {
			assert.Equal(t, "ana", query)
			return []*dbs.User{}, nil
		},
	}

	service := NewUserService(mockDao, nil)
	_, err := service.Search(nil, "  ana  ")

	assert.NoError(t, err)
}

func TestUserService_Search_QueryTooShort(t *testing.T) {
	service := NewUserService(mockUserDao{}, nil)
	_, err := service.Search(nil, "an")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "al menos")
}

func TestUserService_Search_BlankQuery(t *testing.T) {
	service := NewUserService(mockUserDao{}, nil)
	_, err := service.Search(nil, "   ")

	assert.Error(t, err)
}

func TestUserService_Search_DaoError(t *testing.T) {
	mockDao := mockUserDao{
		mockSearchActive: func(ctx *gin.Context, query string, limit int) ([]*dbs.User, error) {
			return nil, errors.New("dao error")
		},
	}

	service := NewUserService(mockDao, nil)
	_, err := service.Search(nil, "ana")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al buscar usuarios")
}

func TestUserService_BatchLookup_Success(t *testing.T) {
	mockDao := mockUserDao{
		mockFindByIDs: func(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error) {
			assert.Equal(t, []int64{1, 2}, userIDs)
			return []*dbs.User{
				{ID: 1, Name: "Ana", Surname: "Gomez", Email: "ana@test.com"},
				{ID: 2, Name: "Bob", Surname: "Perez", Email: "bob@test.com"},
			}, nil
		},
	}

	service := NewUserService(mockDao, nil)
	result, err := service.BatchLookup(nil, []int64{1, 2})

	assert.NoError(t, err)
	assert.Len(t, result.Results, 2)
	assert.Equal(t, int64(1), result.Results[0].UserID)
	assert.Equal(t, "Ana", result.Results[0].Name)
}

func TestUserService_BatchLookup_EmptyIDs(t *testing.T) {
	service := NewUserService(mockUserDao{}, nil)
	_, err := service.BatchLookup(nil, []int64{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "se requiere al menos un id")
}

func TestUserService_BatchLookup_TooManyIDs(t *testing.T) {
	ids := make([]int64, batchLookupMaxIDs+1)
	for i := range ids {
		ids[i] = int64(i + 1)
	}

	service := NewUserService(mockUserDao{}, nil)
	_, err := service.BatchLookup(nil, ids)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no se pueden consultar más de")
}

func TestUserService_BatchLookup_DaoError(t *testing.T) {
	mockDao := mockUserDao{
		mockFindByIDs: func(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error) {
			return nil, errors.New("dao error")
		},
	}

	service := NewUserService(mockDao, nil)
	_, err := service.BatchLookup(nil, []int64{1})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al consultar usuarios")
}
