package handlers

// Rechte-Modell aus FORUM_STRUKTUR.md: statische Hauptforen (nur admin),
// Unterforen mit Besitzer (Ersteller oder mod/admin darf ändern/löschen),
// Löschen nur leerer Foren, Themen nur in Unterforen.

import (
	"net/http"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestForumHandler_CategoryRules(t *testing.T) {
	t.Run("rejects top-level creation for non-admin", func(t *testing.T) {
		_ = newHandlerMockDB(t) // no database access expected

		rec := serveJSONAs(testUser(42), ForumHandler, http.MethodPost, "/api/forums",
			`{"title":"General","description":"d"}`)

		assertJSONError(t, rec, http.StatusForbidden, "Only admins can create top-level forums")
	})

	t.Run("rejects subforum under a subforum", func(t *testing.T) {
		mock := newHandlerMockDB(t)
		createdAt := time.Date(2024, 6, 1, 9, 30, 0, 0, time.UTC)

		// Parent 5 ist selbst ein Unterforum (parent 3).
		mock.ExpectQuery(regexp.QuoteMeta(selectForumByIDQuery)).
			WithArgs(int64(5)).
			WillReturnRows(forumRows().
				AddRow(int64(5), "Sub", "d", int64(3), int64(42), createdAt))

		rec := serveJSONAs(testUser(42), ForumHandler, http.MethodPost, "/api/forums",
			`{"title":"Subsub","parent_forum_id":5}`)

		assertJSONError(t, rec, http.StatusBadRequest, "Subforums can only be created under top-level forums")
	})

	t.Run("rejects request without user", func(t *testing.T) {
		_ = newHandlerMockDB(t) // no database access expected

		rec := serveJSON(ForumHandler, http.MethodPost, "/api/forums", `{"title":"x"}`)

		assertJSONError(t, rec, http.StatusUnauthorized, "Unauthorized")
	})
}

func TestForumHandler_DeleteForum(t *testing.T) {
	createdAt := time.Date(2024, 6, 1, 9, 30, 0, 0, time.UTC)

	t.Run("creator deletes empty subforum", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(selectForumByIDQuery)).
			WithArgs(int64(7)).
			WillReturnRows(forumRows().
				AddRow(int64(7), "Mine", "d", int64(3), int64(42), createdAt))

		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		mock.ExpectExec("DELETE FROM forums WHERE id").
			WithArgs(int64(7)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		rec := serveJSONAs(testUser(42), ForumHandler, http.MethodDelete, "/api/forums/7", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects delete of non-empty forum", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(selectForumByIDQuery)).
			WithArgs(int64(7)).
			WillReturnRows(forumRows().
				AddRow(int64(7), "Mine", "d", int64(3), int64(42), createdAt))

		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		rec := serveJSONAs(testUser(42), ForumHandler, http.MethodDelete, "/api/forums/7", "")

		assertJSONError(t, rec, http.StatusConflict, "Forum is not empty")
	})

	t.Run("rejects delete by non-creator", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(selectForumByIDQuery)).
			WithArgs(int64(7)).
			WillReturnRows(forumRows().
				AddRow(int64(7), "Mine", "d", int64(3), int64(99), createdAt))

		rec := serveJSONAs(testUser(42), ForumHandler, http.MethodDelete, "/api/forums/7", "")

		assertJSONError(t, rec, http.StatusForbidden, "Forbidden")
	})

	t.Run("rejects delete of top-level forum by non-admin", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(selectForumByIDQuery)).
			WithArgs(int64(3)).
			WillReturnRows(forumRows().
				AddRow(int64(3), "Kategorie", "d", nil, nil, createdAt))

		rec := serveJSONAs(testUser(42), ForumHandler, http.MethodDelete, "/api/forums/3", "")

		assertJSONError(t, rec, http.StatusForbidden, "Forbidden")
	})

	t.Run("moderator deletes subforum of another user", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(selectForumByIDQuery)).
			WithArgs(int64(7)).
			WillReturnRows(forumRows().
				AddRow(int64(7), "Theirs", "d", int64(3), int64(99), createdAt))

		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(int64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		mock.ExpectExec("DELETE FROM forums WHERE id").
			WithArgs(int64(7)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		rec := serveJSONAs(testModerator(42), ForumHandler, http.MethodDelete, "/api/forums/7", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
		}
	})
}

