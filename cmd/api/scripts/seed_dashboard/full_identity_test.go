package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/controllers"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
)

func TestFullSeederSeedIdentity_SeedsDashboardIdentityContract(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := newFullSeeder(db, options{})
	s.now = time.Date(2026, time.July, 18, 15, 0, 0, 0, gametime.Location)

	require.NoError(t, s.seedIdentity())

	assert.Len(t, s.ids.users, 18)
	for key, id := range s.ids.users {
		assert.Positive(t, id, key)
	}
	for _, fixture := range []struct {
		key         string
		email       string
		status      string
		hasRole     bool
		invitations bool
	}{
		{"trainer_1", "trainer1.seed-full@paceron.test", "active", true, true},
		{"trainer_2", "trainer2.seed-full@paceron.test", "active", true, true},
		{"r1", "runner1.seed-full@paceron.test", "active", true, true},
		{"r12", "runner12.seed-full@paceron.test", "active", true, true},
		{"blocked", "blocked.seed-full@paceron.test", "blocked", true, true},
		{"inactive", "inactive.seed-full@paceron.test", "inactive", true, true},
		{"no_role", "no-role.seed-full@paceron.test", "active", false, true},
		{"invitations_off", "invitations-off.seed-full@paceron.test", "active", true, false},
	} {
		user := fullIdentityUserByID(t, db, s.ids.users[fixture.key])
		assert.Equal(t, fixture.email, user.Email)
		assert.Equal(t, fixture.status, user.Status)
		assert.Equal(t, fixture.invitations, user.AllowTeamInvitations, fixture.key)
		assert.True(t, user.CreatedAt.Equal(s.now), fixture.key)
		assert.True(t, user.UpdatedAt.Equal(s.now), fixture.key)
		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("Paceron.123")))

		var roleCount int64
		require.NoError(t, db.Model(&dbs.UserRole{}).Where("user_id = ? AND deleted_at IS NULL", user.ID).Count(&roleCount).Error)
		if fixture.hasRole {
			assert.Positive(t, roleCount, fixture.key)
		} else {
			assert.Zero(t, roleCount, fixture.key)
		}
	}

	trainerRoles := fullIdentityRoleNames(t, db, s.ids.users["trainer_1"])
	assert.Equal(t, []string{"corredor", "entrenador"}, trainerRoles)
	for i := 1; i <= 12; i++ {
		assert.Equal(t, []string{"corredor"}, fullIdentityRoleNames(t, db, s.ids.users[fmt.Sprintf("r%d", i)]))
	}

	assert.Len(t, s.ids.roles, 2)
	assert.Len(t, s.ids.tiers, 6)
	assert.Len(t, s.ids.subscriptions, 4)
	for _, roleName := range []string{"corredor", "entrenador"} {
		for hierarchy, tierName := range []string{"base", "medium", "premium"} {
			tier := loadFullIdentityTier(t, db, roleName, tierName)
			assert.Equal(t, hierarchy+1, tier.Hierarchy)
			assert.Equal(t, roleName, tier.RoleName)
			assert.Equal(t, hierarchy > 0, tier.PaymentRequired)
			var permissionCount int64
			require.NoError(t, db.Model(&dbs.TierPermission{}).Where("tier_id = ? AND deleted_at IS NULL", tier.ID).Count(&permissionCount).Error)
			assert.GreaterOrEqual(t, permissionCount, int64(1), roleName+"/"+tierName)
		}
	}

	r1 := loadFullIdentitySubscription(t, db, s.ids.subscriptions["r1_premium_active"])
	assert.Equal(t, "active", r1.Status)
	assert.Equal(t, 2, r1.PaidInstallments)
	assert.Equal(t, s.ids.users["r1"], r1.UserID)
	assert.Equal(t, s.ids.tiers["corredor_premium"], r1.TierID)
	assertInstallmentStatuses(t, db, r1.ID, []string{"paid", "paid", "pending"})
	var r1Third dbs.Installment
	require.NoError(t, db.Where("subscription_id = ? AND installment_number = 3", r1.ID).Take(&r1Third).Error)
	require.NotNil(t, r1Third.DueDate)
	dueDate := r1Third.DueDate.In(gametime.Location)
	assert.Equal(t, s.now.Year(), dueDate.Year())
	assert.Equal(t, s.now.Month(), dueDate.Month())
	assert.Equal(t, s.now.Day(), dueDate.Day())

	r2 := loadFullIdentitySubscription(t, db, s.ids.subscriptions["r2_premium_debt"])
	assert.Equal(t, "active", r2.Status)
	assertInstallmentStatuses(t, db, r2.ID, []string{"paid", "pending"})
	var r2Debt dbs.Installment
	require.NoError(t, db.Where("subscription_id = ? AND installment_number = 2", r2.ID).Take(&r2Debt).Error)
	require.NotNil(t, r2Debt.BlockedDate)
	assert.True(t, r2Debt.BlockedDate.Before(s.now))

	r3Pending := loadFullIdentitySubscription(t, db, s.ids.subscriptions["r3_premium_pending"])
	assert.Equal(t, string(constants.SubscriptionStatusFirstPaymentPending), r3Pending.Status)
	assert.Equal(t, "first_payment_pending", r3Pending.Status)
	r3Active := loadFullIdentitySubscription(t, db, s.ids.subscriptions["r3_medium_active"])
	assert.Equal(t, "active", r3Active.Status)
	assertInstallmentStatuses(t, db, r3Pending.ID, []string{"pending"})
	assert.Equal(t, 900.0, r3Active.InitAmount)

	var setting dbs.PlatformSetting
	require.NoError(t, db.Where("key = ?", "marketplace_fee_percent").Take(&setting).Error)
	var fee any
	require.NoError(t, json.Unmarshal([]byte(setting.Value), &fee))
	assert.Equal(t, float64(5), fee)

	var tokenCount int64
	require.NoError(t, db.Model(&dbs.RefreshToken{}).Count(&tokenCount).Error)
	assert.Zero(t, tokenCount, "seedIdentity no debe emitir refresh tokens")

	assertFullIdentityAuthContract(t, db, s.ids.users)

	counts := fullIdentityTableCounts(t, db)
	assert.Equal(t, map[string]int64{
		"users": 18, "roles": 2, "permissions": 6, "tiers": 6, "tier_permissions": 12,
		"user_roles": 19, "user_role_tier_subscriptions": 4, "installments": 7,
		"platform_settings": 1,
	}, counts)
	firstIDs := copyFullIdentityIDs(s.ids)
	require.NoError(t, s.seedIdentity())
	assert.Equal(t, firstIDs.users, s.ids.users)
	assert.Equal(t, firstIDs.roles, s.ids.roles)
	assert.Equal(t, firstIDs.tiers, s.ids.tiers)
	assert.Equal(t, firstIDs.subscriptions, s.ids.subscriptions)
	assert.Equal(t, counts, fullIdentityTableCounts(t, db))
}

