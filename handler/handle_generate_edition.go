package handler

import (
	"net/http"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/arussellsaw/news/dao"
	"github.com/arussellsaw/news/domain"
)

func handleGenerateEdition(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	e, err := dao.GetEditionForTime(ctx, time.Now(), false)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if e != nil && r.URL.Query().Get("force") == "" {
		w.Write([]byte(e.ID))
		return
	}

	// Get sources from DB
	sources, err := dao.GetSources(ctx)
	if err != nil {
		http.Error(w, "Error getting sources: "+err.Error(), 500)
		return
	}

	e, err = domain.NewEdition(ctx, time.Now(), sources)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	start := time.Now().Add(-120 * time.Hour)
	end := time.Now()
	articles, err := dao.GetArticlesByTime(ctx, start, end)
	if err != nil {
		httpError(ctx, w, "error getting articles", err)
		return
	}

	// Filter invalid UTF-8
	newArticles := []domain.Article{}
L:
	for _, a := range articles {
		for _, el := range a.Content {
			if !utf8.Valid([]byte(el.Value)) {
				continue L
			}
		}
		newArticles = append(newArticles, a)
	}

	// Round-robin by source
	bySource := make(map[string][]domain.Article)
	for _, a := range newArticles {
		bySource[a.Source.Name] = append(bySource[a.Source.Name], a)
	}
	for _, as := range bySource {
		sort.Slice(as, func(i, j int) bool {
			return as[i].Timestamp.After(as[j].Timestamp)
		})
	}
	newArticles = nil
top:
	for s, as := range bySource {
		newArticles = append(newArticles, as[0])
		bySource[s] = as[1:]
		if len(bySource[s]) == 0 {
			delete(bySource, s)
			goto top
		}
	}
	if len(bySource) != 0 {
		goto top
	}

	e.Articles = newArticles

	err = dao.SetEdition(ctx, e)
	if err != nil {
		httpError(ctx, w, "Error storing edition", err)
		return
	}
	w.Write([]byte(e.ID))
}
