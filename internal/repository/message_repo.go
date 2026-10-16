package repository

import (
	"forum-api/internal/models"
	"context"
	"time"
)

type MessageRepository struct{}

func (mr MessageRepository) Create(senderID, recipientID int64, content string) (*models.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var msg models.Message
	err := DB.QueryRowContext(ctx,
		"INSERT INTO messages (sender_id, recipient_id, content) VALUES ($1, $2, $3) RETURNING id, created_at",
		senderID, recipientID, content,
	).Scan(&msg.ID, &msg.CreatedAt)

	if err != nil {
		return nil, err
	}

	msg.SenderID = senderID
	msg.RecipientID = recipientID
	msg.Content = content
	msg.IsRead = false

	return &msg, nil
}

func (mr MessageRepository) GetAllByRecipient(userID int64) ([]models.Message, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := DB.QueryContext(ctx,
		"SELECT id, sender_id, recipient_id, content, is_read, created_at FROM messages WHERE recipient_id = $1 ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.Content, &m.IsRead, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	return messages, rows.Err()
}

func (mr MessageRepository) GetAllByRecipientPaginated(userID int64, offset, limit int) ([]models.Message, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get total count
	var total int64
	err := DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages WHERE recipient_id = $1", userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	rows, err := DB.QueryContext(ctx,
		"SELECT id, sender_id, recipient_id, content, is_read, created_at FROM messages WHERE recipient_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		userID, limit, offset,
	)
	if err != nil {
		return nil, total, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.SenderID, &m.RecipientID, &m.Content, &m.IsRead, &m.CreatedAt); err != nil {
			return nil, total, err
		}
		messages = append(messages, m)
	}

	return messages, total, rows.Err()
}

func (mr MessageRepository) MarkAsRead(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := DB.ExecContext(ctx, "UPDATE messages SET is_read = TRUE WHERE id = $1", id)
	return err
}
