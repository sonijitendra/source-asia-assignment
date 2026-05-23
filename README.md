# Source Asia — Backend Assignment

A single Go HTTP service implementing two features:

1. **Rate-limited request endpoint** with rolling-window tracking and concurrency safety
2. **Product catalog API** with separated metadata/media storage for fast list queries

## Quick Start

```bash
# clone and run
go run ./cmd/server

# server starts on :8080
```

Or build a binary:

```bash
go build -o server ./cmd/server
./server
```

---

## Part 1 — Rate-Limited API

### POST /request

Accepts a JSON request and rate-limits by `user_id`: max **5 accepted requests per user per rolling 1-minute window**.

**Request:**
```json
{
  "user_id": "alice",
  "payload": {"action": "click", "page": "/home"}
}
```

**Success (201 Created):**
```json
{
  "data": {
    "message": "request accepted",
    "user_id": "alice"
  }
}
```

**Rate limited (429 Too Many Requests):**
```json
{
  "error": "rate limit exceeded: max 5 requests per minute per user"
}
```

**Validation errors (400 Bad Request):**
```json
{"error": "user_id is required and must be non-empty"}
{"error": "payload is required"}
{"error": "invalid JSON body"}
```

`payload` accepts any valid JSON value — objects, arrays, strings, numbers, booleans, and null are all valid.

### GET /stats

Returns per-user and global counters. The `accepted` count reflects the current rolling window at the time of the request.

```json
{
  "data": {
    "users": {
      "alice": {"accepted": 3, "rejected": 0},
      "bob":   {"accepted": 5, "rejected": 7}
    },
    "total_accepted": 8,
    "total_rejected": 7
  }
}
```

`rejected` is cumulative (total rejected since the server started). `accepted` is the count of requests still within the 1-minute window.

### Example curl commands

```bash
# Basic request
curl -X POST http://localhost:8080/request \
  -H "Content-Type: application/json" \
  -d '{"user_id": "alice", "payload": "hello"}'

# Fire 10 requests in parallel for the same user (expect 5 x 201, 5 x 429)
seq 10 | xargs -P10 -I{} curl -s -o /dev/null -w "%{http_code}\n" \
  -X POST http://localhost:8080/request \
  -H "Content-Type: application/json" \
  -d '{"user_id": "alice", "payload": "req-{}"}'

# Check stats
curl http://localhost:8080/stats
```

**Windows PowerShell equivalent for concurrency test:**

```powershell
1..10 | ForEach-Object -Parallel {
    $r = Invoke-WebRequest -Uri "http://localhost:8080/request" `
        -Method POST -ContentType "application/json" `
        -Body '{"user_id":"alice","payload":"test"}' `
        -SkipHttpErrorCheck
    $r.StatusCode
} -ThrottleLimit 10
```

### Concurrency Design

The rate limiter uses a **single `sync.Mutex`** protecting a `map[string][]time.Time`.

```
Allow(userID):
  Lock()
  defer Unlock()
  1. filter out timestamps older than 1 minute (in-place, zero-alloc)
  2. if len(remaining) >= 5 → reject, increment rejected counter, return false
  3. else → append time.Now(), return true
