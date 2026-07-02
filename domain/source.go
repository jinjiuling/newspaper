package domain

type Source struct {
	ID              int
	Name            string
	URL             string
	FeedURL         string
	Categories      []string
	ImageHeaders    map[string]string // Custom HTTP headers for image fetching (e.g. Referer)
	ForceFetch      bool
	DisableFetch    bool
	Enabled         bool
	DisableImgProxy bool              // true = 该源图片不走代理，直连源站
}
