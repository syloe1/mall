package dao

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// randomSuffix returns a short random string for generating unique test values.
func randomSuffix() string {
	return fmt.Sprintf("%06d", rng.Intn(1000000))
}

// newTestDB creates an in-memory SQLite database and auto-migrates the given models.
// Use this as the base for all DAO unit tests.
func newTestDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	for _, m := range models {
		require.NoError(t, db.AutoMigrate(m))
	}
	return db
}
