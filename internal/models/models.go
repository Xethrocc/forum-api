package models

import "time"

// User represents a forum user
type User struct {
	ID        int64     `json:"id"`
	Username string    `json:"username"`
	PasswordHash string  `json:"-"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// AuthRequest for login
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse for login response
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// Forum represents a forum or subforum
type Forum struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ParentForumID *int64 `json:"parent_forum_id,omitempty"`
	CreatedBy     *int64 `json:"created_by,omitempty"`
	ModeratorIDs []int64 `json:"moderator_ids"`
	CreatedAt    time.Time `json:"created_at"`
}

// Thread represents a discussion thread
type Thread struct {
	ID        int64     `json:"id"`
	ForumID   int64     `json:"forum_id"`
	Title     string    `json:"title"`
	AuthorID  int64     `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
	LastPostAt time.Time `json:"last_post_at"`
}

// Post represents a post in a thread
type Post struct {
	ID        int64     `json:"id"`
	ThreadID  int64     `json:"thread_id"`
	AuthorID  int64     `json:"author_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Message represents a private message
type Message struct {
	ID        int64     `json:"id"`
	SenderID  int64     `json:"sender_id"`
	RecipientID int64   `json:"recipient_id"`
	Content   string    `json:"content"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

// SearchQuery for API search requests
type SearchQuery struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// SearchResult for search responses
// Type is either "thread" or "post" and identifies which table ID refers to.
// RelevanceScore is the ts_rank of the match; results are ordered by it.
type SearchResult struct {
	ID             int64     `json:"id"`
	Type           string    `json:"type"`
	Content        string    `json:"content"`
	RelevanceScore float64   `json:"relevance_score"`
	CreatedAt      time.Time `json:"created_at"`
}