```

**Why this is correct:** The lock is held across the entire check-and-record sequence. Two goroutines handling the same `user_id` can never both see 4 timestamps, both append, and end up with 6. One will always see the other's write.

**Why a single mutex:** The critical section runs in ~200ns (one slice scan + one append). At realistic HTTP throughput, lock contention is negligible. Per-user locking sounds appealing but requires a second mutex to protect the lock map itself, adding complexity for zero measurable benefit at this scale.

**Why rolling window:** A fixed window lets a user fire 5 requests at 0:59 and another 5 at 1:01. A rolling window tracks exact timestamps, so the limit is truly "5 per any 60-second span."

---

## Part 2 — Product Catalog

### Data Model (the key design decision)

Products and media are stored in **separate maps**:

```
products  map[string]*Product   →  id, name, sku, image_count, video_count, thumbnail_url, created_at
media     map[string]*Media     →  image_urls []string, video_urls []string
skuIdx    map[string]string     →  sku → product_id (O(1) uniqueness check)
```

`GET /products` (list) iterates **only** over the `products` map. It never touches, loads, or serializes any URL arrays. With 1,000 products × 10 images each, the list endpoint serializes ~20 lightweight objects instead of 10,000+ URL strings.

`GET /products/:id` (detail) merges both maps to return the full view with all media.

### POST /products

**Request:**
```json
{
  "name": "Widget A",
  "sku": "SKU-001",
  "image_urls": [
    "https://cdn.example.com/products/sku-001/img-1.jpg",
    "https://cdn.example.com/products/sku-001/img-2.jpg"
  ],
  "video_urls": [
    "https://cdn.example.com/products/sku-001/demo.mp4"
  ]
}
```

**Success (201 Created):**
```json
{
  "data": {
    "id": "p_1",
    "name": "Widget A",
    "sku": "SKU-001",
    "image_count": 2,
    "video_count": 1,
    "thumbnail_url": "https://cdn.example.com/products/sku-001/img-1.jpg",
    "created_at": "2025-05-23T12:00:00Z"
  }
}
```

| Status | Condition |
|--------|-----------|
| 201    | Created successfully |
| 400    | Empty name, empty SKU, invalid URL, too many URLs |
| 409    | Duplicate SKU |

### GET /products

Returns paginated product metadata — **no media URL arrays**.

**Query parameters:**
| Param | Default | Max | Description |
|-------|---------|-----|-------------|
| `page` | 1 | — | Page number (1-indexed) |
| `page_size` | 20 | 100 | Items per page |

**Response:**
```json
{
  "data": [
    {
      "id": "p_1",
      "name": "Widget A",
      "sku": "SKU-001",
      "image_count": 2,
      "video_count": 1,
      "thumbnail_url": "https://cdn.example.com/products/sku-001/img-1.jpg",
      "created_at": "2025-05-23T12:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

Products are sorted newest-first. The `thumbnail_url` is automatically set to the first image URL (if any).

### GET /products/:id

Returns the full product detail **including all media URLs**.

```json
{
  "data": {
    "id": "p_1",
    "name": "Widget A",
    "sku": "SKU-001",
    "image_urls": [
      "https://cdn.example.com/products/sku-001/img-1.jpg",
      "https://cdn.example.com/products/sku-001/img-2.jpg"
    ],
    "video_urls": [
      "https://cdn.example.com/products/sku-001/demo.mp4"
    ],
    "thumbnail_url": "https://cdn.example.com/products/sku-001/img-1.jpg",
    "created_at": "2025-05-23T12:00:00Z"
  }
}
```

Returns 404 for unknown IDs.

### POST /products/:id/media

Appends URLs to an existing product. At least one of `image_urls` or `video_urls` must be provided.

```bash
curl -X POST http://localhost:8080/products/p_1/media \
  -H "Content-Type: application/json" \
  -d '{"image_urls": ["https://cdn.example.com/products/sku-001/img-3.jpg"]}'
```

Returns updated product metadata (200) or 404/400.

### Validation Rules

| Rule | Limit |
|------|-------|
| URL scheme | Must be `http://` or `https://` |
| URL length | Max 2048 characters |
| URLs per array per request | Max 20 |
| Name / SKU | Required, non-empty (whitespace trimmed) |
| SKU uniqueness | Enforced — duplicate returns 409 |

### Example: Seed 50 products

```bash
for i in $(seq 1 50); do
  curl -s -X POST http://localhost:8080/products \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"Product $i\",
      \"sku\": \"SKU-$(printf '%03d' $i)\",
      \"image_urls\": [
        \"https://cdn.example.com/p$i/img1.jpg\",
        \"https://cdn.example.com/p$i/img2.jpg\",
        \"https://cdn.example.com/p$i/img3.jpg\"
      ],
      \"video_urls\": [
        \"https://cdn.example.com/p$i/demo.mp4\"
      ]
    }" > /dev/null
done

# verify list is lightweight
curl -s http://localhost:8080/products?page=1\&page_size=10 | python3 -m json.tool
```

---

## Performance Optimization

### How list vs detail queries differ

| Aspect | GET /products (list) | GET /products/:id (detail) |
|--------|---------------------|---------------------------|
| Data touched | `products` map only | `products` + `media` maps |
| Serialization | ~5 small fields per item | Full URL arrays |
| With 1k products × 10 images | ~20 objects, ~2KB | 1 object, ~1KB |

The list endpoint never iterates over, loads, or serializes any `image_urls` or `video_urls` arrays. The `image_count`, `video_count`, and `thumbnail_url` fields are maintained as denormalized counters on the product metadata, updated at write time.

### What would change with PostgreSQL + CDN

In production you'd normalize further:

- **Products table**: `id`, `name`, `sku`, `image_count`, `video_count`, `thumbnail_url`, `created_at`
- **Media table**: `id`, `product_id`, `url`, `type` (image/video), `position`, `created_at`
- List query: `SELECT id, name, sku, image_count, video_count, thumbnail_url FROM products ORDER BY created_at DESC LIMIT 20 OFFSET 0` — zero joins, no media touched
- Detail query: `SELECT ... FROM products p JOIN media m ON p.id = m.product_id WHERE p.id = $1`
- Media URLs would point to a CDN; the API only stores references
- Cursor-based pagination would replace offset-based for large datasets
- `image_count` / `video_count` maintained via triggers or application-level updates

---

## Production Limitations

This is an in-memory, single-instance demo. In production:

| Limitation | What you'd do instead |
|---|---|
| **Single process** | Run multiple instances behind a load balancer; rate limiting moves to Redis (e.g., `MULTI/EXEC` sliding window) |
| **No persistence** | All data lost on restart; use PostgreSQL or similar |
| **No authentication** | `user_id` is self-reported; real systems use JWT / API keys |
| **Unbounded memory** | Rate limiter entries grow with unique users; add TTL-based eviction or a capped LRU |
| **Offset pagination** | Becomes expensive at large offsets; use cursor-based (keyset) pagination |
| **List sort cost** | Sorting all products on every request is O(n log n); maintain a sorted index or use DB `ORDER BY` |
| **No request size limit** | Add middleware to cap body size (e.g., 1MB) |
| **No graceful shutdown** | Add `signal.Notify` + `server.Shutdown` for clean drain |

---

## Design Tradeoffs

**Single mutex for rate limiter vs per-user locking:** Chose simplicity. The critical section is a single slice scan (~5 elements) + append. Lock contention is effectively zero at normal traffic. Per-user locking doubles the code and introduces its own race conditions around lock-map management.

**`sync.RWMutex` for catalog vs single `Mutex`:** The catalog benefits from read-heavy access patterns (listing/viewing products is more common than creating). `RWMutex` lets concurrent readers proceed without blocking each other.

**Denormalized counts on Product:** `image_count` and `video_count` are updated at write time. This trades slight write overhead for zero-cost reads during listing. The same pattern works in SQL with counter columns.

**Sorted on read vs maintaining a sorted structure:** For the expected scale (~thousands of products), sorting on read is simpler and fast enough. A `sync`-safe B-tree would be justified at millions of entries.

---

## Project Structure

```
.
├── cmd/server/main.go            # Entry point, router setup
├── internal/
│   ├── ratelimit/
│   │   └── limiter.go            # Rolling window rate limiter
│   ├── catalog/
│   │   ├── models.go             # Type definitions
│   │   └── store.go              # In-memory store (3 maps)
│   └── api/
│       ├── response.go           # JSON response helpers
│       ├── request.go            # POST /request, GET /stats
│       └── product.go            # Product CRUD handlers
├── go.mod
├── go.sum
└── README.md
```

## AI Tools

This project was developed with assistance from an AI coding assistant (Gemini) for code generation, documentation, and design review.
