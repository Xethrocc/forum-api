package repository

import (
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestSearchRepository_Search(t *testing.T) {
	t.Run("returns combined thread and post matches ordered by relevance", func(t *testing.T) {
		mock := newMockDB(t)
		createdAt := time.Date(2024, 5, 2, 12, 0, 0, 0, time.UTC)

		mock.ExpectQuery(regexp.QuoteMeta(searchCountQuery)).
			WithArgs("golang").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))

		mock.ExpectQuery(regexp.QuoteMeta(searchSelectQuery)).
			WithArgs("golang", int64(10), int64(0)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "type", "content", "created_at", "relevance"}).
				AddRow(int64(7), "thread", "Learning Golang", createdAt, 0.6).
				AddRow(int64(12), "post", "golang channels are great", createdAt.Add(time.Minute), 0.3).
				AddRow(int64(3), "post", "more about golang", createdAt.Add(2*time.Minute), 0.1))

		results, total, err := SearchRepository{}.Search("golang", 1, 10)
		if err != nil {
			t.Fatalf("Search() error = %v, want nil", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(results) != 3 {
			t.Fatalf("len(results) = %d, want 3", len(results))
		}

		first := results[0]
		if first.ID != 7 || first.Type != "thread" || first.Content != "Learning Golang" {
			t.Errorf("results[0] = %+v, want the thread hit", first)
		}
		if first.RelevanceScore != 0.6 {
			t.Errorf("results[0].RelevanceScore = %v, want 0.6", first.RelevanceScore)
		}
		if !first.CreatedAt.Equal(createdAt) {
			t.Errorf("results[0].CreatedAt = %v, want %v", first.CreatedAt, createdAt)
		}
		if results[1].Type != "post" || results[1].ID != 12 {
			t.Errorf("results[1] = %+v, want the post hit", results[1])
		}
		if results[2].RelevanceScore != 0.1 {
			t.Errorf("results[2].RelevanceScore = %v, want 0.1", results[2].RelevanceScore)
		}
	})

	t.Run("translates page and limit into a SQL offset", func(t *testing.T) {
		mock := newMockDB(t)
		createdAt := time.Date(2024, 5, 3, 8, 30, 0, 0, time.UTC)

		mock.ExpectQuery(regexp.QuoteMeta(searchCountQuery)).
			WithArgs("golang").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(25)))

		// Page 3 with a limit of 5 has to skip the first 10 matches.
		mock.ExpectQuery(regexp.QuoteMeta(searchSelectQuery)).
			WithArgs("golang", int64(5), int64(10)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "type", "content", "created_at", "relevance"}).
				AddRow(int64(21), "post", "golang tip", createdAt, 0.2))

		results, total, err := SearchRepository{}.Search("golang", 3, 5)
		if err != nil {
			t.Fatalf("Search() error = %v, want nil", err)
		}
		if total != 25 {
			t.Errorf("total = %d, want 25", total)
		}
		if len(results) != 1 {
			t.Fatalf("len(results) = %d, want 1", len(results))
		}
	})

	t.Run("falls back to page 1 and limit 10 for invalid paging", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(searchCountQuery)).
			WithArgs("golang").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

		mock.ExpectQuery(regexp.QuoteMeta(searchSelectQuery)).
			WithArgs("golang", int64(10), int64(0)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "type", "content", "created_at", "relevance"}))

		results, total, err := SearchRepository{}.Search("golang", 0, -1)
		if err != nil {
			t.Fatalf("Search() error = %v, want nil", err)
		}
		if total != 0 {
			t.Errorf("total = %d, want 0", total)
		}
		if len(results) != 0 {
			t.Errorf("len(results) = %d, want 0", len(results))
		}
		if results == nil {
			t.Error("results = nil, want empty slice so the API encodes []")
		}
	})

	t.Run("returns error when the count query fails", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(searchCountQuery)).
			WithArgs("golang").
			WillReturnError(errors.New("connection reset"))

		results, total, err := SearchRepository{}.Search("golang", 1, 10)
		if err == nil {
			t.Fatal("Search() error = nil, want error")
		}
		if results != nil {
			t.Errorf("Search() results = %+v, want nil", results)
		}
		if total != 0 {
			t.Errorf("total = %d, want 0", total)
		}
	})

	t.Run("returns error when the result query fails", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(searchCountQuery)).
			WithArgs("golang").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))

		mock.ExpectQuery(regexp.QuoteMeta(searchSelectQuery)).
			WithArgs("golang", int64(10), int64(0)).
			WillReturnError(errors.New("connection reset"))

		results, _, err := SearchRepository{}.Search("golang", 1, 10)
		if err == nil {
			t.Fatal("Search() error = nil, want error")
		}
		if results != nil {
			t.Errorf("Search() results = %+v, want nil", results)
		}
	})

	t.Run("returns error when reading rows fails", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(searchCountQuery)).
			WithArgs("golang").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

		mock.ExpectQuery(regexp.QuoteMeta(searchSelectQuery)).
			WithArgs("golang", int64(10), int64(0)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "type", "content", "created_at", "relevance"}).
				AddRow(int64(1), "thread", "Learning Golang", time.Now(), 0.5).
				RowError(0, errors.New("read error")))

		results, _, err := SearchRepository{}.Search("golang", 1, 10)
		if err == nil {
			t.Fatal("Search() error = nil, want error")
		}
		if results != nil {
			t.Errorf("Search() results = %+v, want nil", results)
		}
	})

	t.Run("returns error when a result row cannot be scanned", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(searchCountQuery)).
			WithArgs("golang").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

		mock.ExpectQuery(regexp.QuoteMeta(searchSelectQuery)).
			WithArgs("golang", int64(10), int64(0)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "type", "content", "created_at", "relevance"}).
				AddRow("not-a-number", "thread", "Learning Golang", time.Now(), 0.5))

		results, _, err := SearchRepository{}.Search("golang", 1, 10)
		if err == nil {
			t.Fatal("Search() error = nil, want error")
		}
		if results != nil {
			t.Errorf("Search() results = %+v, want nil", results)
		}
	})
}