func TestFullSeederSeedIdentity_PreservesCanonicalRows(t *testing.T) {
	db := testutils.SetupTestDB(t)
	role := dbs.Role{Name: "entrenador", Description: "canonical role description"}
	require.NoError(t, db.Create(&role).Error)
	permission := dbs.Permission{Name: "crear_equipos", Description: "canonical permission description"}
	require.NoError(t, db.Create(&permission).Error)
	baseTier := dbs.Tier{
		Name: "base", Description: "canonical tier description", RoleID: role.ID,
		RoleName: role.Name, Hierarchy: 1,
	}
	require.NoError(t, db.Create(&baseTier).Error)
	unrelatedRole := dbs.Role{Name: "external_role", Description: "untouched"}
	require.NoError(t, db.Create(&unrelatedRole).Error)
	setting := dbs.PlatformSetting{Key: "marketplace_fee_percent", Value: "7.5"}
	require.NoError(t, db.Create(&setting).Error)

	s := newFullSeeder(db, options{})
	require.NoError(t, s.seedIdentity())

	var gotRole, gotUnrelated dbs.Role
	require.NoError(t, db.First(&gotRole, role.ID).Error)
	require.NoError(t, db.First(&gotUnrelated, unrelatedRole.ID).Error)
	assert.Equal(t, role.Description, gotRole.Description)
	assert.Equal(t, unrelatedRole.Description, gotUnrelated.Description)
	var gotPermission dbs.Permission
	require.NoError(t, db.First(&gotPermission, permission.ID).Error)
	assert.Equal(t, permission.Description, gotPermission.Description)
	var gotTier dbs.Tier
	require.NoError(t, db.First(&gotTier, baseTier.ID).Error)
	assert.Equal(t, baseTier.Description, gotTier.Description)
	assert.Equal(t, baseTier.PaymentRequired, gotTier.PaymentRequired)
	assert.Equal(t, baseTier.TierAmount, gotTier.TierAmount)
	var gotSetting dbs.PlatformSetting
	require.NoError(t, db.Where("key = ?", setting.Key).Take(&gotSetting).Error)
	assert.Equal(t, setting.Value, gotSetting.Value)
}

