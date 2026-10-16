package repository

import (
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

const forumInsertQuery = "INSERT INTO forums (title, description, parent_forum_id, created_by) VALUES ($1, $2, $3, $4) RETURNING id, created_at"

const forumSelectAllQuery = "SELECT id, title, description, parent_forum_id, created_by, created_at FROM forums ORDER BY id"

func TestForumRepository_Create(t *testing.T) {
	t.Run("creates top level forum without parent", func(t *testing.T) {
		mock := newMockDB(t)
		createdAt := time.Date(2024, 3, 10, 9, 0, 0, 0, time.UTC)

		mock.ExpectQuery(regexp.QuoteMeta(forumInsertQuery)).
			WithArgs("General", "General discussion", nil, nil).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(int64(1), createdAt))

		forum, err := ForumRepository{}.Create("General", "General discussion", nil, nil)
		if err != nil {
			t.Fatalf("Create() error = %v, want nil", err)
		}
		if forum == nil {
			t.Fatal("Create() returned nil forum")
		}

		if forum.ID != 1 {
			t.Errorf("forum.ID = %d, want 1", forum.ID)
		}
		if forum.Title != "General" {
			t.Errorf("forum.Title = %q, want %q", forum.Title, "General")
		}
		if forum.Description != "General discussion" {
			t.Errorf("forum.Description = %q, want %q", forum.Description, "General discussion")
		}
		if forum.ParentForumID != nil {
			t.Errorf("forum.ParentForumID = %v, want nil", *forum.ParentForumID)
		}
		if !forum.CreatedAt.Equal(createdAt) {
			t.Errorf("forum.CreatedAt = %v, want %v", forum.CreatedAt, createdAt)
		}
	})

	t.Run("creates subforum with parent id", func(t *testing.T) {
		mock := newMockDB(t)
		createdAt := time.Date(2024, 3, 11, 10, 30, 0, 0, time.UTC)
		parentID := int64(1)
		createdByID := int64(42)

		mock.ExpectQuery(regexp.QuoteMeta(forumInsertQuery)).
			WithArgs("Go", "Everything about Go", int64(1), int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(int64(2), createdAt))

		forum, err := ForumRepository{}.Create("Go", "Everything about Go", &parentID, &createdByID)
		if err != nil {
			t.Fatalf("Create() error = %v, want nil", err)
		}
		if forum.ParentForumID == nil {
			t.Fatal("forum.ParentForumID = nil, want 1")
		}
		if *forum.ParentForumID != 1 {
			t.Errorf("*forum.ParentForumID = %d, want 1", *forum.ParentForumID)
		}
	})

	t.Run("returns error when insert fails", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(forumInsertQuery)).
			WithArgs("General", "General discussion", nil, nil).
			WillReturnError(errors.New("pq: insert failed"))

		forum, err := ForumRepository{}.Create("General", "General discussion", nil, nil)
		if err == nil {
			t.Fatal("Create() error = nil, want error")
		}
		if forum != nil {
			t.Errorf("Create() forum = %+v, want nil", forum)
		}
	})
}

func TestForumRepository_GetAll(t *testing.T) {
	t.Run("returns all forums ordered by id", func(t *testing.T) {
		mock := newMockDB(t)
		createdAt := time.Date(2024, 3, 10, 9, 0, 0, 0, time.UTC)

		mock.ExpectQuery(regexp.QuoteMeta(forumSelectAllQuery)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "parent_forum_id", "created_by", "created_at"}).
				AddRow(int64(1), "General", "General discussion", nil, nil, createdAt).
				AddRow(int64(2), "Go", "Everything about Go", int64(1), nil, createdAt.Add(time.Hour)).
				AddRow(int64(3), "Offtopic", "Anything else", int64(1), nil, createdAt.Add(2*time.Hour)))

		forums, err := ForumRepository{}.GetAll()
		if err != nil {
			t.Fatalf("GetAll() error = %v, want nil", err)
		}
		if len(forums) != 3 {
			t.Fatalf("len(GetAll()) = %d, want 3", len(forums))
		}

		if forums[0].ID != 1 || forums[1].ID != 2 || forums[2].ID != 3 {
			t.Errorf("GetAll() ids = [%d %d %d], want [1 2 3]", forums[0].ID, forums[1].ID, forums[2].ID)
		}
		if forums[0].Title != "General" {
			t.Errorf("forums[0].Title = %q, want %q", forums[0].Title, "General")
		}
		if forums[0].ParentForumID != nil {
			t.Errorf("forums[0].ParentForumID = %v, want nil", *forums[0].ParentForumID)
		}
		if forums[1].ParentForumID == nil || *forums[1].ParentForumID != 1 {
			t.Errorf("forums[1].ParentForumID = %v, want 1", forums[1].ParentForumID)
		}
		if forums[2].Description != "Anything else" {
			t.Errorf("forums[2].Description = %q, want %q", forums[2].Description, "Anything else")
		}
		if !forums[2].CreatedAt.Equal(createdAt.Add(2 * time.Hour)) {
			t.Errorf("forums[2].CreatedAt = %v, want %v", forums[2].CreatedAt, createdAt.Add(2*time.Hour))
		}
	})

	t.Run("returns empty slice when no forums exist", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(forumSelectAllQuery)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "parent_forum_id", "created_by", "created_at"}))

		forums, err := ForumRepository{}.GetAll()
		if err != nil {
			t.Fatalf("GetAll() error = %v, want nil", err)
		}
		if len(forums) != 0 {
			t.Errorf("len(GetAll()) = %d, want 0", len(forums))
		}
	})

	t.Run("returns error when query fails", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(forumSelectAllQuery)).
			WillReturnError(errors.New("connection reset"))

		forums, err := ForumRepository{}.GetAll()
		if err == nil {
			t.Fatal("GetAll() error = nil, want error")
		}
		if forums != nil {
			t.Errorf("GetAll() forums = %+v, want nil", forums)
		}
	})

	t.Run("returns error when row scanning fails", func(t *testing.T) {
		mock := newMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(forumSelectAllQuery)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "title", "description", "parent_forum_id", "created_by", "created_at"}).
				AddRow(int64(1), "General", "General discussion", "not-a-number", nil, time.Now()).
				RowError(0, errors.New("scan error")))

		forums, err := ForumRepository{}.GetAll()
		if err == nil {
			t.Fatal("GetAll() error = nil, want error")
		}
		if forums != nil {
			t.Errorf("GetAll() forums = %+v, want nil", forums)
		}
	})
}
