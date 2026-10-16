package repository

import (
	"forum-api/internal/models"
	"context"
	"time"
)

type PostRepository struct{}

func (pr PostRepository) Create(threadID int64, content string, authorID int64) (*models.Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var post models.Post
	err := DB.QueryRowContext(ctx,
		"INSERT INTO posts (thread_id, content, author_id) VALUES ($1, $2, $3) RETURNING id, created_at",
		threadID, content, authorID,
	).Scan(&post.ID, &post.CreatedAt)

	if err != nil {
		return nil, err
	}

	post.ThreadID = threadID
	post.Content = content
	post.AuthorID = authorID

	return &post, nil
}

func (pr PostRepository) GetByID(id int64) (*models.Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var post models.Post
	err := DB.QueryRowContext(ctx,
		"SELECT id, thread_id, author_id, content, created_at FROM posts WHERE id = $1",
		id,
	).Scan(&post.ID, &post.ThreadID, &post.AuthorID, &post.Content, &post.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &post, nil
}

func (pr PostRepository) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := DB.ExecContext(ctx, "DELETE FROM posts WHERE id = $1", id)
	return err
}

func (pr PostRepository) GetAllByThread(threadID int64) ([]models.Post, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DB.QueryContext(ctx,
		"SELECT id, thread_id, author_id, content, created_at FROM posts WHERE thread_id = $1 ORDER BY created_at ASC",
		threadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(&p.ID, &p.ThreadID, &p.AuthorID, &p.Content, &p.CreatedAt); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}

	return posts, rows.Err()
}

func (pr PostRepository) GetAllByThreadPaginated(threadID int64, offset, limit int) ([]models.Post, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get total count
	var total int64
	err := DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts WHERE thread_id = $1", threadID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	rows, err := DB.QueryContext(ctx,
		"SELECT id, thread_id, author_id, content, created_at FROM posts WHERE thread_id = $1 ORDER BY created_at ASC LIMIT $2 OFFSET $3",
		threadID, limit, offset,
	)
	if err != nil {
		return nil, total, err
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(&p.ID, &p.ThreadID, &p.AuthorID, &p.Content, &p.CreatedAt); err != nil {
			return nil, total, err
		}
		posts = append(posts, p)
	}

	return posts, total, rows.Err()
}
