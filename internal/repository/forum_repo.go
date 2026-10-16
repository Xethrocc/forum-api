package repository

import (
	"context"
	"time"

	"forum-api/internal/models"
)

type ForumRepository struct{}

func (fr ForumRepository) Create(title, description string, parentID, createdBy *int64) (*models.Forum, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var forum models.Forum
	query := `INSERT INTO forums (title, description, parent_forum_id, created_by) VALUES ($1, $2, $3, $4) RETURNING id, created_at`
	err := DB.QueryRowContext(ctx, query, title, description, parentID, createdBy).Scan(&forum.ID, &forum.CreatedAt)
	if err != nil {
		return nil, err
	}

	forum.Title = title
	forum.Description = description
	forum.ParentForumID = parentID
	forum.CreatedBy = createdBy

	return &forum, nil
}

func (fr ForumRepository) Update(id int64, title, description string) (*models.Forum, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var forum models.Forum
	err := DB.QueryRowContext(ctx,
		"UPDATE forums SET title = $1, description = $2 WHERE id = $3 RETURNING id, title, description, parent_forum_id, created_by, created_at",
		title, description, id,
	).Scan(&forum.ID, &forum.Title, &forum.Description, &forum.ParentForumID, &forum.CreatedBy, &forum.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &forum, nil
}

func (fr ForumRepository) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := DB.ExecContext(ctx, "DELETE FROM forums WHERE id = $1", id)
	return err
}

// HasChildren reports whether a forum still holds subforums or threads.
// Only empty forums may be deleted.
func (fr ForumRepository) HasChildren(id int64) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var has bool
	err := DB.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM forums WHERE parent_forum_id = $1)
		 OR EXISTS(SELECT 1 FROM threads WHERE forum_id = $1)`,
		id,
	).Scan(&has)
	return has, err
}

func (fr ForumRepository) GetAll() ([]models.Forum, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DB.QueryContext(ctx, "SELECT id, title, description, parent_forum_id, created_by, created_at FROM forums ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var forums []models.Forum
	for rows.Next() {
		var f models.Forum
		if err := rows.Scan(&f.ID, &f.Title, &f.Description, &f.ParentForumID, &f.CreatedBy, &f.CreatedAt); err != nil {
			return nil, err
		}
		forums = append(forums, f)
	}

	return forums, rows.Err()
}

func (fr ForumRepository) GetAllPaginated(offset, limit int) ([]models.Forum, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get total count
	var total int64
	err := DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM forums").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	rows, err := DB.QueryContext(ctx,
		"SELECT id, title, description, parent_forum_id, created_by, created_at FROM forums ORDER BY id LIMIT $1 OFFSET $2",
		limit, offset,
	)
	if err != nil {
		return nil, total, err
	}
	defer rows.Close()

	var forums []models.Forum
	for rows.Next() {
		var f models.Forum
		if err := rows.Scan(&f.ID, &f.Title, &f.Description, &f.ParentForumID, &f.CreatedBy, &f.CreatedAt); err != nil {
			return nil, total, err
		}
		forums = append(forums, f)
	}

	return forums, total, rows.Err()
}

func (fr ForumRepository) GetByID(id int64) (*models.Forum, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var forum models.Forum
	err := DB.QueryRowContext(ctx,
		"SELECT id, title, description, parent_forum_id, created_by, created_at FROM forums WHERE id = $1",
		id,
	).Scan(&forum.ID, &forum.Title, &forum.Description, &forum.ParentForumID, &forum.CreatedBy, &forum.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &forum, nil
}