func TestFullSeederSeedIdentity_RejectsNonNumericMarketplaceFee(t *testing.T) {
	db := testutils.SetupTestDB(t)
	require.NoError(t, db.Create(&dbs.PlatformSetting{Key: "marketplace_fee_percent", Value: "null"}).Error)

	err := newFullSeeder(db, options{}).seedIdentity()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON numérico")
	var userCount int64
	require.NoError(t, db.Model(&dbs.User{}).Count(&userCount).Error)
	assert.Zero(t, userCount, "el rechazo debe revertir toda la transacción")
}

func TestFullSeederSeedIdentity_UsesExistingCanonicalTierAmount(t *testing.T) {
	db := testutils.SetupTestDB(t)
	role := dbs.Role{Name: "corredor"}
	require.NoError(t, db.Create(&role).Error)
	premium := dbs.Tier{
		Name: "premium", RoleID: role.ID, RoleName: role.Name,
		Hierarchy: 3, PaymentRequired: true, TierAmount: 4200,
	}
	require.NoError(t, db.Create(&premium).Error)
	s := newFullSeeder(db, options{})

	require.NoError(t, s.seedIdentity())

	var seededTier dbs.Tier
	require.NoError(t, db.First(&seededTier, premium.ID).Error)
	assert.Equal(t, 4200.0, seededTier.TierAmount)
	subscription := loadFullIdentitySubscription(t, db, s.ids.subscriptions["r1_premium_active"])
	assert.Equal(t, 4200.0, subscription.InitAmount)
	var thirdInstallment dbs.Installment
	require.NoError(t, db.Where("subscription_id = ? AND installment_number = 3", subscription.ID).Take(&thirdInstallment).Error)
	assert.Equal(t, 4200.0, thirdInstallment.Amount)
}

func fullIdentityUserByID(t *testing.T, db *gorm.DB, id int64) dbs.User {
	t.Helper()
	var user dbs.User
	require.NoError(t, db.First(&user, id).Error)
	return user
}

func fullIdentityRoleNames(t *testing.T, db *gorm.DB, userID int64) []string {
	t.Helper()
	var roles []string
	require.NoError(t, db.Table("user_roles").Select("roles.name").Joins("JOIN roles ON roles.id = user_roles.role_id").Where("user_roles.user_id = ? AND user_roles.deleted_at IS NULL", userID).Order("roles.name").Pluck("roles.name", &roles).Error)
	return roles
}

func loadFullIdentityTier(t *testing.T, db *gorm.DB, roleName, tierName string) dbs.Tier {
	t.Helper()
	var tier dbs.Tier
	require.NoError(t, db.Where("role_name = ? AND name = ? AND deleted_at IS NULL", roleName, tierName).Take(&tier).Error)
	return tier
}

func loadFullIdentitySubscription(t *testing.T, db *gorm.DB, id int64) dbs.UserRoleTierSubscription {
	t.Helper()
	var subscription dbs.UserRoleTierSubscription
	require.NoError(t, db.First(&subscription, id).Error)
	return subscription
}

func assertInstallmentStatuses(t *testing.T, db *gorm.DB, subscriptionID int64, want []string) {
	t.Helper()
	var rows []dbs.Installment
	require.NoError(t, db.Where("subscription_id = ?", subscriptionID).Order("installment_number").Find(&rows).Error)
	got := make([]string, len(rows))
	for i := range rows {
		got[i] = rows[i].Status
	}
	assert.Equal(t, want, got)
}

