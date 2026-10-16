package repository

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// newMockDB creates a sqlmock database, swaps it in for the package level DB
// global and restores the previous value when the test finishes. It also fails
// the test if not all expected queries were executed.
func newMockDB(t *testing.T) sqlmock.Sqlmock {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock database: %v", err)
	}

	oldDB := DB
	DB = db

	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled mock expectations: %v", err)
		}
		db.Close()
		DB = oldDB
	})

	return mock
}
