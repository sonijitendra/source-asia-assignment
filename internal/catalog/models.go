package catalog

import "time"

// Product holds lightweight metadata. Media URLs are stored separately
// so that list queries never touch the heavy arrays.
type Product struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	SKU          string    `json:"sku"`
	ImageCount   int       `json:"image_count"`
	VideoCount   int       `json:"video_count"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Media stores the actual URL slices, keyed by the same product ID
// but in a separate map so list queries skip them entirely.
type Media struct {
	ImageURLs []string `json:"image_urls"`
	VideoURLs []string `json:"video_urls"`
}

// ProductDetail is the merged view returned by the detail endpoint.
type ProductDetail struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	SKU          string    `json:"sku"`
	ImageURLs    []string  `json:"image_urls"`
	VideoURLs    []string  `json:"video_urls"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// --- Request / Response shapes -----------------------------------------------

type CreateRequest struct {
	Name      string   `json:"name"`
	SKU       string   `json:"sku"`
	ImageURLs []string `json:"image_urls"`
	VideoURLs []string `json:"video_urls"`
}

type AddMediaRequest struct {
	ImageURLs []string `json:"image_urls"`
	VideoURLs []string `json:"video_urls"`
}

type PaginationMeta struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}
