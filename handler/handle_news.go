package handler

import (
	"html"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/arussellsaw/news/dao"
	"github.com/arussellsaw/news/domain"
)

// Pre-compiled regex for HTML tag removal
var htmlTagRe = regexp.MustCompile(`(<\/?[a-zA-Z]+?[^>]*\/?>)*`)

func handleNews(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get sources from DB
	sources, err := dao.GetSources(ctx)
	if err != nil {
		http.Error(w, "Error getting sources: "+err.Error(), 500)
		return
	}

	// Load recent articles from all sources
	articles, err := dao.GetAllArticles(ctx, 201)
	if err != nil {
		http.Error(w, "Error getting articles: "+err.Error(), 500)
		return
	}

	if len(articles) == 0 {
		log.Println("No articles found in database")
	}

	// Build edition-like structure for template
	e, _ := domain.NewEdition(ctx, time.Now(), sources)

	// Filter by category
	cat := r.URL.Query().Get("cat")
	if cat != "" {
		newArticles := []domain.Article{}
	articles:
		for _, a := range articles {
			for _, c := range a.Source.Categories {
				if c == cat {
					newArticles = append(newArticles, a)
					continue articles
				}
			}
		}
		articles = newArticles
	}

	// Filter by source
	src := r.URL.Query().Get("src")
	if src != "" {
		newArticles := []domain.Article{}
		for _, a := range articles {
			if a.Source.Name == src {
				newArticles = append(newArticles, a)
			}
		}
		articles = newArticles
	}

	// Clean HTML from descriptions and content
	for i := range articles {
		articles[i].Description = removeHTMLTag(articles[i].Description)
		for j := range articles[i].Content {
			articles[i].Content[j].Value = html.UnescapeString(removeHTMLTag(articles[i].Content[j].Value))
		}
	}

	e.Articles = articles
	e.ComputeSections()

	err = execHTML(w, newsTmpl, e)
	if err != nil {
		http.Error(w, "Error executing template: "+err.Error(), 500)
		return
	}
}

func removeHTMLTag(in string) string {
	groups := htmlTagRe.FindAllString(in, -1)
	sort.Slice(groups, func(i, j int) bool {
		return len(groups[i]) > len(groups[j])
	})
	for _, group := range groups {
		if strings.TrimSpace(group) != "" {
			in = strings.ReplaceAll(in, group, "")
		}
	}
	return in
}
