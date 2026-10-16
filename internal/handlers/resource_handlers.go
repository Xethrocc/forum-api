package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"forum-api/internal/auth"
	"forum-api/internal/models"
	"forum-api/internal/repository"
)

// userFromContext returns the authenticated user set by the JWTAuth middleware.
func userFromContext(r *http.Request) *models.User {
	u, _ := r.Context().Value("user").(*models.User)
	return u
}

func isAdmin(u *models.User) bool {
	return u != nil && u.Role == "admin"
}

func isModerator(u *models.User) bool {
	return u != nil && (u.Role == "moderator" || u.Role == "admin")
}

// canManageForum: Hauptforen (Kategorien) verwaltet nur admin; Unterforen
// der Ersteller oder mod/admin. Siehe FORUM_STRUKTUR.md.
func canManageForum(u *models.User, f *models.Forum) bool {
	if f.ParentForumID == nil {
		return isAdmin(u)
	}
	if isModerator(u) {
		return true
	}
	return f.CreatedBy != nil && *f.CreatedBy == u.ID
}

// ForumHandler handles forum CRUD
func ForumHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := r.URL.Path
	// Extract ID if present
	parts := strings.Split(path, "/")
	id := ""
	for i, p := range parts {
		if p == "forums" && i+1 < len(parts) {
			id = parts[i+1]
			break
		}
	}

	switch r.Method {
	case http.MethodPost:
		user := userFromContext(r)
		if user == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		var req struct {
			Title       string  `json:"title"`
			Description string  `json:"description"`
			ParentID    *int64  `json:"parent_forum_id"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Title == "" {
			http.Error(w, `{"error":"Title is required"}`, http.StatusBadRequest)
			return
		}

		createdBy := user.ID
		if req.ParentID == nil {
			// Hauptforen sind statisch: nur admin darf sie anlegen
			if !isAdmin(user) {
				http.Error(w, `{"error":"Only admins can create top-level forums"}`, http.StatusForbidden)
				return
			}
		} else {
			// Unterforen hängen direkt unter einem Hauptforum
			parent, err := repository.ForumRepository{}.GetByID(*req.ParentID)
			if err != nil {
				http.Error(w, `{"error":"Parent forum not found"}`, http.StatusBadRequest)
				return
			}
			if parent.ParentForumID != nil {
				http.Error(w, `{"error":"Subforums can only be created under top-level forums"}`, http.StatusBadRequest)
				return
			}
		}

		forum, err := repository.ForumRepository{}.Create(req.Title, req.Description, req.ParentID, &createdBy)
		if err != nil {
			http.Error(w, `{"error":"Failed to create forum"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": forum.ID,
			"title": forum.Title,
			"description": forum.Description,
			"parent_forum_id": forum.ParentForumID,
			"created_at": forum.CreatedAt,
		})

	case http.MethodGet:
		if id != "" {
			forumID, _ := strconv.ParseInt(id, 10, 64)
			forum, err := repository.ForumRepository{}.GetByID(forumID)
			if err != nil {
				http.Error(w, `{"error":"Forum not found"}`, http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(forum)
		} else {
			page := 1
			limit := 10
			if p := r.URL.Query().Get("page"); p != "" {
				if val, err := strconv.Atoi(p); err == nil && val > 0 {
					page = val
				}
			}
			if l := r.URL.Query().Get("limit"); l != "" {
				if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
					limit = val
				}
			}
			offset := (page - 1) * limit

			forums, total, err := repository.ForumRepository{}.GetAllPaginated(offset, limit)
			if err != nil {
				http.Error(w, `{"error":"Failed to fetch forums"}`, http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": forums,
				"meta": map[string]interface{}{
					"total":    total,
					"page":     page,
					"limit":    limit,
					"has_next": total > int64(page*limit),
				},
			})
		}

	case http.MethodPut:
		if id == "" {
			http.Error(w, `{"error":"Forum ID required"}`, http.StatusBadRequest)
			return
		}
		user := userFromContext(r)
		if user == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		forumID, _ := strconv.ParseInt(id, 10, 64)
		forum, err := repository.ForumRepository{}.GetByID(forumID)
		if err != nil {
			http.Error(w, `{"error":"Forum not found"}`, http.StatusNotFound)
			return
		}
		if !canManageForum(user, forum) {
			http.Error(w, `{"error":"Forbidden"}`, http.StatusForbidden)
			return
		}

		var req struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Title == "" {
			http.Error(w, `{"error":"Title is required"}`, http.StatusBadRequest)
			return
		}

		updated, err := repository.ForumRepository{}.Update(forumID, req.Title, req.Description)
		if err != nil {
			http.Error(w, `{"error":"Failed to update forum"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(updated)

	case http.MethodDelete:
		if id == "" {
			http.Error(w, `{"error":"Forum ID required"}`, http.StatusBadRequest)
			return
		}
		user := userFromContext(r)
		if user == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		forumID, _ := strconv.ParseInt(id, 10, 64)
		forum, err := repository.ForumRepository{}.GetByID(forumID)
		if err != nil {
			http.Error(w, `{"error":"Forum not found"}`, http.StatusNotFound)
			return
		}
		if !canManageForum(user, forum) {
			http.Error(w, `{"error":"Forbidden"}`, http.StatusForbidden)
			return
		}

		// Löschen nur, wenn keine Unterforen und keine Themen drin sind
		has, err := repository.ForumRepository{}.HasChildren(forumID)
		if err != nil {
			http.Error(w, `{"error":"Failed to check forum contents"}`, http.StatusInternalServerError)
			return
		}
		if has {
			http.Error(w, `{"error":"Forum is not empty"}`, http.StatusConflict)
			return
		}

		if err := (repository.ForumRepository{}).Delete(forumID); err != nil {
			http.Error(w, `{"error":"Failed to delete forum"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Forum deleted",
			"id":      forumID,
		})

	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// ThreadHandler handles thread operations
func ThreadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := r.URL.Path
	parts := strings.Split(path, "/")
	forumID := ""
	threadID := ""

	for i, p := range parts {
		if p == "forums" && i+2 < len(parts) && parts[i+1] != "threads" {
			forumID = parts[i+1]
		}
		if p == "threads" && i+1 < len(parts) {
			threadID = parts[i+1]
		}
	}

	switch r.Method {
	case http.MethodPost:
		user := userFromContext(r)
		if user == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if forumID == "" {
			http.Error(w, `{"error":"Forum ID required"}`, http.StatusBadRequest)
			return
		}

		// author_id im Body wird ignoriert: Autor ist immer der Token-User
		var req struct {
			Title string `json:"title"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Title == "" {
			http.Error(w, `{"error":"Title is required"}`, http.StatusBadRequest)
			return
		}

		fID, _ := strconv.ParseInt(forumID, 10, 64)

		// Themen sind nur in Unterforen erlaubt (Hauptforen sind Kategorien)
		forum, err := repository.ForumRepository{}.GetByID(fID)
		if err != nil {
			http.Error(w, `{"error":"Forum not found"}`, http.StatusBadRequest)
			return
		}
		if forum.ParentForumID == nil {
			http.Error(w, `{"error":"Threads can only be created in subforums"}`, http.StatusBadRequest)
			return
		}

		thread, err := repository.ThreadRepository{}.Create(fID, req.Title, user.ID)
		if err != nil {
			http.Error(w, `{"error":"Failed to create thread"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": thread.ID,
			"title": thread.Title,
			"forum_id": thread.ForumID,
			"author_id": thread.AuthorID,
			"created_at": thread.CreatedAt,
		})

	case http.MethodGet:
		if threadID != "" {
			id, _ := strconv.ParseInt(threadID, 10, 64)
			thread, err := repository.ThreadRepository{}.GetByID(id)
			if err != nil {
				http.Error(w, `{"error":"Thread not found"}`, http.StatusNotFound)
				return
			}

			posts, _ := repository.PostRepository{}.GetAllByThread(id)

			json.NewEncoder(w).Encode(map[string]interface{}{
				"thread": thread,
				"posts": posts,
			})
		} else {
			var fID int64 = 1
			if forumID != "" {
				fID, _ = strconv.ParseInt(forumID, 10, 64)
			}
			page := 1
			limit := 10
			if p := r.URL.Query().Get("page"); p != "" {
				if val, err := strconv.Atoi(p); err == nil && val > 0 {
					page = val
				}
			}
			if l := r.URL.Query().Get("limit"); l != "" {
				if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
					limit = val
				}
			}
			offset := (page - 1) * limit

			threads, total, err := repository.ThreadRepository{}.GetAllByForumPaginated(fID, offset, limit)
			if err != nil {
				http.Error(w, `{"error":"Failed to fetch threads"}`, http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": threads,
				"meta": map[string]interface{}{
					"total":    total,
					"page":     page,
					"limit":    limit,
					"has_next": total > int64(page*limit),
				},
			})
		}

	case http.MethodDelete:
		if threadID == "" {
			http.Error(w, `{"error":"Thread ID required"}`, http.StatusBadRequest)
			return
		}
		user := userFromContext(r)
		if user == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		id, _ := strconv.ParseInt(threadID, 10, 64)
		thread, err := repository.ThreadRepository{}.GetByID(id)
		if err != nil {
			http.Error(w, `{"error":"Thread not found"}`, http.StatusNotFound)
			return
		}
		// Löschen darf der Ersteller oder mod/admin
		if thread.AuthorID != user.ID && !isModerator(user) {
			http.Error(w, `{"error":"Forbidden"}`, http.StatusForbidden)
			return
		}
		if err := (repository.ThreadRepository{}).Delete(id); err != nil {
			http.Error(w, `{"error":"Failed to delete thread"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Thread deleted",
			"id":      id,
		})

	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// PostHandler handles post operations
func PostHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := r.URL.Path
	parts := strings.Split(path, "/")
	threadID := ""
	postID := ""

	for i, p := range parts {
		if p == "threads" && i+2 < len(parts) && parts[i+1] != "posts" {
			threadID = parts[i+1]
		}
		if p == "posts" && i+1 < len(parts) {
			postID = parts[i+1]
		}
	}

	if threadID == "" {
		http.Error(w, `{"error":"Thread ID required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		user := userFromContext(r)
		if user == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// author_id im Body wird ignoriert: Autor ist immer der Token-User
		var req struct {
			Content string `json:"content"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Content == "" {
			http.Error(w, `{"error":"Content is required"}`, http.StatusBadRequest)
			return
		}

		threadIDInt, _ := strconv.ParseInt(threadID, 10, 64)
		post, err := repository.PostRepository{}.Create(threadIDInt, req.Content, user.ID)
		if err != nil {
			http.Error(w, `{"error":"Failed to create post"}`, http.StatusInternalServerError)
			return
		}

		repository.ThreadRepository{}.UpdateLastPost(threadIDInt)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": post.ID,
			"content": post.Content,
			"author_id": post.AuthorID,
			"created_at": post.CreatedAt,
		})

	case http.MethodGet:
		id, _ := strconv.ParseInt(threadID, 10, 64)
		page := 1
		limit := 10
		if p := r.URL.Query().Get("page"); p != "" {
			if val, err := strconv.Atoi(p); err == nil && val > 0 {
				page = val
			}
		}
		if l := r.URL.Query().Get("limit"); l != "" {
			if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
				limit = val
			}
		}
		offset := (page - 1) * limit

		posts, total, err := repository.PostRepository{}.GetAllByThreadPaginated(id, offset, limit)
		if err != nil {
			http.Error(w, `{"error":"Failed to fetch posts"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": posts,
			"meta": map[string]interface{}{
				"total":    total,
				"page":     page,
				"limit":    limit,
				"has_next": total > int64(page*limit),
			},
		})

	case http.MethodDelete:
		if postID == "" {
			http.Error(w, `{"error":"Post ID required"}`, http.StatusBadRequest)
			return
		}
		user := userFromContext(r)
		if user == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		pID, _ := strconv.ParseInt(postID, 10, 64)
		post, err := repository.PostRepository{}.GetByID(pID)
		if err != nil {
			http.Error(w, `{"error":"Post not found"}`, http.StatusNotFound)
			return
		}
		// Löschen darf der Ersteller oder mod/admin
		if post.AuthorID != user.ID && !isModerator(user) {
			http.Error(w, `{"error":"Forbidden"}`, http.StatusForbidden)
			return
		}
		if err := (repository.PostRepository{}).Delete(pID); err != nil {
			http.Error(w, `{"error":"Failed to delete post"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Post deleted",
			"id":      pID,
		})

	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// MessageHandler handles private messages
func MessageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get authenticated user from JWT token
	token := r.Header.Get("Authorization")
	user, err := auth.GetAuthenticatedUser(token)
	if err != nil {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req struct {
			RecipientID int64  `json:"recipient_id"`
			Content     string `json:"content"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Content == "" {
			http.Error(w, `{"error":"Content is required"}`, http.StatusBadRequest)
			return
		}

		msg, err := repository.MessageRepository{}.Create(user.ID, req.RecipientID, req.Content)
		if err != nil {
			http.Error(w, `{"error":"Failed to send message"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": msg.ID,
			"content": msg.Content,
			"recipient_id": msg.RecipientID,
			"created_at": msg.CreatedAt,
		})

	case http.MethodGet:
		page := 1
		limit := 10
		if p := r.URL.Query().Get("page"); p != "" {
			if val, err := strconv.Atoi(p); err == nil && val > 0 {
				page = val
			}
		}
		if l := r.URL.Query().Get("limit"); l != "" {
			if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
				limit = val
			}
		}
		offset := (page - 1) * limit

		messages, total, err := repository.MessageRepository{}.GetAllByRecipientPaginated(user.ID, offset, limit)
		if err != nil {
			http.Error(w, `{"error":"Failed to fetch messages"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": messages,
			"meta": map[string]interface{}{
				"total":    total,
				"page":     page,
				"limit":    limit,
				"has_next": total > int64(page*limit),
			},
		})

	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// SearchHandler handles full-text search across threads and posts
func SearchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, `{"error":"Missing query parameter"}`, http.StatusBadRequest)
		return
	}

	page := 1
	limit := 10
	if p := r.URL.Query().Get("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}

	results, total, err := repository.SearchRepository{}.Search(query, page, limit)
	if err != nil {
		http.Error(w, `{"error":"Failed to search"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": results,
		"meta": map[string]interface{}{
			"total":    total,
			"page":     page,
			"limit":    limit,
			"has_next": total > int64(page*limit),
			"query":    query,
		},
	})
}

// UserHandler handles user profile operations
func UserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		GetUser(w, r)
	} else {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// AdminUserHandler handles admin-only user operations
func AdminUserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Extract user ID from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	userID := ""
	for i, p := range parts {
		if p == "users" && i+1 < len(parts) {
			userID = parts[i+1]
			break
		}
	}

	if userID == "" {
		http.Error(w, `{"error":"User ID required"}`, http.StatusBadRequest)
		return
	}

	targetID, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"Invalid user ID"}`, http.StatusBadRequest)
		return
	}

	// Suspend user
	err = repository.UserRepository{}.Suspend(targetID)
	if err != nil {
		http.Error(w, `{"error":"Failed to suspend user"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "User suspended successfully",
		"user_id": targetID,
	})
}

// AdminStatsHandler returns admin statistics
func AdminStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	users, err := repository.UserRepository{}.GetAll()
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch stats"}`, http.StatusInternalServerError)
		return
	}

	forums, err := repository.ForumRepository{}.GetAll()
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch stats"}`, http.StatusInternalServerError)
		return
	}

	threads, err := repository.ThreadRepository{}.GetAll()
	if err != nil {
		http.Error(w, `{"error":"Failed to fetch stats"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_users": len(users),
		"total_forums": len(forums),
		"total_threads": len(threads),
	})
}
