package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewPaymentDao(t *testing.T) {
	dao := NewPaymentDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestPaymentDao_ImplementsInterface(t *testing.T) {
	dao := NewPaymentDao(&gorm.DB{})
	var iface PaymentDaoInterface = dao
	_ = iface
}

func TestPaymentDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewPaymentDao(&gorm.DB{})
	})
}

func TestPaymentDao_Create_AndFindByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentDao(db)

	payment := &dbs.Payment{
		Concept:     "order",
		Description: "Test payment",
		Amount:      1000.50,
		CurrencyID:  "ARS",
		Status:      "pending",
		PayerEmail:  "test@example.com",
	}

	err := dao.Create(nil, payment)
	require.NoError(t, err)
	assert.NotZero(t, payment.ID)

	found, err := dao.FindByID(nil, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "order", found.Concept)
	assert.Equal(t, 1000.50, found.Amount)
	assert.Equal(t, "pending", found.Status)
}

func TestPaymentDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestPaymentDao_FindByPaymentID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentDao(db)

	payment := &dbs.Payment{
		Concept:     "subscription",
		Description: "Monthly sub",
		Amount:      500,
		CurrencyID:  "ARS",
		Status:      "approved",
		PaymentID:   "12345678",
		PayerEmail:  "sub@example.com",
	}
	require.NoError(t, dao.Create(nil, payment))

	found, err := dao.FindByPaymentID(nil, "12345678")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "subscription", found.Concept)
}

func TestPaymentDao_FindByPaymentID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentDao(db)

	found, err := dao.FindByPaymentID(nil, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestPaymentDao_FindByExternalReference_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentDao(db)

	extRef := "ext-ref-123"
	payment := &dbs.Payment{
		Concept:     "session",
		Description: "PT session",
		Amount:      2000,
		CurrencyID:  "ARS",
		Status:      "pending",
		ExternalRef: &extRef,
		PayerEmail:  "pt@example.com",
	}
	require.NoError(t, dao.Create(nil, payment))

	found, err := dao.FindByExternalReference(nil, "ext-ref-123")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "session", found.Concept)
}

func TestPaymentDao_FindByExternalReference_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentDao(db)

	found, err := dao.FindByExternalReference(nil, "no-such-ref")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestPaymentDao_UpdateStatus(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentDao(db)

	payment := &dbs.Payment{
		Concept:     "order",
		Description: "Status update test",
		Amount:      750,
		CurrencyID:  "ARS",
		Status:      "pending",
		PayerEmail:  "status@example.com",
	}
	require.NoError(t, dao.Create(nil, payment))

	err := dao.UpdateStatus(nil, payment.ID, "approved", "accredited")
	require.NoError(t, err)

	found, err := dao.FindByID(nil, payment.ID)
	require.NoError(t, err)
	assert.Equal(t, "approved", found.Status)
	assert.Equal(t, "accredited", found.StatusDetail)
}

func TestPaymentDao_UpdateStatus_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentDao(db)

	err := dao.UpdateStatus(nil, 999999, "approved", "accredited")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payment not found")
}

func TestPaymentDao_UpdateRawResponse(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewPaymentDao(db)

	payment := &dbs.Payment{
		Concept:     "order",
		Description: "Raw update test",
		Amount:      300,
		CurrencyID:  "ARS",
		Status:      "pending",
		PayerEmail:  "raw@example.com",
	}
	require.NoError(t, dao.Create(nil, payment))

	err := dao.UpdateRawResponse(nil, payment.ID, `{"id":123}`)
	require.NoError(t, err)

	found, err := dao.FindByID(nil, payment.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.NotNil(t, found.RawResponse)
}
