package handler

import (
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/arussellsaw/news/dao"
)

func handleImg(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "missing url param", 400)
		return
	}

	// Build request to source with custom headers
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		http.Error(w, "bad url", 400)
		return
	}

	// Set default headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Apply source-specific image headers if source ID provided
	if srcIDStr := r.URL.Query().Get("src"); srcIDStr != "" {
		if srcID, err := strconv.Atoi(srcIDStr); err == nil {
			ctx := r.Context()
			if source, err := dao.GetSourceByID(ctx, srcID); err == nil && source.ImageHeaders != nil {
				for k, v := range source.ImageHeaders {
					req.Header.Set(k, v)
				}
			}
		}
	}

	// Default Referer from source URL if none set
	if req.Header.Get("Referer") == "" {
		req.Header.Set("Referer", "https://www.google.com/")
	}

	resp, err := c.Do(req)
	if err != nil {
		log.Printf("Error fetching image: %v", err)
		http.Error(w, err.Error(), 502)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "upstream returned "+resp.Status, resp.StatusCode)
		return
	}

	// Forward content type from upstream
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=2592000")

	io.Copy(w, resp.Body)
}
