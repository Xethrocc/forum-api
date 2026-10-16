-- Database Schema for Forum API

-- Users table
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) DEFAULT 'user',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Forums table (includes subforums via parent_forum_id)
-- Hauptforen: parent_forum_id IS NULL, statisch (nur admin).
-- Unterforen: parent_forum_id zeigt auf ein Hauptforum, created_by = Ersteller.
CREATE TABLE forums (
    id SERIAL PRIMARY KEY,
    title VARCHAR(100) NOT NULL,
    description TEXT,
    parent_forum_id INTEGER REFERENCES forums(id),
    created_by INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Thread table
CREATE TABLE threads (
    id SERIAL PRIMARY KEY,
    forum_id INTEGER NOT NULL REFERENCES forums(id),
    title VARCHAR(200) NOT NULL,
    author_id INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_post_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    search_vector tsvector
);

-- Posts table
CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    thread_id INTEGER NOT NULL REFERENCES threads(id),
    author_id INTEGER NOT NULL REFERENCES users(id),
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    search_vector tsvector
);

-- Messages (private) table
CREATE TABLE messages (
    id SERIAL PRIMARY KEY,
    sender_id INTEGER NOT NULL REFERENCES users(id),
    recipient_id INTEGER NOT NULL REFERENCES users(id),
    content TEXT NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_threads_forum_id ON threads(forum_id);
CREATE INDEX idx_posts_thread_id ON posts(thread_id);
CREATE INDEX idx_messages_recipient ON messages(recipient_id);
CREATE INDEX idx_users_username ON users(username);

-- Full-text search (tsvector/tsquery)
-- GIN indexes make `search_vector @@ plainto_tsquery(...)` lookups fast.
CREATE INDEX idx_threads_search_vector ON threads USING GIN (search_vector);
CREATE INDEX idx_posts_search_vector ON posts USING GIN (search_vector);

-- Trigger functions that keep search_vector in sync with the indexed text.
-- Threads are searched by title (weight A), posts by content (weight B), so
-- matches in thread titles rank above matches in post bodies.
CREATE OR REPLACE FUNCTION threads_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A');
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_threads_search_vector
    BEFORE INSERT OR UPDATE ON threads
    FOR EACH ROW EXECUTE FUNCTION threads_search_vector_update();

CREATE OR REPLACE FUNCTION posts_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := setweight(to_tsvector('english', COALESCE(NEW.content, '')), 'B');
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_posts_search_vector
    BEFORE INSERT OR UPDATE ON posts
    FOR EACH ROW EXECUTE FUNCTION posts_search_vector_update();

-- Migration for databases created before full-text search existed.
-- The triggers above keep the vectors current afterwards; the UPDATEs backfill
-- the rows that were written before the triggers were installed.
-- ALTER TABLE threads ADD COLUMN IF NOT EXISTS search_vector tsvector;
-- ALTER TABLE posts ADD COLUMN IF NOT EXISTS search_vector tsvector;
-- UPDATE threads SET search_vector = setweight(to_tsvector('english', COALESCE(title, '')), 'A');
-- UPDATE posts SET search_vector = setweight(to_tsvector('english', COALESCE(content, '')), 'B');

-- Seed: statische Hauptforen (Kategorien, Thema Wissen & Information).
-- Idempotent, damit Schema-Init und spätere Läufe nicht duplizieren.
-- Für bestehende Datenbanken: ALTER TABLE forums ADD COLUMN IF NOT EXISTS created_by INTEGER REFERENCES users(id);
-- ALTER TABLE users DROP COLUMN IF EXISTS email; (E-Mail-Adresse entfällt, siehe FORUM_STRUKTUR.md)
INSERT INTO forums (title, description)
SELECT v.title, v.description FROM (VALUES
    ('Wissen & Nachschlagen', 'Dokumentationen, Tutorials, How-tos, Referenzen'),
    ('Technik & IT', 'Hardware, Netzwerk, Systeme, Self-Hosting'),
    ('Softwareentwicklung', 'Sprachen, Tools, Code, Projekte'),
    ('KI & Automatisierung', 'Modelle, Agenten, Workflows'),
    ('Wissenschaft & Forschung', 'Papers, Methoden, Erkenntnisse'),
    ('Fragen & Austausch', 'Fragen, Diskussion, Transfer')
) AS v(title, description)
WHERE NOT EXISTS (SELECT 1 FROM forums f WHERE f.title = v.title);
