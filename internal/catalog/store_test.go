package catalog

import (
	"sync"
	"testing"
)

func TestCreate_BasicFlow(t *testing.T) {
	s := NewStore()

	p, err := s.Create(CreateRequest{
		Name:      "Widget",
		SKU:       "W-001",
		ImageURLs: []string{"https://cdn.example.com/img1.jpg"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
	if p.ImageCount != 1 {
		t.Errorf("expected image_count=1, got %d", p.ImageCount)
	}
	if p.ThumbnailURL != "https://cdn.example.com/img1.jpg" {
		t.Errorf("unexpected thumbnail: %s", p.ThumbnailURL)
	}
}

func TestCreate_DuplicateSKU(t *testing.T) {
	s := NewStore()

	_, err := s.Create(CreateRequest{Name: "A", SKU: "dup"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Create(CreateRequest{Name: "B", SKU: "dup"})
	if err != ErrDuplicateSKU {
		t.Errorf("expected ErrDuplicateSKU, got %v", err)
	}
}

func TestList_NoMediaInResponse(t *testing.T) {
	s := NewStore()

	s.Create(CreateRequest{
		Name:      "P1",
		SKU:       "S1",
		ImageURLs: []string{"https://cdn.example.com/img.jpg"},
	})

	products, meta := s.List(1, 20)
	if meta.Total != 1 {
		t.Fatalf("expected total=1, got %d", meta.Total)
	}

	// Product struct has no image_urls/video_urls fields — this is
	// enforced at the type level. Just verify the metadata is right.
	if products[0].ImageCount != 1 {
		t.Error("expected image_count=1")
	}
}

func TestGetByID_ReturnsFullMedia(t *testing.T) {
	s := NewStore()

	p, _ := s.Create(CreateRequest{
		Name:      "P",
		SKU:       "S",
		ImageURLs: []string{"https://cdn.example.com/a.jpg", "https://cdn.example.com/b.jpg"},
		VideoURLs: []string{"https://cdn.example.com/v.mp4"},
	})

	detail, err := s.GetByID(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.ImageURLs) != 2 {
		t.Errorf("expected 2 images, got %d", len(detail.ImageURLs))
	}
	if len(detail.VideoURLs) != 1 {
		t.Errorf("expected 1 video, got %d", len(detail.VideoURLs))
	}
}

func TestAddMedia_UpdatesCounts(t *testing.T) {
	s := NewStore()

	p, _ := s.Create(CreateRequest{Name: "P", SKU: "S"})

	updated, err := s.AddMedia(p.ID, AddMediaRequest{
		ImageURLs: []string{"https://cdn.example.com/new.jpg"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ImageCount != 1 {
		t.Errorf("expected image_count=1 after add, got %d", updated.ImageCount)
	}
	if updated.ThumbnailURL != "https://cdn.example.com/new.jpg" {
		t.Error("thumbnail should be set from first image")
	}
}

// TestCreate_ConcurrentDuplicateSKU fires 50 goroutines trying to create
// with the same SKU. Exactly one should succeed.
func TestCreate_ConcurrentDuplicateSKU(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	var successes int64
	var mu sync.Mutex

	n := 50
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := s.Create(CreateRequest{
				Name: "Contended",
				SKU:  "RACE-SKU",
			})
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if successes != 1 {
		t.Errorf("expected exactly 1 successful create, got %d", successes)
	}
}
