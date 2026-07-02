package domain

import (
	"context"
	"time"

	"github.com/arussellsaw/news/idgen"
)

var (
	morningEdition time.Duration = 6 * time.Hour
	eveningEdition time.Duration = 17 * time.Hour
)

// Section holds a pre-allocated set of articles for a named layout section.
// Articles are ordered to match the slot positions in the template (slot 0, slot 1, ...).
// Templates access them via index .Articles 0, .Articles 1, etc.
type Section struct {
	Name     string
	Articles []Article
}

type Edition struct {
	ID         string
	Name       string
	Sources    []Source
	Articles   []Article
	Categories []string
	Date       string

	StartTime time.Time
	EndTime   time.Time
	Created   time.Time

	Metadata map[string]string

	// Sections holds the pre-computed sections with articles already assigned.
	// Replaces the old SectionNames + GetArticle approach.
	Sections []Section

	Article    Article
}

// sectionSlotSpec defines the ordered requirements for each section.
// Each entry is {minContentChars, needImg}.
// Articles are assigned in slot order — the first matching article goes to slot 0, etc.
var sectionSlotSpec = map[string][]struct {
	minSize int
	needImg bool
}{
	"section-1": {
		{3000, true},  // biggest-article
		{400, false},  // small-article
		{400, false},  // small-article
		{3000, false}, // medium-article
	},
	"section-2": {
		{3000, false}, // medium-article
		{3000, false}, // medium-article
		{0, false},    // small-article x6
		{0, false},
		{0, false},
		{0, false},
		{0, false},
		{0, false},
	},
	"section-3": {
		{2000, false}, // article
		{1100, true},  // article (with image)
		{2000, false}, // article
		{2000, false}, // article
	},
	"section-4": {
		{400, false},  // small-article
		{400, false},  // small-article
		{1000, false}, // article
		{1500, true},  // big-article
		{1300, false}, // small-article
	},
	"section-5": {
		{400, false}, // article x3
		{400, false},
		{400, false},
	},
	"section-6": {
		{400, false}, // small-article x6
		{400, false},
		{400, false},
		{400, false},
		{400, false},
		{400, false},
	},
	"section-7": {
		{1500, true}, // big-article
		{1500, true}, // big-article
	},
	"section-8": {
		{400, false},  // small-article
		{400, false},  // small-article
		{1000, false}, // article
		{1500, true},  // big-article
		{1300, false}, // small-article
	},
}

var patternOrder = []string{
	"section-1", "section-2", "section-3", "section-4",
	"section-5", "section-6", "section-6", "section-7",
	"section-3", "section-8", "section-2",
}

// ComputeSections pre-allocates articles into section slots.
// Each section's Articles slice is ordered to match the template's slot positions.
// Sections that can't be fully filled are dropped, so no empty sections are rendered.
func (e *Edition) ComputeSections() {
	// Bucket articles by content size
	byBucket := make([][]Article, 6)
	for _, a := range e.Articles {
		b := bucketForSize(a.Size())
		byBucket[b] = append(byBucket[b], a)
	}

	claimed := make(map[string]bool)
	e.Sections = nil

	for _, name := range patternOrder {
		spec := sectionSlotSpec[name]
		slots := make([]Article, 0, len(spec))
		ok := true

		for _, req := range spec {
			a := pickArticle(byBucket, claimed, req.minSize, req.needImg)
			if a == nil {
				ok = false
				break
			}
			slots = append(slots, *a)
		}

		if !ok || len(slots) == 0 {
			break // not enough articles — skip remaining sections
		}

		e.Sections = append(e.Sections, Section{Name: name, Articles: slots})
	}
}

// bucketForSize maps content length to a size bucket index.
// Buckets: 0:<200, 1:200-499, 2:500-2999, 3:3000-4999, 4:5000-6999, 5:>=7000
func bucketForSize(s int) int {
	switch {
	case s < 200:
		return 0
	case s < 500:
		return 1
	case s < 3000:
		return 2
	case s < 5000:
		return 3
	case s < 7000:
		return 4
	default:
		return 5
	}
}

// pickArticle finds and claims the best-fit article for a slot requirement.
// It searches from the most specific bucket down to the least, checking
// each bucket only if the minSize requirement can be satisfied.
func pickArticle(byBucket [][]Article, claimed map[string]bool, minSize int, needImg bool) *Article {
	// Order of buckets to try, from best-fit to smallest.
	// minSize determines which buckets are eligible.
	var order []int
	switch {
	case minSize >= 5000:
		order = []int{5, 4, 3}
	case minSize >= 3000:
		order = []int{5, 4, 3}
	case minSize >= 1500:
		order = []int{5, 4, 3, 2}
	case minSize >= 1000:
		order = []int{5, 4, 3, 2}
	case minSize >= 500:
		order = []int{5, 4, 3, 2}
	case minSize >= 400:
		order = []int{5, 4, 3, 2, 1}
	case minSize > 0:
		order = []int{5, 4, 3, 2, 1, 0}
	default:
		order = []int{5, 4, 3, 2, 1, 0}
	}

	for _, b := range order {
		if b < len(byBucket) && len(byBucket[b]) > 0 && bucketMinSize(b) >= minSize {
			for i := range byBucket[b] {
				a := &byBucket[b][i]
				if claimed[a.ID] {
					continue
				}
				if needImg && a.ImageURL == "" {
					continue
				}
				claimed[a.ID] = true
				return a
			}
		}
	}

	// Fallback: linear scan across all buckets for any unclaimed article
	for b := 5; b >= 0; b-- {
		if b >= len(byBucket) {
			continue
		}
		for i := range byBucket[b] {
			a := &byBucket[b][i]
			if claimed[a.ID] {
				continue
			}
			if needImg && a.ImageURL == "" {
				continue
			}
			claimed[a.ID] = true
			return a
		}
	}

	return nil
}

// bucketMinSize returns the minimum content length for a given bucket index.
func bucketMinSize(b int) int {
	switch b {
	case 0:
		return 0
	case 1:
		return 200
	case 2:
		return 500
	case 3:
		return 3000
	case 4:
		return 5000
	case 5:
		return 7000
	default:
		return 0
	}
}

func NewEdition(ctx context.Context, now time.Time, sources []Source) (*Edition, error) {
	morning := now.Truncate(24 * time.Hour).Add(morningEdition)
	evening := now.Truncate(24 * time.Hour).Add(eveningEdition)

	catMap := make(map[string]struct{})
	for _, s := range sources {
		for _, c := range s.Categories {
			catMap[c] = struct{}{}
		}
	}
	cats := make([]string, 0, len(catMap))
	for c := range catMap {
		cats = append(cats, c)
	}

	e := Edition{
		ID:         idgen.New("edt"),
		Sources:    sources,
		Categories: cats,
		Created:    time.Now(),
	}

	e.Date = time.Now().Format("Monday January 02 2006")

	switch {
	case now.After(morning) && now.Before(evening):
		e.StartTime = morning
		e.EndTime = evening
		e.Name = "Morning Edition"
	case now.Before(morning) || now.After(evening):
		e.StartTime = evening
		e.EndTime = morning.Add(24 * time.Hour)
		e.Name = "Evening Edition"
	}

	return &e, nil
}
