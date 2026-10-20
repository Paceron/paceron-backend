package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils"
)

// ---------------------------------------------------------------------------
// Task 10 (D11/S6): el cambio/reset de contraseña revoca los refresh tokens
// activos del usuario. El error de revocación propaga (la contraseña ya cambió
// y es el estado autoritativo, pero las sesiones sobrevivientes son el riesgo
// que este cambio cubre). Los mensajes HTTP no cambian (S6, no-leak).
// ---------------------------------------------------------------------------

func TestChangePassword_RevokesRefreshTokens(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass123"), bcrypt.DefaultCost)
	var revokedUserID int64
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, Email: "john@test.com", Password: string(hashedPassword)}, nil
		},
		mockUpdate: func(ctx *gin.Context, u *dbs.User) error { return nil },
	}
	refreshDao := &mockRefreshTokenDao{
		revokeByUserIDFn: func(ctx *gin.Context, userID int64) error {
			revokedUserID = userID
			return nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, refreshDao, &mockExpoPushClient{}, nil)
	err := svc.ChangePassword(nil, 42, "OldPass123", "NewPass456")

	require.NoError(t, err)
	assert.Equal(t, int64(42), revokedUserID)
}

func TestChangePassword_RevocationErrorPropagates(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass123"), bcrypt.DefaultCost)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, Password: string(hashedPassword)}, nil
		},
		mockUpdate: func(ctx *gin.Context, u *dbs.User) error { return nil },
	}
	updateCalled := false
	refreshDao := &mockRefreshTokenDao{
		revokeByUserIDFn: func(ctx *gin.Context, userID int64) error {
			updateCalled = true
			return assert.AnError
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, refreshDao, &mockExpoPushClient{}, nil)
	err := svc.ChangePassword(nil, 1, "OldPass123", "NewPass456")

	// El mensaje no arrastra el error crudo del DAO (no-leak, mismo 500 de siempre).
	require.EqualError(t, err, "error al cambiar la contraseña")
	assert.True(t, updateCalled)
}

func TestResetPassword_RevokesRefreshTokens(t *testing.T) {
	codeHash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	var revokedUserID int64

	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 7, Email: "user@test.com", Password: "old-hash", Status: "active"}, nil
		},
	}
	userDao := mockUserDao{
		mockUpdate: func(ctx *gin.Context, u *dbs.User) error { return nil },
	}
	resetDao := mockPasswordResetDao{
		mockFindActiveByUserID: func(ctx *gin.Context, userID int64) (*dbs.PasswordResetToken, error) {
			return &dbs.PasswordResetToken{
				ID:        7,
				UserID:    7,
				CodeHash:  string(codeHash),
				ExpiresAt: time.Now().Add(5 * time.Minute),
			}, nil
		},
		mockMarkUsed: func(ctx *gin.Context, id int64) error { return nil },
	}
	refreshDao := &mockRefreshTokenDao{
		revokeByUserIDFn: func(ctx *gin.Context, userID int64) error {
			revokedUserID = userID
			return nil
		},
	}

	svc := NewPasswordResetService(authDao, userDao, resetDao, refreshDao, nil)
	err := svc.ResetPassword(nil, "user@test.com", "123456", "NewPass123")

	require.NoError(t, err)
	assert.Equal(t, int64(7), revokedUserID)
}

func TestResetPassword_RevocationErrorPropagates(t *testing.T) {
	codeHash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 7, Email: "user@test.com", Password: "old-hash", Status: "active"}, nil
		},
	}
	userDao := mockUserDao{
		mockUpdate: func(ctx *gin.Context, u *dbs.User) error { return nil },
	}
	resetDao := mockPasswordResetDao{
		mockFindActiveByUserID: func(ctx *gin.Context, userID int64) (*dbs.PasswordResetToken, error) {
			return &dbs.PasswordResetToken{
				ID:        7,
				UserID:    7,
				CodeHash:  string(codeHash),
				ExpiresAt: time.Now().Add(5 * time.Minute),
			}, nil
		},
		mockMarkUsed: func(ctx *gin.Context, id int64) error { return nil },
	}
	revoked := false
	refreshDao := &mockRefreshTokenDao{
		revokeByUserIDFn: func(ctx *gin.Context, userID int64) error {
			revoked = true
			return assert.AnError
		},
	}

	svc := NewPasswordResetService(authDao, userDao, resetDao, refreshDao, nil)
	err := svc.ResetPassword(nil, "user@test.com", "123456", "NewPass123")

	require.Error(t, err)
	assert.Equal(t, "error al restablecer la contraseña", err.Error())
	assert.True(t, revoked)
}

// TestChangePassword_RefreshTokenRejected punta a punta S6 con daos reales:
// tras ChangePassword el refresh ya no encuentra el token activo — el refresh
// con ese token sigue el mismo camino que hoy para un token inválido/expirado
// (FindActiveByHash → nil → "refresh token inválido o expirado").
func TestChangePassword_RefreshTokenRejected(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"

	db := testutils.SetupTestDB(t)
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass123"), bcrypt.DefaultCost)
	u := &dbs.User{
		Name:      "Rafa",
		Surname:   "S6",
		Email:     "task10-refresh@test.com",
		DNI:       "80000012",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Password:  string(hashedPassword),
		Status:    "active",
	}
	require.NoError(t, db.Create(u).Error)

	rawToken := "raw-refresh-token-to-revoke"
	refreshToken := &dbs.RefreshToken{
		UserID:    u.ID,
		SessionID: "session-task10",
		TokenHash: utils.HashToken(rawToken),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	require.NoError(t, db.Create(refreshToken).Error)

	userService := NewUserService(daos.NewUserDao(db), nil, nil, daos.NewRefreshTokenDao(db), nil, nil)
	require.NoError(t, userService.ChangePassword(nil, u.ID, "OldPass123", "NewPass456"))

	authService := NewAuthService(
		daos.NewAuthDao(db), daos.NewUserRoleDao(db), daos.NewRoleDao(db), daos.NewRefreshTokenDao(db), nil,
	)
	_, err := authService.Refresh(nil, rawToken)

	require.Error(t, err)
	assert.Equal(t, "refresh token inválido o expirado", err.Error())
}