func TestThreadHandler_CreateOnlyInSubforums(t *testing.T) {
	t.Run("rejects threads in top-level forums", func(t *testing.T) {
		mock := newHandlerMockDB(t)
		createdAt := time.Date(2024, 6, 1, 9, 30, 0, 0, time.UTC)

		mock.ExpectQuery(regexp.QuoteMeta(selectForumByIDQuery)).
			WithArgs(int64(3)).
			WillReturnRows(forumRows().
				AddRow(int64(3), "Kategorie", "d", nil, nil, createdAt))

		rec := serveJSONAs(testUser(42), ThreadHandler, http.MethodPost, "/api/forums/3/threads",
			`{"title":"Geht nicht"}`)

		assertJSONError(t, rec, http.StatusBadRequest, "Threads can only be created in subforums")
	})
}

func TestThreadHandler_DeleteThread(t *testing.T) {
	createdAt := time.Date(2024, 7, 10, 18, 45, 0, 0, time.UTC)

	t.Run("creator deletes thread with posts", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery("SELECT id, forum_id, title, author_id, created_at, last_post_at FROM threads WHERE id").
			WithArgs(int64(9)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "forum_id", "title", "author_id", "created_at", "last_post_at"}).
				AddRow(int64(9), int64(7), "Mein Thema", int64(42), createdAt, createdAt))

		mock.ExpectBegin()
		mock.ExpectExec("DELETE FROM posts WHERE thread_id").
			WithArgs(int64(9)).
			WillReturnResult(sqlmock.NewResult(0, 2))
		mock.ExpectExec("DELETE FROM threads WHERE id").
			WithArgs(int64(9)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		rec := serveJSONAs(testUser(42), ThreadHandler, http.MethodDelete, "/api/threads/9", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects delete by non-creator", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery("SELECT id, forum_id, title, author_id, created_at, last_post_at FROM threads WHERE id").
			WithArgs(int64(9)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "forum_id", "title", "author_id", "created_at", "last_post_at"}).
				AddRow(int64(9), int64(7), "Fremdes Thema", int64(99), createdAt, createdAt))

		rec := serveJSONAs(testUser(42), ThreadHandler, http.MethodDelete, "/api/threads/9", "")

		assertJSONError(t, rec, http.StatusForbidden, "Forbidden")
	})
}

func TestPostHandler_DeletePost(t *testing.T) {
	createdAt := time.Date(2024, 7, 10, 18, 45, 0, 0, time.UTC)

	t.Run("creator deletes own post", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery("SELECT id, thread_id, author_id, content, created_at FROM posts WHERE id").
			WithArgs(int64(5)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "thread_id", "author_id", "content", "created_at"}).
				AddRow(int64(5), int64(9), int64(42), "mein Text", createdAt))

		mock.ExpectExec("DELETE FROM posts WHERE id").
			WithArgs(int64(5)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		rec := serveJSONAs(testUser(42), PostHandler, http.MethodDelete, "/api/threads/9/posts/5", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
		}
	})

	t.Run("rejects delete by non-creator", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery("SELECT id, thread_id, author_id, content, created_at FROM posts WHERE id").
			WithArgs(int64(5)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "thread_id", "author_id", "content", "created_at"}).
				AddRow(int64(5), int64(9), int64(99), "fremder Text", createdAt))

		rec := serveJSONAs(testUser(42), PostHandler, http.MethodDelete, "/api/threads/9/posts/5", "")

		assertJSONError(t, rec, http.StatusForbidden, "Forbidden")
	})
}
