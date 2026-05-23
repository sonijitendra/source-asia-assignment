package catalog

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrDuplicateSKU = errors.New("sku already exists")
	ErrNotFound     = errors.New("product not found")
)

// Store is a concurrency-safe, in-memory product catalog.
//
// Data layout (the key design decision):
//
//	products  map[string]*Product   — lightweight metadata only
//	media     map[string]*Media     — heavy URL arrays, same key
//	skuIdx    map[string]string     — sku → product-id for O(1) uniqueness check
//
// GET /products iterates only over `products`, never touching `media`.
// GET /products/:id merges both maps for the full detail view.
type Store struct {
	mu       sync.RWMutex
	products map[string]*Product
	media    map[string]*Media
	skuIdx   map[string]string
	seq      int64
}

func NewStore() *Store {
	return &Store{
		products: make(map[string]*Product),
		media:    make(map[string]*Media),
		skuIdx:   make(map[string]string),
	}
}

// Create adds a new product. SKU uniqueness is checked under the write lock
// so concurrent creates with the same SKU never both succeed.
func (s *Store) Create(req CreateRequest) (*Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, taken := s.skuIdx[req.SKU]; taken {
		return nil, ErrDuplicateSKU
	}

	s.seq++
	id := fmt.Sprintf("p_%d", s.seq)
	now := time.Now()

	thumbnail := ""
	if len(req.ImageURLs) > 0 {
		thumbnail = req.ImageURLs[0]
	}

	p := &Product{
		ID:           id,
		Name:         req.Name,
		SKU:          req.SKU,
		ImageCount:   len(req.ImageURLs),
		VideoCount:   len(req.VideoURLs),
		ThumbnailURL: thumbnail,
		CreatedAt:    now,
	}

	// Defensive copy so the caller can't mutate our internal slices.
	m := &Media{
		ImageURLs: copyStrings(req.ImageURLs),
		VideoURLs: copyStrings(req.VideoURLs),
	}

	s.products[id] = p
	s.media[id] = m
	s.skuIdx[req.SKU] = id

	return p, nil
}

// List returns a page of products (metadata only — no media URLs).
// Products are ordered newest-first for consistent pagination.
func (s *Store) List(page, pageSize int) ([]Product, PaginationMeta) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]Product, 0, len(s.products))
	for _, p := range s.products {
		all = append(all, *p) // copy values, not pointers
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	total := len(all)
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	meta := PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}

	start := (page - 1) * pageSize
	if start >= total {
		return []Product{}, meta
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return all[start:end], meta
}

// GetByID returns the full product detail including all media URLs.
func (s *Store) GetByID(id string) (*ProductDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.products[id]
	if !ok {
		return nil, ErrNotFound
	}

	m := s.media[id]

	return &ProductDetail{
		ID:           p.ID,
		Name:         p.Name,
		SKU:          p.SKU,
		ImageURLs:    copyStrings(m.ImageURLs),
		VideoURLs:    copyStrings(m.VideoURLs),
		ThumbnailURL: p.ThumbnailURL,
		CreatedAt:    p.CreatedAt,
	}, nil
}

// AddMedia appends URLs to an existing product and returns updated metadata.
func (s *Store) AddMedia(id string, req AddMediaRequest) (*Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.products[id]
	if !ok {
		return nil, ErrNotFound
	}

	m := s.media[id]
	m.ImageURLs = append(m.ImageURLs, req.ImageURLs...)
	m.VideoURLs = append(m.VideoURLs, req.VideoURLs...)

	p.ImageCount = len(m.ImageURLs)
	p.VideoCount = len(m.VideoURLs)

	if p.ThumbnailURL == "" && len(m.ImageURLs) > 0 {
		p.ThumbnailURL = m.ImageURLs[0]
	}

	out := *p // return a copy
	return &out, nil
}

// copyStrings returns a non-nil copy of src (so JSON marshals to [] not null).
func copyStrings(src []string) []string {
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}
