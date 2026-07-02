package handler

import (
	"net/http"
	"time"

	"github.com/arussellsaw/news/dao"
	"github.com/arussellsaw/news/domain"
)

func handleArticle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get categories from sources
	sources, err := dao.GetSources(ctx)
	if err != nil {
		http.Error(w, "Error getting sources: "+err.Error(), 500)
		return
	}
	catMap := make(map[string]struct{})
	for _, s := range sources {
		for _, c := range s.Categories {
			catMap[c] = struct{}{}
		}
	}
	categories := make([]string, 0, len(catMap))
	for c := range catMap {
		categories = append(categories, c)
	}

	article, err := dao.GetArticle(ctx, r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "Error getting article: "+err.Error(), 500)
		return
	}

	a := articlePage{
		Article:    article,
		Categories: categories,
		Name:       time.Now().Format("Monday January 02 2006"),
		Date:       time.Now().Format("Monday January 02 2006"),
	}
	err = execHTML(w, articleTmpl, a)
	if err != nil {
		http.Error(w, "Error executing template: "+err.Error(), 500)
		return
	}
}

type articlePage struct {
	*domain.Article
	Categories []string
	Name       string
	Date       string
}
