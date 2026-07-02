package handler

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/arussellsaw/news/dao"
	"github.com/arussellsaw/news/domain"
	"github.com/arussellsaw/news/idgen"
	"github.com/mmcdole/gofeed"
)

func handlePoll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sources, err := dao.GetSources(ctx)
	if err != nil {
		httpError(ctx, w, "Error getting sources", err)
		return
	}

	// Use a detached context so polling survives after the HTTP response is sent.
	// r.Context() is canceled the moment handlePoll returns, which would kill
	// all downstream DB queries and article checks.
	pollCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	go func() {
		defer cancel()
		pollAllSources(pollCtx, sources)
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Polling started"))
}

func pollAllSources(ctx context.Context, sources []domain.Source) {
	for _, s := range sources {
		log.Printf("Polling source: %s (%s)", s.Name, s.FeedURL)
		err := pollSource(ctx, s)
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("Polling canceled for %s: %v", s.Name, ctx.Err())
				return
			}
			log.Printf("Error polling %s: %v", s.Name, err)
		}
	}
}

func pollSource(ctx context.Context, s domain.Source) error {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(s.FeedURL, ctx)
	if err != nil {
		return err
	}

	for _, item := range feed.Items {
		link := strings.TrimSpace(item.Link)
		if link == "" {
			continue
		}

		// Check if article already exists
		existing, err := dao.GetArticleByURL(ctx, link)
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("Context canceled during article check, stopping poll")
				return ctx.Err()
			}
			log.Printf("Error checking existing article: %v", err)
			continue
		}
		if existing != nil {
			continue
		}

		// Extract full content if possible
		description := item.Description
		imageURL := ""
		if item.Image != nil {
			imageURL = item.Image.URL
		}
		authorName := ""
		if item.Author != nil {
			authorName = item.Author.Name
		}

		// Try to fetch full article content
		//content := extractArticleContent(ctx, link)
		//if content == "" {
		//	content = description
		//}
        
		// 使用通用提取器获取最丰满的正文内容，兼容 RSS/Atom 及非标扩展字段
		content := extractUniversalContent(item)
        
		ts := time.Now()
		if item.PublishedParsed != nil {
			ts = *item.PublishedParsed
		}

		a := domain.Article{
			ID:          idgen.New("art"),
			Title:       strings.TrimSpace(item.Title),
			Description: description,
			Content:     toElements(content, "\n"),
			ImageURL:    imageURL,
			Link:        link,
			Author:      authorName,
			Source:      s,
			Timestamp:   ts,
			TS:          ts.Format("Mon Jan 2 15:04"),
		}

		if err := dao.SetArticle(ctx, &a); err != nil {
			log.Printf("Error storing article %s: %v", link, err)
			continue
		}
		log.Printf("Stored article: %s - %s", a.ID, a.Title)
	}
	return nil
}

// extractUniversalContent 通用正文提取器，抹平各大网站 RSS 协议的字段差异
func extractUniversalContent(item *gofeed.Item) string {
	// 优先级 1：Content 字段 (通常对应 RSS 的 <content:encoded> 或 Atom 的 <content>)
	if content := strings.TrimSpace(item.Content); content != "" {
		return content
	}

	// 优先级 2：部分非标 RSS 会把正文强行塞进自定义的 Extension 扩展字段里
	if ext, ok := item.Extensions["content"]; ok {
		if encoded, ok := ext["encoded"]; ok && len(encoded) > 0 {
			if content := strings.TrimSpace(encoded[0].Value); content != "" {
				return content
			}
		}
	}

	// 优先级 3：Description 字段 (通常对应 RSS 的 <description> 或 Atom 的 <summary>)
	if desc := strings.TrimSpace(item.Description); desc != "" {
		return desc
	}

	// 优先级 4：兜底方案，如果啥都没有只能返回标题
	return item.Title
}

func extractArticleContent(ctx context.Context, url string) string {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ""
	}

	// Remove script and style elements
	doc.Find("script, style, nav, header, footer, .ad, .advertisement, .sidebar").Remove()

	// Try common article content selectors
	selectors := []string{
		"article",
		".article-content",
		".article-body",
		".post-content",
		".entry-content",
		".content-article",
		"#article-content",
		".story-body",
		"main",
	}

	for _, sel := range selectors {
		if content := doc.Find(sel).First(); content.Length() > 0 {
			text := strings.TrimSpace(content.Text())
			if len(text) > 100 {
				return text
			}
		}
	}

	// Fallback: get body text
	text := strings.TrimSpace(doc.Find("body").Text())
	if len(text) > 500 {
		// Truncate very long body text
		return text[:2000]
	}
	return text
}

func toElements(s, br string) []domain.Element {
	var (
		lines = strings.Split(s, br)
		out   []domain.Element
		row   string
	)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if row != "" {
				out = append(out, domain.Element{Type: "text", Value: row})
				row = ""
			}
			continue
		}
		if row != "" {
			row += " "
		}
		row += line
	}
	if row != "" {
		out = append(out, domain.Element{Type: "text", Value: row})
	}
	if len(out) == 0 {
		out = append(out, domain.Element{Type: "text", Value: s})
	}
	return out
}

func httpError(ctx context.Context, w http.ResponseWriter, msg string, err error) {
	log.Printf("%s: %v", msg, err)
	http.Error(w, err.Error(), 500)
}