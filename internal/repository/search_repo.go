package repository

import (
	"context"
	"time"

	"forum-api/internal/models"
)

type SearchRepository struct{}

// searchCountQuery counts the combined matches across threads and posts.
const searchCountQuery = `
SELECT COUNT(*) FROM (
	SELECT id FROM threads WHERE search_vector @@ plainto_tsquery('english', $1)
	UNION ALL
	SELECT id FROM posts WHERE search_vector @@ plainto_tsquery('english', $1)
) matches`

// searchSelectQuery returns the combined matches ordered by relevance. The
// thread title is used as the searchable content for thread hits, the post
// body for post hits. plainto_tsquery turns the raw user input into a safe
// tsquery (no user-supplied tsquery syntax is interpreted).
const searchSelectQuery = `
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

// Search runs a full-text search over threads and posts using their tsvector
// columns and returns the combined, relevance-ordered results for the given
// page together with the total number of matches. Page numbering is 1-based,
// matching the pagination used by the other paginated repository methods.
func (sr SearchRepository) Search(query string, page, limit int) ([]models.SearchResult, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	// Get total count
	var total int64
	err := DB.QueryRowContext(ctx, searchCountQuery, query).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	rows, err := DB.QueryContext(ctx, searchSelectQuery, query, limit, offset)
	if err != nil {
		return nil, total, err
	}
	defer rows.Close()

	results := make([]models.SearchResult, 0)
	for rows.Next() {
		var res models.SearchResult
		if err := rows.Scan(&res.ID, &res.Type, &res.Content, &res.CreatedAt, &res.RelevanceScore); err != nil {
			return nil, total, err
		}
		results = append(results, res)
	}

	if err := rows.Err(); err != nil {
		return nil, total, err
	}

	return results, total, nil
}
