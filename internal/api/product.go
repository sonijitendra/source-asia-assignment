package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"source-asia-assignment/internal/catalog"

	"github.com/gin-gonic/gin"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
	maxURLsPerArray = 20
	maxURLLength    = 2048
)

type ProductHandler struct {
	store *catalog.Store
}

func NewProductHandler(s *catalog.Store) *ProductHandler {
	return &ProductHandler{store: s}
}

// Create is POST /products.
func (h *ProductHandler) Create(c *gin.Context) {
	var req catalog.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.SKU = strings.TrimSpace(req.SKU)

	if req.Name == "" {
		jsonError(c, http.StatusBadRequest, "name is required")
		return
	}
	if req.SKU == "" {
		jsonError(c, http.StatusBadRequest, "sku is required")
		return
	}

	if err := validateMediaArrays(req.ImageURLs, req.VideoURLs); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	product, err := h.store.Create(req)
	if err != nil {
		if err == catalog.ErrDuplicateSKU {
			jsonError(c, http.StatusConflict, "a product with this SKU already exists")
			return
		}
		jsonError(c, http.StatusInternalServerError, "unexpected error")
		return
	}

	jsonOK(c, http.StatusCreated, product)
}

// List is GET /products — returns metadata only, no media URLs.
func (h *ProductHandler) List(c *gin.Context) {
	page := queryInt(c, "page", defaultPage)
	pageSize := queryInt(c, "page_size", defaultPageSize)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	products, meta := h.store.List(page, pageSize)

	c.JSON(http.StatusOK, gin.H{
		"data":       products,
		"pagination": meta,
	})
}

// GetByID is GET /products/:id — returns full detail including media URLs.
func (h *ProductHandler) GetByID(c *gin.Context) {
	id := c.Param("id")

	detail, err := h.store.GetByID(id)
	if err != nil {
		if err == catalog.ErrNotFound {
			jsonError(c, http.StatusNotFound, "product not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "unexpected error")
		return
	}

	jsonOK(c, http.StatusOK, detail)
}

// AddMedia is POST /products/:id/media — appends URLs to an existing product.
func (h *ProductHandler) AddMedia(c *gin.Context) {
	id := c.Param("id")

	var req catalog.AddMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if len(req.ImageURLs) == 0 && len(req.VideoURLs) == 0 {
		jsonError(c, http.StatusBadRequest, "at least one of image_urls or video_urls is required")
		return
	}

	if err := validateMediaArrays(req.ImageURLs, req.VideoURLs); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	product, err := h.store.AddMedia(id, req)
	if err != nil {
		if err == catalog.ErrNotFound {
			jsonError(c, http.StatusNotFound, "product not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "unexpected error")
		return
	}

	jsonOK(c, http.StatusOK, product)
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

func validateMediaArrays(images, videos []string) error {
	if len(images) > maxURLsPerArray {
		return fmt.Errorf("too many image URLs (max %d per request)", maxURLsPerArray)
	}
	if len(videos) > maxURLsPerArray {
		return fmt.Errorf("too many video URLs (max %d per request)", maxURLsPerArray)
	}
	for _, u := range images {
		if err := validateURL(u); err != nil {
			return fmt.Errorf("invalid image URL: %w", err)
		}
	}
	for _, u := range videos {
		if err := validateURL(u); err != nil {
			return fmt.Errorf("invalid video URL: %w", err)
		}
	}
	return nil
}

func validateURL(raw string) error {
	if len(raw) > maxURLLength {
		return fmt.Errorf("exceeds max length of %d characters", maxURLLength)
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("malformed URL: %s", raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("must use http or https scheme: %s", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("missing host: %s", raw)
	}
	return nil
}

func queryInt(c *gin.Context, key string, fallback int) int {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}
