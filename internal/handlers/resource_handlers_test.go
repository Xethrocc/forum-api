package handlers

// Integration tests for the forum and thread handlers. The shared test helpers
// (newHandlerMockDB, serveJSON, decodeJSONBody, assertContentTypeJSON,
// assertJSONError) live in auth_test.go in this package.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"forum-api/internal/models"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

// Queries issued by the forum and thread repositories.
const (
	insertForumQuery = `INSERT INTO forums (title, description, parent_forum_id, created_by) VALUES ($1, $2, $3, $4) RETURNING id, created_at`

	selectForumByIDQuery = "SELECT id, title, description, parent_forum_id, created_by, created_at FROM forums WHERE id = $1"

	insertThreadQuery = "INSERT INTO threads (forum_id, title, author_id) VALUES ($1, $2, $3) RETURNING id, created_at"
)

// serveJSONAs runs a handler with an authenticated user in the request
// context. Handlers take author and role from there, never from the body.
func serveJSONAs(user *models.User, handler http.HandlerFunc, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), "user", user))

	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func testUser(id int64) *models.User {
	return &models.User{ID: id, Username: "tester", Role: "user"}
}

func testAdmin(id int64) *models.User {
	return &models.User{ID: id, Username: "boss", Role: "admin"}
}

func testModerator(id int64) *models.User {
	return &models.User{ID: id, Username: "mod", Role: "moderator"}
}

// forumRows builds the result set of selectForumByIDQuery.
func forumRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "title", "description", "parent_forum_id", "created_by", "created_at"})
}

type forumPayload struct {
	ID            int64     `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	ParentForumID *int64    `json:"parent_forum_id"`
	CreatedAt     time.Time `json:"created_at"`
}

type threadPayload struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	ForumID   int64     `json:"forum_id"`
	AuthorID  int64     `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}

func TestForumHandler_CreateForum(t *testing.T) {
	t.Run("creates forum, inserts row and returns 201 with forum payload", func(t *testing.T) {
		mock := newHandlerMockDB(t)
		createdAt := time.Date(2024, 6, 1, 9, 30, 0, 0, time.UTC)

		// Elternforum wird geprüft: muss ein Hauptforum sein.
		mock.ExpectQuery(regexp.QuoteMeta(selectForumByIDQuery)).
			WithArgs(int64(3)).
			WillReturnRows(forumRows().
				AddRow(int64(3), "Tech", "All things tech", nil, nil, createdAt))

		// Database side-effect: the forum row must be inserted with the request
		// values, the parent forum id and the creating user.
		mock.ExpectQuery(regexp.QuoteMeta(insertForumQuery)).
			WithArgs("General", "General discussion", int64(3), int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(int64(11), createdAt))

		rec := serveJSONAs(testUser(42), ForumHandler, http.MethodPost, "/api/forums",
			`{"title":"General","description":"General discussion","parent_forum_id":3}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
		}
		assertContentTypeJSON(t, rec)

		var resp forumPayload
		decodeJSONBody(t, rec, &resp)

		if resp.ID != 11 {
			t.Errorf("id = %d, want 11", resp.ID)
		}
		if resp.Title != "General" {
			t.Errorf("title = %q, want %q", resp.Title, "General")
		}
		if resp.Description != "General discussion" {
			t.Errorf("description = %q, want %q", resp.Description, "General discussion")
		}
		if resp.ParentForumID == nil || *resp.ParentForumID != 3 {
			t.Errorf("parent_forum_id = %v, want 3", resp.ParentForumID)
		}
		if !resp.CreatedAt.Equal(createdAt) {
			t.Errorf("created_at = %v, want %v", resp.CreatedAt, createdAt)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("forum row was not inserted as expected: %v", err)
		}
	})

	t.Run("creates top-level forum when parent_forum_id is null", func(t *testing.T) {
		mock := newHandlerMockDB(t)
		createdAt := time.Date(2024, 6, 1, 9, 30, 0, 0, time.UTC)

		// Hauptforen anlegen darf nur admin.
		mock.ExpectQuery(regexp.QuoteMeta(insertForumQuery)).
			WithArgs("General", "General discussion", nil, int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(int64(11), createdAt))

		rec := serveJSONAs(testAdmin(42), ForumHandler, http.MethodPost, "/api/forums",
			`{"title":"General","description":"General discussion","parent_forum_id":null}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
		}

		var resp forumPayload
		decodeJSONBody(t, rec, &resp)

		if resp.ParentForumID != nil {
			t.Errorf("parent_forum_id = %v, want nil", *resp.ParentForumID)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("forum row was not inserted as expected: %v", err)
		}
	})

	t.Run("rejects missing title", func(t *testing.T) {
		_ = newHandlerMockDB(t) // no database access expected

		rec := serveJSONAs(testUser(42), ForumHandler, http.MethodPost, "/api/forums", `{"description":"no title"}`)

		assertJSONError(t, rec, http.StatusBadRequest, "Title is required")
	})

	t.Run("returns 500 when the insert fails", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(insertForumQuery)).
			WithArgs("General", "General discussion", nil, int64(42)).
			WillReturnError(errors.New("connection reset"))

		rec := serveJSONAs(testAdmin(42), ForumHandler, http.MethodPost, "/api/forums",
			`{"title":"General","description":"General discussion"}`)

		assertJSONError(t, rec, http.StatusInternalServerError, "Failed to create forum")
	})
}

