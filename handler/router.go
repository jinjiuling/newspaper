package handler

import (
	"compress/gzip"
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"

	"github.com/arussellsaw/news/domain"
)

var (
	jar, _ = cookiejar.New(&cookiejar.Options{})
	c      = http.Client{
		Jar: jar,
	}

	// Pre-compiled templates
	newsTmpl    *template.Template
	articleTmpl *template.Template
)

func initTemplates() error {
	var err error
	newsTmpl = template.New("frame.html").Funcs(template.FuncMap{
		"imgURL": func(src domain.Source, rawURL string) template.URL {
			if src.DisableImgProxy {
				return template.URL(rawURL)
			}
			return template.URL("/img?url=" + url.QueryEscape(rawURL) + "&src=" + fmt.Sprint(src.ID))
		},
	})
	newsTmpl, err = newsTmpl.ParseFiles(
		"tmpl/frame.html",
		"tmpl/frontpage.html",
		"tmpl/section.html",
		"tmpl/article-tile.html",
	)
	if err != nil {
		return err
	}

	articleTmpl = template.New("frame.html").Funcs(template.FuncMap{
		"minus":   TemplateFuncs["minus"],
		"plus":    TemplateFuncs["plus"],
		"safeHTML": TemplateFuncs["safeHTML"],
		"imgURL": func(src domain.Source, rawURL string) template.URL {
			if src.DisableImgProxy {
				return template.URL(rawURL)
			}
			return template.URL("/img?url=" + url.QueryEscape(rawURL) + "&src=" + fmt.Sprint(src.ID))
		},
	})
	articleTmpl, err = articleTmpl.ParseFiles(
		"tmpl/frame.html",
		"tmpl/article.html",
	)
	if err != nil {
		return err
	}
	return nil
}

// gzipResponseWriter wraps http.ResponseWriter to compress writes
type gzipResponseWriter struct {
	http.Flusher
	http.ResponseWriter
	io.Closer
	Writer io.Writer
}

func (g gzipResponseWriter) Write(b []byte) (int, error) {
	return g.Writer.Write(b)
}

func (g gzipResponseWriter) Flush() {
	if g.Flusher != nil {
		g.Flusher.Flush()
	}
}

var gzipPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression)
		return w
	},
}

func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		next.ServeHTTP(w, r)
		return
		}

		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)
		defer func() {
			gz.Close()
			gzipPool.Put(gz)
		}()

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")

		if f, ok := w.(http.Flusher); ok {
			next.ServeHTTP(gzipResponseWriter{ResponseWriter: w, Flusher: f, Closer: gz, Writer: gz}, r)
		} else {
			next.ServeHTTP(gzipResponseWriter{ResponseWriter: w, Closer: gz, Writer: gz}, r)
		}
	})
}

// cachedFileServer wraps http.FileServer with Cache-Control headers
type cachedFileServer struct {
	h http.Handler
}

func (cfs cachedFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=604800")
	cfs.h.ServeHTTP(w, r)
}

func Init(ctx context.Context) http.Handler {
	if err := initTemplates(); err != nil {
		panic("failed to parse templates: " + err.Error())
	}

	m := http.NewServeMux()

	// Static files with cache headers
	m.Handle("/static/", http.StripPrefix("/static/", cachedFileServer{http.FileServer(http.Dir("static/"))}))

	// Image proxy
	m.HandleFunc("/img", handleImg)

	// Public pages
	m.HandleFunc("/article", handleArticle)
	m.HandleFunc("/image", handleDitherImage)
	m.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	m.HandleFunc("/barcode", handleQRCode)
	m.HandleFunc("/generate-edition", handleGenerateEdition)
	m.HandleFunc("/poll", handlePoll)
	m.HandleFunc("/", handleNews)

	// Admin routes
	m.HandleFunc("/admin/login", handleAdminLogin)
	m.HandleFunc("/admin/logout", handleAdminLogout)
	m.HandleFunc("/admin/sources", requireAuth(handleAdminSources))
	m.HandleFunc("/admin/sources/add", requireAuth(handleAdminSourceAdd))
	m.HandleFunc("/admin/sources/edit", requireAuth(handleAdminSourceEdit))
	m.HandleFunc("/admin/sources/delete", requireAuth(handleAdminSourceDelete))
	m.HandleFunc("/admin/sources/toggle", requireAuth(handleAdminSourceToggle))
	m.HandleFunc("/admin/articles", requireAuth(handleAdminArticles))
	m.HandleFunc("/admin/articles/delete", requireAuth(handleAdminArticleDelete))
	m.HandleFunc("/admin/articles/delete-source", requireAuth(handleAdminDeleteBySource))
	m.HandleFunc("/admin/poll", requireAuth(handleAdminPoll))
	m.HandleFunc("/admin/", requireAuth(handleAdminDashboard))

	return gzipMiddleware(m)
}
