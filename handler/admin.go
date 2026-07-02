package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/arussellsaw/news/dao"
	"github.com/arussellsaw/news/domain"
	"golang.org/x/crypto/bcrypt"
)

// Pre-compiled admin templates
var (
	adminLoginTmpl         *template.Template
	adminDashboardTmpl     *template.Template
	adminSourcesTmpl       *template.Template
	adminSourceFormTmpl    *template.Template
	adminArticlesTmpl      *template.Template
)

func init() {
	var err error
	adminLoginTmpl, err = template.ParseFiles("tmpl/admin_login.html")
	if err != nil {
		log.Printf("Warning: failed to parse admin_login.html: %v", err)
	}
	adminDashboardTmpl, err = template.ParseFiles("tmpl/admin_dashboard.html")
	if err != nil {
		log.Printf("Warning: failed to parse admin_dashboard.html: %v", err)
	}
	adminSourcesTmpl, err = template.ParseFiles("tmpl/admin_sources.html")
	if err != nil {
		log.Printf("Warning: failed to parse admin_sources.html: %v", err)
	}
	adminSourceFormTmpl, err = template.ParseFiles("tmpl/admin_source_form.html")
	if err != nil {
		log.Printf("Warning: failed to parse admin_source_form.html: %v", err)
	}
	adminArticlesTmpl, err = template.New("admin_articles.html").Funcs(TemplateFuncs).ParseFiles("tmpl/admin_articles.html")
	if err != nil {
		log.Printf("Warning: failed to parse admin_articles.html: %v", err)
	}
}

// Session management

func getSessionToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Printf("Warning: failed to generate random token: %v", err)
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func createSession(w http.ResponseWriter) {
	token := getSessionToken()
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := dao.CreateSession(context.Background(), token, expiresAt); err != nil {
		log.Printf("Error creating session: %v", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    token,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func isValidSession(r *http.Request) bool {
	cookie, err := r.Cookie("admin_session")
	if err != nil {
		return false
	}
	expiresAt, err := dao.GetSession(context.Background(), cookie.Value)
	if err != nil {
		return false
	}
	if time.Now().After(expiresAt) {
		dao.DeleteSession(context.Background(), cookie.Value)
		return false
	}
	return true
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isValidSession(r) {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

func checkPassword(password string) bool {
	expected := os.Getenv("ADMIN_PASSWORD")
	if expected == "" {
		return false
	}
	// 如果是 bcrypt hash，用 bcrypt 校验
	if len(expected) > 0 && expected[0] == '$' {
		return bcrypt.CompareHashAndPassword([]byte(expected), []byte(password)) == nil
	}
	// 否则明文比较
	return password == expected
}

// ===== Login/Logout =====

func handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		execHTML(w, adminLoginTmpl, nil)
		return
	}

	r.ParseForm()
	password := r.FormValue("password")
	if !checkPassword(password) {
		execHTML(w, adminLoginTmpl, map[string]string{"Error": "密码错误"})
		return
	}

	createSession(w)
	http.Redirect(w, r, "/admin/", http.StatusFound)
}

func handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("admin_session")
	if err == nil {
		dao.DeleteSession(context.Background(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "admin_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/admin/login", http.StatusFound)
}

// ===== Dashboard =====

func handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sources, _ := dao.GetAllSources(ctx)
	articleCount := 0
	db := dao.GetDB()
	if db != nil {
		db.QueryRow("SELECT COUNT(*) FROM articles").Scan(&articleCount)
	}

	data := struct {
		Sources      []domain.Source
		ArticleCount int
		CurrentTime  string
	}{
		Sources:      sources,
		ArticleCount: articleCount,
		CurrentTime:  time.Now().Format("2006-01-02 15:04:05"),
	}

	execHTML(w, adminDashboardTmpl, data)
}

// ===== Source Management =====

func handleAdminSources(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sources, _ := dao.GetAllSources(ctx)
	execHTML(w, adminSourcesTmpl, sources)
}

func handleAdminSourceAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		execHTML(w, adminSourceFormTmpl, map[string]interface{}{"Action": "add"})
		return
	}

	r.ParseForm()
	enabled := r.FormValue("enabled") == "on"
	disableImgProxy := r.FormValue("disable_img_proxy") == "on"
	s := &domain.Source{
		Name:            r.FormValue("name"),
		URL:             r.FormValue("url"),
		FeedURL:         r.FormValue("feed_url"),
		Categories:      []string{r.FormValue("category")},
		Enabled:         enabled,
		DisableImgProxy: disableImgProxy,
	}

	// Parse image headers (JSON)
	if hdrs := r.FormValue("image_headers"); hdrs != "" {
		var h map[string]string
		if json.Unmarshal([]byte(hdrs), &h) == nil {
			s.ImageHeaders = h
		}
	} else {
		// Default: use source URL as Referer
		s.ImageHeaders = map[string]string{"Referer": s.URL + "/"}
	}

	ctx := r.Context()
	if err := dao.AddSource(ctx, s); err != nil {
		execHTML(w, adminSourceFormTmpl, map[string]interface{}{"Action": "add", "Error": err.Error(), "Source": s})
		return
	}

	http.Redirect(w, r, "/admin/sources", http.StatusFound)
}

func handleAdminSourceEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method == "GET" {
		id, _ := strconv.Atoi(r.URL.Query().Get("id"))
		s, err := dao.GetSourceByID(ctx, id)
		if err != nil {
			http.Error(w, "Source not found", 404)
			return
		}
		execHTML(w, adminSourceFormTmpl, map[string]interface{}{"Action": "edit", "Source": s})
		return
	}

	r.ParseForm()
	id, _ := strconv.Atoi(r.FormValue("id"))
	enabled := r.FormValue("enabled") == "on"
	disableImgProxy := r.FormValue("disable_img_proxy") == "on"
	s := &domain.Source{
		ID:              id,
		Name:            r.FormValue("name"),
		URL:             r.FormValue("url"),
		FeedURL:         r.FormValue("feed_url"),
		Categories:      []string{r.FormValue("category")},
		Enabled:         enabled,
		DisableImgProxy: disableImgProxy,
	}

	// Parse image headers (JSON)
	if hdrs := r.FormValue("image_headers"); hdrs != "" {
		var h map[string]string
		if json.Unmarshal([]byte(hdrs), &h) == nil {
			s.ImageHeaders = h
		}
	} else {
		s.ImageHeaders = map[string]string{"Referer": s.URL + "/"}
	}

	if err := dao.UpdateSource(ctx, s); err != nil {
		execHTML(w, adminSourceFormTmpl, map[string]interface{}{"Action": "edit", "Error": err.Error(), "Source": s})
		return
	}

	http.Redirect(w, r, "/admin/sources", http.StatusFound)
}

func handleAdminSourceDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	r.ParseForm()
	id, _ := strconv.Atoi(r.FormValue("id"))
	ctx := r.Context()

	if err := dao.DeleteSource(ctx, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	http.Redirect(w, r, "/admin/sources", http.StatusFound)
}

func handleAdminSourceToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	r.ParseForm()
	id, _ := strconv.Atoi(r.FormValue("id"))
	ctx := r.Context()

	s, err := dao.GetSourceByID(ctx, id)
	if err != nil {
		http.Error(w, "Source not found", 404)
		return
	}

	s.Enabled = !s.Enabled
	if err := dao.UpdateSource(ctx, s); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	http.Redirect(w, r, "/admin/sources", http.StatusFound)
}

// ===== Article Management =====

func handleAdminArticles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := 20

	sourceFilter := r.URL.Query().Get("source")

	articles, total, _ := dao.GetArticles(ctx, page, pageSize, sourceFilter)
	sourceNames, _ := dao.GetDistinctSourceNames(ctx)

	totalPages := (total + pageSize - 1) / pageSize

	data := struct {
		Articles     []domain.Article
		SourceNames  []string
		CurrentPage  int
		TotalPages   int
		Total        int
		SourceFilter string
	}{
		Articles:     articles,
		SourceNames:  sourceNames,
		CurrentPage:  page,
		TotalPages:   totalPages,
		Total:        total,
		SourceFilter: sourceFilter,
	}

	execHTML(w, adminArticlesTmpl, data)
}

func handleAdminArticleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	r.ParseForm()
	id := r.FormValue("id")
	ctx := r.Context()

	if err := dao.DeleteArticle(ctx, id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	redirect := r.FormValue("redirect")
	if redirect == "" {
		redirect = "/admin/articles"
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

func handleAdminDeleteBySource(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	r.ParseForm()
	sourceName := r.FormValue("source_name")
	ctx := r.Context()

	n, err := dao.DeleteArticlesBySource(ctx, sourceName)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	log.Printf("Deleted %d articles from source: %s", n, sourceName)
	http.Redirect(w, r, "/admin/articles", http.StatusFound)
}

// ===== Poll =====

func handleAdminPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	ctx := r.Context()
	sources, err := dao.GetSources(ctx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	go pollAllSources(ctx, sources)

	http.Redirect(w, r, "/admin/", http.StatusFound)
}
