package dao

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/arussellsaw/news/domain"
	_ "modernc.org/sqlite"
)

var (
	db           *sql.DB
	mu           sync.RWMutex
	articleCache = make(map[string]domain.Article)

	// Source cache keyed by feed_url to avoid N+1 queries
	sourceCache   = make(map[string]*domain.Source)
	sourceCacheMu sync.RWMutex
)

func Init(ctx context.Context) error {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "news.db"
	}

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	_, err = db.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("setting WAL mode: %w", err)
	}

	if err := createTables(ctx); err != nil {
		return fmt.Errorf("creating tables: %w", err)
	}

	if err := seedDefaultSource(ctx); err != nil {
		return fmt.Errorf("seeding default source: %w", err)
	}

	return nil
}

func createTables(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			feed_url TEXT NOT NULL UNIQUE,
			categories TEXT DEFAULT '[]',
			image_headers TEXT DEFAULT '{}',
			disable_img_proxy INTEGER DEFAULT 0,
			enabled INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS articles (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			content TEXT DEFAULT '[]',
			image_url TEXT DEFAULT '',
			link TEXT UNIQUE,
			author TEXT DEFAULT '',
			source_name TEXT DEFAULT '',
			source_feed_url TEXT DEFAULT '',
			timestamp DATETIME,
			ts TEXT DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS editions (
			id TEXT PRIMARY KEY,
			name TEXT DEFAULT '',
			date TEXT DEFAULT '',
			start_time DATETIME,
			end_time DATETIME,
			created_at DATETIME,
			article_ids TEXT DEFAULT '[]',
			categories TEXT DEFAULT '[]',
			metadata TEXT DEFAULT '{}'
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_timestamp ON articles(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_source ON articles(source_feed_url)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_link ON articles(link)`,
	}

	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("executing query: %w", err)
		}
	}

	// Migration: add image_headers column to existing databases
	db.ExecContext(ctx, `ALTER TABLE sources ADD COLUMN image_headers TEXT DEFAULT '{}'`)

	// Migration: add disable_img_proxy column to existing databases
	db.ExecContext(ctx, `ALTER TABLE sources ADD COLUMN disable_img_proxy INTEGER DEFAULT 0`)

	return nil
}

func seedDefaultSource(ctx context.Context) error {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sources").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	cats, _ := json.Marshal([]string{"新闻"})
	_, err = db.ExecContext(ctx,
		"INSERT INTO sources (name, url, feed_url, categories, enabled) VALUES (?, ?, ?, ?, 1)",
		"澎湃新闻", "https://www.thepaper.cn", "https://rsshub.app/thepaper/newsDetail", string(cats),
	)
	if err != nil {
		log.Printf("Warning: failed to seed default source: %v", err)
	}
	return nil
}

// ===== Source operations =====

func GetSources(ctx context.Context) ([]domain.Source, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, name, url, feed_url, categories, image_headers, disable_img_proxy, enabled FROM sources WHERE enabled = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []domain.Source
	for rows.Next() {
		var s domain.Source
		var catsJSON string
		var hdrsJSON string
		var disableImgProxy int
		var enabled int
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.FeedURL, &catsJSON, &hdrsJSON, &disableImgProxy, &enabled); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(catsJSON), &s.Categories)
		json.Unmarshal([]byte(hdrsJSON), &s.ImageHeaders)
		s.DisableImgProxy = disableImgProxy == 1
		s.Enabled = enabled == 1
		sources = append(sources, s)
	}

	// Populate source cache
	sourceCacheMu.Lock()
	for i := range sources {
		sourceCache[sources[i].FeedURL] = &sources[i]
	}
	sourceCacheMu.Unlock()

	return sources, nil
}

func GetAllSources(ctx context.Context) ([]domain.Source, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, name, url, feed_url, categories, image_headers, disable_img_proxy, enabled FROM sources ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []domain.Source
	for rows.Next() {
		var s domain.Source
		var catsJSON string
		var hdrsJSON string
		var disableImgProxy int
		var enabled int
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.FeedURL, &catsJSON, &hdrsJSON, &disableImgProxy, &enabled); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(catsJSON), &s.Categories)
		json.Unmarshal([]byte(hdrsJSON), &s.ImageHeaders)
		s.DisableImgProxy = disableImgProxy == 1
		s.Enabled = enabled == 1
		sources = append(sources, s)
	}
	return sources, nil
}

func AddSource(ctx context.Context, s *domain.Source) error {
	cats, _ := json.Marshal(s.Categories)
	hdrs, _ := json.Marshal(s.ImageHeaders)
	result, err := db.ExecContext(ctx,
		"INSERT INTO sources (name, url, feed_url, categories, image_headers, disable_img_proxy, enabled) VALUES (?, ?, ?, ?, ?, ?, ?)",
		s.Name, s.URL, s.FeedURL, string(cats), string(hdrs), boolToInt(s.DisableImgProxy), boolToInt(s.Enabled),
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	s.ID = int(id)
	return nil
}

func UpdateSource(ctx context.Context, s *domain.Source) error {
	cats, _ := json.Marshal(s.Categories)
	hdrs, _ := json.Marshal(s.ImageHeaders)
	_, err := db.ExecContext(ctx,
		"UPDATE sources SET name=?, url=?, feed_url=?, categories=?, image_headers=?, disable_img_proxy=?, enabled=? WHERE id=?",
		s.Name, s.URL, s.FeedURL, string(cats), string(hdrs), boolToInt(s.DisableImgProxy), boolToInt(s.Enabled), s.ID,
	)
	return err
}

func DeleteSource(ctx context.Context, id int) error {
	_, err := db.ExecContext(ctx, "DELETE FROM sources WHERE id=?", id)
	return err
}

func GetSourceByID(ctx context.Context, id int) (*domain.Source, error) {
	var s domain.Source
	var catsJSON string
	var hdrsJSON string
	var disableImgProxy int
	var enabled int
	err := db.QueryRowContext(ctx,
		"SELECT id, name, url, feed_url, categories, image_headers, disable_img_proxy, enabled FROM sources WHERE id=?", id,
	).Scan(&s.ID, &s.Name, &s.URL, &s.FeedURL, &catsJSON, &hdrsJSON, &disableImgProxy, &enabled)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(catsJSON), &s.Categories)
	json.Unmarshal([]byte(hdrsJSON), &s.ImageHeaders)
	s.DisableImgProxy = disableImgProxy == 1
	s.Enabled = enabled == 1
	return &s, nil
}

// ===== Article operations =====

func SetArticle(ctx context.Context, a *domain.Article) error {
	contentJSON, _ := json.Marshal(a.Content)
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO articles (id, title, description, content, image_url, link, author, source_name, source_feed_url, timestamp, ts)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Title, a.Description, string(contentJSON), a.ImageURL, a.Link,
		a.Author, a.Source.Name, a.Source.FeedURL, a.Timestamp, a.TS,
	)
	if err != nil {
		return err
	}
	mu.Lock()
	articleCache[a.ID] = *a
	mu.Unlock()
	return nil
}

func scanArticle(row interface{ Scan(dest ...interface{}) error }) (*domain.Article, error) {
	var a domain.Article
	var contentJSON string
	var ts time.Time
	err := row.Scan(&a.ID, &a.Title, &a.Description, &contentJSON, &a.ImageURL, &a.Link,
		&a.Author, &a.Source.Name, &a.Source.FeedURL, &ts, &a.TS)
	if err != nil {
		return nil, err
	}
	a.Timestamp = ts
	json.Unmarshal([]byte(contentJSON), &a.Content)

	// Use source cache first, fall back to DB query
	sourceCacheMu.RLock()
	if cached, ok := sourceCache[a.Source.FeedURL]; ok {
		a.Source = *cached
		sourceCacheMu.RUnlock()
	} else {
		sourceCacheMu.RUnlock()
		if fullSrc, err := getSourceByFeedURL(context.Background(), a.Source.FeedURL); err == nil {
			a.Source = *fullSrc
			// Populate cache for future lookups
			sourceCacheMu.Lock()
			sourceCache[a.Source.FeedURL] = fullSrc
			sourceCacheMu.Unlock()
		} else {
			a.Source = domain.Source{Name: a.Source.Name, FeedURL: a.Source.FeedURL}
		}
	}
	return &a, nil
}

func getSourceByFeedURL(ctx context.Context, feedURL string) (*domain.Source, error) {
	var s domain.Source
	var catsJSON string
	var hdrsJSON string
	var disableImgProxy int
	var enabled int
	err := db.QueryRowContext(ctx,
		"SELECT id, name, url, feed_url, categories, image_headers, disable_img_proxy, enabled FROM sources WHERE feed_url=?", feedURL,
	).Scan(&s.ID, &s.Name, &s.URL, &s.FeedURL, &catsJSON, &hdrsJSON, &disableImgProxy, &enabled)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(catsJSON), &s.Categories)
	json.Unmarshal([]byte(hdrsJSON), &s.ImageHeaders)
	s.DisableImgProxy = disableImgProxy == 1
	s.Enabled = enabled == 1
	return &s, nil
}

func GetArticle(ctx context.Context, id string) (*domain.Article, error) {
	mu.RLock()
	cached, ok := articleCache[id]
	if ok {
		mu.RUnlock()
		return &cached, nil
	}
	mu.RUnlock()

	row := db.QueryRowContext(ctx,
		"SELECT id, title, description, content, image_url, link, author, source_name, source_feed_url, timestamp, ts FROM articles WHERE id=?",
		id,
	)
	a, err := scanArticle(row)
	if err != nil {
		return nil, err
	}

	mu.Lock()
	articleCache[a.ID] = *a
	mu.Unlock()
	return a, nil
}

func GetArticleByURL(ctx context.Context, url string) (*domain.Article, error) {
	row := db.QueryRowContext(ctx,
		"SELECT id, title, description, content, image_url, link, author, source_name, source_feed_url, timestamp, ts FROM articles WHERE link=?",
		url,
	)
	a, err := scanArticle(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

func GetArticlesByTime(ctx context.Context, start, end time.Time) ([]domain.Article, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, title, description, content, image_url, link, author, source_name, source_feed_url, timestamp, ts FROM articles WHERE timestamp > ? AND timestamp < ? ORDER BY timestamp DESC",
		start, end,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []domain.Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		articles = append(articles, *a)
	}
	return articles, nil
}

func GetArticles(ctx context.Context, page, pageSize int, sourceFilter string) ([]domain.Article, int, error) {
	offset := (page - 1) * pageSize

	var countQuery, dataQuery string
	var args []interface{}

	if sourceFilter != "" {
		countQuery = "SELECT COUNT(*) FROM articles WHERE source_name = ?"
		dataQuery = "SELECT id, title, description, content, image_url, link, author, source_name, source_feed_url, timestamp, ts FROM articles WHERE source_name = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?"
		args = []interface{}{sourceFilter, pageSize, offset}
	} else {
		countQuery = "SELECT COUNT(*) FROM articles"
		dataQuery = "SELECT id, title, description, content, image_url, link, author, source_name, source_feed_url, timestamp, ts FROM articles ORDER BY timestamp DESC LIMIT ? OFFSET ?"
		args = []interface{}{pageSize, offset}
	}

	var total int
	countArgs := []interface{}{}
	if sourceFilter != "" {
		countArgs = append(countArgs, sourceFilter)
	}
	err := db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var articles []domain.Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, 0, err
		}
		articles = append(articles, *a)
	}
	return articles, total, nil
}

func DeleteArticle(ctx context.Context, id string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM articles WHERE id=?", id)
	if err != nil {
		return err
	}
	mu.Lock()
	delete(articleCache, id)
	mu.Unlock()
	return nil
}

func DeleteArticlesBySource(ctx context.Context, sourceName string) (int64, error) {
	result, err := db.ExecContext(ctx, "DELETE FROM articles WHERE source_name=?", sourceName)
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	mu.Lock()
	for key, article := range articleCache {
		if article.Source.Name == sourceName {
			delete(articleCache, key)
		}
	}
	mu.Unlock()
	return n, nil
}

func GetDistinctSourceNames(ctx context.Context) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT DISTINCT source_name FROM articles ORDER BY source_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// ===== Edition operations =====

func GetEditionForTime(ctx context.Context, t time.Time, allowRecent bool) (*domain.Edition, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, name, date, start_time, end_time, created_at, article_ids, categories, metadata FROM editions ORDER BY end_time DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []*domain.Edition
	var maxEdition *domain.Edition

	for rows.Next() {
		se := storedEdition{}
		var articleIDsJSON, catsJSON, metaJSON string
		if err := rows.Scan(&se.ID, &se.Name, &se.Date, &se.StartTime, &se.EndTime, &se.Created,
			&articleIDsJSON, &catsJSON, &metaJSON); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(articleIDsJSON), &se.Articles)
		json.Unmarshal([]byte(catsJSON), &se.Categories)
		json.Unmarshal([]byte(metaJSON), &se.Metadata)

		if maxEdition == nil || se.EndTime.After(maxEdition.EndTime) {
			maxEdition = &domain.Edition{
				ID: se.ID, Name: se.Name, Date: se.Date,
				StartTime: se.StartTime, EndTime: se.EndTime, Created: se.Created,
				Categories: se.Categories, Metadata: se.Metadata,
			}
		}
		if se.EndTime.After(t) {
			e, err := editionFromStored(ctx, se)
			if err != nil {
				return nil, err
			}
			candidates = append(candidates, e)
		}
	}

	if len(candidates) == 0 {
		if maxEdition != nil && maxEdition.ID != "" && allowRecent {
			se := storedEdition{
				ID: maxEdition.ID, Name: maxEdition.Name, Date: maxEdition.Date,
				StartTime: maxEdition.StartTime, EndTime: maxEdition.EndTime, Created: maxEdition.Created,
				Categories: maxEdition.Categories, Metadata: maxEdition.Metadata,
			}
			var articleIDsJSON string
			db.QueryRowContext(ctx, "SELECT article_ids FROM editions WHERE id=?", maxEdition.ID).Scan(&articleIDsJSON)
			json.Unmarshal([]byte(articleIDsJSON), &se.Articles)
			return editionFromStored(ctx, se)
		}
		return nil, nil
	}

	selected := candidates[0]
	for _, e := range candidates[1:] {
		if e.Created.After(selected.Created) {
			selected = e
		}
	}
	return selected, nil
}

func SetEdition(ctx context.Context, e *domain.Edition) error {
	articleIDs, _ := json.Marshal(getArticleIDs(e.Articles))
	cats, _ := json.Marshal(e.Categories)
	meta, _ := json.Marshal(e.Metadata)

	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO editions (id, name, date, start_time, end_time, created_at, article_ids, categories, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Name, e.Date, e.StartTime, e.EndTime, e.Created,
		string(articleIDs), string(cats), string(meta),
	)
	return err
}

// ===== Helper types =====

type storedEdition struct {
	ID         string
	Name       string
	Date       string
	StartTime  time.Time
	EndTime    time.Time
	Created    time.Time
	Articles   []string
	Categories []string
	Metadata   map[string]string
}

func editionFromStored(ctx context.Context, se storedEdition) (*domain.Edition, error) {
	e := &domain.Edition{
		ID:         se.ID,
		Name:       se.Name,
		Date:       se.Date,
		StartTime:  se.StartTime,
		EndTime:    se.EndTime,
		Created:    se.Created,
		Categories: se.Categories,
		Metadata:   se.Metadata,
	}

	sources, err := GetSources(ctx)
	if err != nil {
		return nil, err
	}
	e.Sources = sources

	for _, id := range se.Articles {
		a, err := GetArticle(ctx, id)
		if err != nil {
			continue
		}
		e.Articles = append(e.Articles, *a)
	}
	return e, nil
}

func getArticleIDs(articles []domain.Article) []string {
	var ids []string
	for _, a := range articles {
		ids = append(ids, a.ID)
	}
	return ids
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}



func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

func GetDB() *sql.DB {
	return db
}

// GetAllArticles returns the most recent articles regardless of time range
func GetAllArticles(ctx context.Context, limit int) ([]domain.Article, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, title, description, content, image_url, link, author, source_name, source_feed_url, timestamp, ts FROM articles ORDER BY timestamp DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []domain.Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		articles = append(articles, *a)
	}
	return articles, nil
}

// Session management functions
func CreateSession(ctx context.Context, token string, expiresAt time.Time) error {
	_, err := db.ExecContext(ctx,
		"INSERT INTO sessions (token, expires_at) VALUES (?, ?)",
		token, expiresAt,
	)
	return err
}

func GetSession(ctx context.Context, token string) (time.Time, error) {
	var expiresAt time.Time
	err := db.QueryRowContext(ctx,
		"SELECT expires_at FROM sessions WHERE token = ?",
		token,
	).Scan(&expiresAt)
	return expiresAt, err
}

func DeleteSession(ctx context.Context, token string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
	return err
}

func CleanupExpiredSessions(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < ?", time.Now())
	return err
}