func assertFullIdentityAuthContract(t *testing.T, db *gorm.DB, users map[string]int64) {
	t.Helper()
	oldJWTSecret, oldIssuer, oldAudience := config.JWTSecret, config.JWTIssuer, config.JWTAudience
	oldGinMode := gin.Mode()
	config.JWTSecret, config.JWTIssuer, config.JWTAudience = "test-secret-key-for-full-seed", "test-issuer", "test-audience"
	t.Cleanup(func() {
		config.JWTSecret, config.JWTIssuer, config.JWTAudience = oldJWTSecret, oldIssuer, oldAudience
		gin.SetMode(oldGinMode)
	})
	gin.SetMode(gin.TestMode)

	authService := services.NewAuthService(
		daos.NewAuthDao(db), daos.NewUserRoleDao(db), daos.NewRoleDao(db), daos.NewRefreshTokenDao(db), nil,
	)
	for _, key := range []string{"trainer_1", "trainer_2", "r1", "r2", "r3"} {
		var user dbs.User
		require.NoError(t, db.First(&user, users[key]).Error)
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		_, err := authService.Login(ctx, user.Email, "Paceron.123")
		require.NoError(t, err, key)
	}
	for _, key := range []string{"blocked", "inactive"} {
		var user dbs.User
		require.NoError(t, db.First(&user, users[key]).Error)
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		_, err := authService.Login(ctx, user.Email, "Paceron.123")
		require.Error(t, err, key)
	}

	permissionsService := services.NewPermissionsQueryService(
		daos.NewUserDao(db), daos.NewUserRoleDao(db), daos.NewRoleDao(db), daos.NewTierDao(db),
		daos.NewTierPermissionDao(db), daos.NewPermissionDao(db), daos.NewTierSubscriptionDao(db),
	)
	permissionsController := controllers.NewPermissionsQueryController(permissionsService)
	for _, key := range []string{"trainer_1", "trainer_2", "r1", "r2", "r3"} {
		response := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(response)
		ctx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/auth/permissions?user_id=%d", users[key]), nil)
		permissionsController.GetUserPermissions(ctx)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var result services.PermissionsQueryResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
		require.NotEmpty(t, result.Roles, key)
		if key == "trainer_1" || key == "trainer_2" {
			assert.Len(t, result.Roles, 2)
		} else {
			assert.Len(t, result.Roles, 1)
		}
		for _, role := range result.Roles {
			assert.NotEmpty(t, role.Permissions, role.Name)
			if key == "trainer_1" || key == "trainer_2" {
				if role.Name == "corredor" {
					assert.ElementsMatch(t, []string{"consultar_entrenamientos"}, role.Permissions)
				} else if role.Name == "entrenador" {
					assert.ElementsMatch(t, []string{"crear_equipos"}, role.Permissions)
				}
			}
		}
		wantTier, wantPermissions := map[string]string{
			"r1": "premium", "r2": "premium", "r3": "medium",
		}[key], map[string][]string{
			"r1": {"consultar_entrenamientos", "registrar_entrenamientos", "ver_progreso"},
			"r2": {"consultar_entrenamientos", "registrar_entrenamientos", "ver_progreso"},
			"r3": {"consultar_entrenamientos", "registrar_entrenamientos"},
		}[key]
		if wantTier != "" {
			assert.Equal(t, wantTier, result.Roles[0].Tier)
			assert.ElementsMatch(t, wantPermissions, result.Roles[0].Permissions)
		}
	}
}

func copyFullIdentityIDs(ids fullSeedIDs) fullSeedIDs {
	copyMap := func(src map[string]int64) map[string]int64 {
		dst := make(map[string]int64, len(src))
		for key, value := range src {
			dst[key] = value
		}
		return dst
	}
	return fullSeedIDs{
		users: copyMap(ids.users), roles: copyMap(ids.roles), tiers: copyMap(ids.tiers),
		subscriptions: copyMap(ids.subscriptions),
	}
}

func fullIdentityTableCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	models := map[string]any{
		"users": &dbs.User{}, "roles": &dbs.Role{}, "permissions": &dbs.Permission{},
		"tiers": &dbs.Tier{}, "tier_permissions": &dbs.TierPermission{}, "user_roles": &dbs.UserRole{},
		"user_role_tier_subscriptions": &dbs.UserRoleTierSubscription{}, "installments": &dbs.Installment{},
		"platform_settings": &dbs.PlatformSetting{},
	}
	counts := make(map[string]int64, len(models))
	for table, model := range models {
		var count int64
		require.NoError(t, db.Model(model).Count(&count).Error)
		counts[table] = count
	}
	return counts
}
