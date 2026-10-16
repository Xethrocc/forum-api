package repository

import (
	"forum-api/internal/models"
	"context"
	"time"
)

type ThreadRepository struct{}

func (tr ThreadRepository) Create(forumID int64, title string, authorID int64) (*models.Thread, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var thread models.Thread
	err := DB.QueryRowContext(ctx,
		"INSERT INTO threads (forum_id, title, author_id) VALUES ($1, $2, $3) RETURNING id, created_at",
		forumID, title, authorID,
	).Scan(&thread.ID, &thread.CreatedAt)

	if err != nil {
		return nil, err
	}

	thread.ForumID = forumID
	thread.Title = title
	thread.AuthorID = authorID

	return &thread, nil
}

func (tr ThreadRepository) GetAllByForum(forumID int64) ([]models.Thread, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DB.QueryContext(ctx,
		"SELECT id, forum_id, title, author_id, created_at, last_post_at FROM threads WHERE forum_id = $1 ORDER BY last_post_at DESC",
		forumID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []models.Thread
	for rows.Next() {
		var t models.Thread
		if err := rows.Scan(&t.ID, &t.ForumID, &t.Title, &t.AuthorID, &t.CreatedAt, &t.LastPostAt); err != nil {
			return nil, err
		}
		threads = append(threads, t)
	}

	return threads, rows.Err()
}

func (tr ThreadRepository) GetAllByForumPaginated(forumID int64, offset, limit int) ([]models.Thread, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get total count
	var total int64
	err := DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM threads WHERE forum_id = $1", forumID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	rows, err := DB.QueryContext(ctx,
		"SELECT id, forum_id, title, author_id, created_at, last_post_at FROM threads WHERE forum_id = $1 ORDER BY last_post_at DESC LIMIT $2 OFFSET $3",
		forumID, limit, offset,
	)
	if err != nil {
		return nil, total, err
	}
	defer rows.Close()

	var threads []models.Thread
	for rows.Next() {
		var t models.Thread
		if err := rows.Scan(&t.ID, &t.ForumID, &t.Title, &t.AuthorID, &t.CreatedAt, &t.LastPostAt); err != nil {
			return nil, total, err
		}
		threads = append(threads, t)
	}

	return threads, total, rows.Err()
}

func (tr ThreadRepository) GetByID(id int64) (*models.Thread, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var thread models.Thread
	err := DB.QueryRowContext(ctx,
		"SELECT id, forum_id, title, author_id, created_at, last_post_at FROM threads WHERE id = $1",
		id,
	).Scan(&thread.ID, &thread.ForumID, &thread.Title, &thread.AuthorID, &thread.CreatedAt, &thread.LastPostAt)

	if err != nil {
		return nil, err
	}

	return &thread, nil
}

func (tr ThreadRepository) UpdateLastPost(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := DB.ExecContext(ctx, "UPDATE threads SET last_post_at = CURRENT_TIMESTAMP WHERE id = $1", id)
	return err
}

// Delete removes a thread together with its posts. The schema has no
// ON DELETE CASCADE, so posts are removed explicitly inside a transaction.
func (tr ThreadRepository) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM posts WHERE thread_id = $1", id); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM threads WHERE id = $1", id); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (tr ThreadRepository) GetAll() ([]models.Thread, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DB.QueryContext(ctx,
		"SELECT id, forum_id, title, author_id, created_at, last_post_at FROM threads",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []models.Thread
	for rows.Next() {
		var t models.Thread
		if err := rows.Scan(&t.ID, &t.ForumID, &t.Title, &t.AuthorID, &t.CreatedAt, &t.LastPostAt); err != nil {
			return nil, err
		}
		threads = append(threads, t)
	}
	return threads, rows.Err()
}