func TestThreadHandler_CreateThread(t *testing.T) {
	t.Run("creates thread in forum, inserts row and returns 201 with thread payload", func(t *testing.T) {
		mock := newHandlerMockDB(t)
		createdAt := time.Date(2024, 7, 10, 18, 45, 0, 0, time.UTC)

		// Ziel-Forum wird geprüft: Themen nur in Unterforen (parent != NULL).
		mock.ExpectQuery(regexp.QuoteMeta(selectForumByIDQuery)).
			WithArgs(int64(7)).
			WillReturnRows(forumRows().
				AddRow(int64(7), "Subforum", "a subforum", int64(3), int64(42), createdAt))

		// Database side-effect: the thread row must be inserted into the forum
		// taken from the request path. The author is the token user (42), the
		// body author_id is ignored.
		mock.ExpectQuery(regexp.QuoteMeta(insertThreadQuery)).
			WithArgs(int64(7), "Welcome thread", int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(int64(99), createdAt))

		rec := serveJSONAs(testUser(42), ThreadHandler, http.MethodPost, "/api/forums/7/threads",
			`{"title":"Welcome thread","author_id":999}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusCreated, rec.Body.String())
		}
		assertContentTypeJSON(t, rec)

		var resp threadPayload
		decodeJSONBody(t, rec, &resp)

		if resp.ID != 99 {
			t.Errorf("id = %d, want 99", resp.ID)
		}
		if resp.Title != "Welcome thread" {
			t.Errorf("title = %q, want %q", resp.Title, "Welcome thread")
		}
		if resp.ForumID != 7 {
			t.Errorf("forum_id = %d, want 7", resp.ForumID)
		}
		if resp.AuthorID != 42 {
			t.Errorf("author_id = %d, want 42", resp.AuthorID)
		}
		if !resp.CreatedAt.Equal(createdAt) {
			t.Errorf("created_at = %v, want %v", resp.CreatedAt, createdAt)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("thread row was not inserted as expected: %v", err)
		}
	})

	t.Run("rejects missing title", func(t *testing.T) {
		_ = newHandlerMockDB(t) // no database access expected

		rec := serveJSONAs(testUser(42), ThreadHandler, http.MethodPost, "/api/forums/7/threads", `{"author_id":42}`)

		assertJSONError(t, rec, http.StatusBadRequest, "Title is required")
	})

	t.Run("rejects request without forum or thread id", func(t *testing.T) {
		_ = newHandlerMockDB(t) // no database access expected

		rec := serveJSONAs(testUser(42), ThreadHandler, http.MethodPost, "/api/threads", `{"title":"Welcome thread"}`)

		assertJSONError(t, rec, http.StatusBadRequest, "Forum ID required")
	})

	t.Run("returns 500 when the insert fails", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(selectForumByIDQuery)).
			WithArgs(int64(7)).
			WillReturnRows(forumRows().
				AddRow(int64(7), "Subforum", "a subforum", int64(3), int64(42), time.Now()))

		mock.ExpectQuery(regexp.QuoteMeta(insertThreadQuery)).
			WithArgs(int64(7), "Welcome thread", int64(42)).
			WillReturnError(errors.New("connection reset"))

		rec := serveJSONAs(testUser(42), ThreadHandler, http.MethodPost, "/api/forums/7/threads",
			`{"title":"Welcome thread","author_id":999}`)

		assertJSONError(t, rec, http.StatusInternalServerError, "Failed to create thread")
	})
}

// Queries issued by the search repository (see internal/repository/search_repo.go).
const (
	searchCountQuery = `
SELECT COUNT(*) FROM (
	SELECT id FROM threads WHERE search_vector @@ plainto_tsquery('english', $1)
	UNION ALL
	SELECT id FROM posts WHERE search_vector @@ plainto_tsquery('english', $1)
) matches`

	searchSelectQuery = `
SELECT id, type, content, created_at, relevance FROM (
	SELECT id, 'thread' AS type, title AS content, created_at,
		ts_rank(search_vector, plainto_tsquery('english', $1)) AS relevance
	FROM threads
	WHERE search_vector @@ plainto_tsquery('english', $1)
	UNION ALL
	SELECT id, 'post' AS type, content, created_at,
		ts_rank(search_vector, plainto_tsquery('english', $1)) AS relevance
	FROM posts
	WHERE search_vector @@ plainto_tsquery('english', $1)
) results
ORDER BY relevance DESC, created_at DESC, id DESC
LIMIT $2 OFFSET $3`
)

type searchResultPayload struct {
	ID             int64   `json:"id"`
	Type           string  `json:"type"`
	Content        string  `json:"content"`
	RelevanceScore float64 `json:"relevance_score"`
}

type searchResponse struct {
	Data []searchResultPayload  `json:"data"`
	Meta map[string]interface{} `json:"meta"`
}

func TestSearchHandler(t *testing.T) {
	t.Run("returns combined thread and post hits with pagination meta", func(t *testing.T) {
		mock := newHandlerMockDB(t)
		createdAt := time.Date(2024, 8, 5, 10, 15, 0, 0, time.UTC)

		// The total match count drives the pagination meta data.
		mock.ExpectQuery(regexp.QuoteMeta(searchCountQuery)).
			WithArgs("golang").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))

		// Page 1 with limit 2 returns the two most relevant matches.
		mock.ExpectQuery(regexp.QuoteMeta(searchSelectQuery)).
			WithArgs("golang", int64(2), int64(0)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "type", "content", "created_at", "relevance"}).
				AddRow(int64(7), "thread", "Learning Golang", createdAt, 0.6).
				AddRow(int64(12), "post", "golang channels are great", createdAt.Add(time.Minute), 0.3))

		rec := serveJSON(SearchHandler, http.MethodGet, "/api/search?q=golang&page=1&limit=2", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}
		assertContentTypeJSON(t, rec)

		var resp searchResponse
		decodeJSONBody(t, rec, &resp)

		if len(resp.Data) != 2 {
			t.Fatalf("len(data) = %d, want 2 (body %q)", len(resp.Data), rec.Body.String())
		}

		first := resp.Data[0]
		if first.ID != 7 || first.Type != "thread" || first.Content != "Learning Golang" {
			t.Errorf("data[0] = %+v, want the thread hit", first)
		}
		if first.RelevanceScore != 0.6 {
			t.Errorf("data[0].relevance_score = %v, want 0.6", first.RelevanceScore)
		}
		if resp.Data[1].ID != 12 || resp.Data[1].Type != "post" {
			t.Errorf("data[1] = %+v, want the post hit", resp.Data[1])
		}
		if resp.Data[1].RelevanceScore != 0.3 {
			t.Errorf("data[1].relevance_score = %v, want 0.3", resp.Data[1].RelevanceScore)
		}

		if got := resp.Meta["total"]; got != float64(3) {
			t.Errorf("meta.total = %v, want 3", got)
		}
		if got := resp.Meta["page"]; got != float64(1) {
			t.Errorf("meta.page = %v, want 1", got)
		}
		if got := resp.Meta["limit"]; got != float64(2) {
			t.Errorf("meta.limit = %v, want 2", got)
		}
		if got := resp.Meta["has_next"]; got != true {
			t.Errorf("meta.has_next = %v, want true", got)
		}
		if got := resp.Meta["query"]; got != "golang" {
			t.Errorf("meta.query = %v, want %q", got, "golang")
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("search was not executed as expected: %v", err)
		}
	})

	t.Run("paginates through the combined results", func(t *testing.T) {
		mock := newHandlerMockDB(t)
		createdAt := time.Date(2024, 8, 6, 9, 0, 0, 0, time.UTC)

		mock.ExpectQuery(regexp.QuoteMeta(searchCountQuery)).
			WithArgs("golang").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(5)))

		// Page 2 with limit 2 has to skip the first two matches.
		mock.ExpectQuery(regexp.QuoteMeta(searchSelectQuery)).
			WithArgs("golang", int64(2), int64(2)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "type", "content", "created_at", "relevance"}).
				AddRow(int64(21), "post", "another golang post", createdAt, 0.2))

		rec := serveJSON(SearchHandler, http.MethodGet, "/api/search?q=golang&page=2&limit=2", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp searchResponse
		decodeJSONBody(t, rec, &resp)

		if len(resp.Data) != 1 {
			t.Fatalf("len(data) = %d, want 1", len(resp.Data))
		}
		if got := resp.Meta["page"]; got != float64(2) {
			t.Errorf("meta.page = %v, want 2", got)
		}
		if got := resp.Meta["has_next"]; got != true {
			t.Errorf("meta.has_next = %v, want true (5 matches, page 2 of limit 2)", got)
		}
	})

	t.Run("uses default paging when page and limit are absent", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(searchCountQuery)).
			WithArgs("golang").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

		mock.ExpectQuery(regexp.QuoteMeta(searchSelectQuery)).
			WithArgs("golang", int64(10), int64(0)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "type", "content", "created_at", "relevance"}))

		rec := serveJSON(SearchHandler, http.MethodGet, "/api/search?q=golang", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body %q)", rec.Code, http.StatusOK, rec.Body.String())
		}

		var resp searchResponse
		decodeJSONBody(t, rec, &resp)

		if len(resp.Data) != 0 {
			t.Errorf("len(data) = %d, want 0", len(resp.Data))
		}
		if got := resp.Meta["total"]; got != float64(0) {
			t.Errorf("meta.total = %v, want 0", got)
		}
		if got := resp.Meta["page"]; got != float64(1) {
			t.Errorf("meta.page = %v, want 1", got)
		}
		if got := resp.Meta["limit"]; got != float64(10) {
			t.Errorf("meta.limit = %v, want 10", got)
		}
		if got := resp.Meta["has_next"]; got != false {
			t.Errorf("meta.has_next = %v, want false", got)
		}
	})

	t.Run("rejects missing query parameter", func(t *testing.T) {
		_ = newHandlerMockDB(t) // no database access expected

		rec := serveJSON(SearchHandler, http.MethodGet, "/api/search?page=1&limit=10", "")

		assertJSONError(t, rec, http.StatusBadRequest, "Missing query parameter")
	})

	t.Run("returns 500 when the search fails", func(t *testing.T) {
		mock := newHandlerMockDB(t)

		mock.ExpectQuery(regexp.QuoteMeta(searchCountQuery)).
			WithArgs("golang").
			WillReturnError(errors.New("connection reset"))

		rec := serveJSON(SearchHandler, http.MethodGet, "/api/search?q=golang", "")

		assertJSONError(t, rec, http.StatusInternalServerError, "Failed to search")
	})
}
