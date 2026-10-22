package dao

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"mall/model"
)

// ─── helpers ────────────────────────────────────────────────────────────────────

func initTestUserDB(t *testing.T) *GormUserDB {
	t.Helper()
	db := newTestDB(t, &model.User{})
	return &GormUserDB{DB: db}
}

func seedUser(t *testing.T, db *gorm.DB, opts ...func(*model.User)) *model.User {
	t.Helper()
	u := &model.User{
		Username: "user-" + randomSuffix(),
		Password: "hashed-password",
		Role:     model.RoleUser,
	}
	for _, opt := range opts {
		opt(u)
	}
	require.NoError(t, db.Create(u).Error)
	return u
}

// ─── FindByIDForUpdate ──────────────────────────────────────────────────────────

func TestGormUserDB_FindByIDForUpdate(t *testing.T) {
	g := initTestUserDB(t)
	u := seedUser(t, g.DB)

	got, err := g.FindByIDForUpdate(u.ID)
	assert.NoError(t, err)
	assert.Equal(t, u.Username, got.Username)
}

func TestGormUserDB_FindByIDForUpdate_NotFound(t *testing.T) {
	g := initTestUserDB(t)
	_, err := g.FindByIDForUpdate(9999)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── Transaction ────────────────────────────────────────────────────────────────

func TestGormUserDB_Transaction(t *testing.T) {
	g := initTestUserDB(t)

	err := g.Transaction(func(tx UserDB) error {
		u := &model.User{Username: "tx-user-" + randomSuffix(), Password: "pw", Role: model.RoleUser}
		return tx.Create(u)
	})
	assert.NoError(t, err)
}

func TestGormUserDB_Transaction_Rollback(t *testing.T) {
	g := initTestUserDB(t)

	testErr := assert.AnError
	err := g.Transaction(func(tx UserDB) error {
		u := &model.User{Username: "rollback-" + randomSuffix(), Password: "pw", Role: model.RoleUser}
		require.NoError(t, tx.Create(u))
		return testErr
	})
	assert.ErrorIs(t, err, testErr)

	// user was rolled back
	_, err = g.FindById(1) // no user with ID=1 was committed
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// ─── Create / Update / GetByUsername ───────────────────────────────────────────

func TestGormUserDB_Create(t *testing.T) {
	g := initTestUserDB(t)
	u := &model.User{Username: "new-user", Password: "pw", Role: model.RoleUser}
	assert.NoError(t, g.Create(u))
	assert.NotZero(t, u.ID)
}

func TestGormUserDB_FindById(t *testing.T) {
	g := initTestUserDB(t)
	u := seedUser(t, g.DB)

	got, err := g.FindById(u.ID)
	assert.NoError(t, err)
	assert.Equal(t, u.Username, got.Username)
}

func TestGormUserDB_GetByUsername(t *testing.T) {
	g := initTestUserDB(t)
	u := seedUser(t, g.DB, func(u *model.User) { u.Username = "alice" })

	got, err := g.GetByUsername("alice")
	assert.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)
}

func TestGormUserDB_GetByUsername_NotFound(t *testing.T) {
	g := initTestUserDB(t)
	_, err := g.GetByUsername("nobody")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGormUserDB_Update(t *testing.T) {
	g := initTestUserDB(t)
	u := seedUser(t, g.DB)

	u.Nickname = "new-nick"
	assert.NoError(t, g.Update(u))

	got, _ := g.FindById(u.ID)
	assert.Equal(t, "new-nick", got.Nickname)
}
