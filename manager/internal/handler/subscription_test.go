package handler

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

func createActiveTestNode(t *testing.T) *model.Node {
	t.Helper()
	node := model.Node{
		Name:     "Test-Node",
		Type:     "xray",
		Address:  "node.example.com",
		Port:     443,
		Protocol: "vless",
		Status:   "active",
		Network:  "ws",
		WSPath:   "/ws",
	}
	if result := db.DB.Create(&node); result.Error != nil {
		t.Fatalf("failed to create node: %v", result.Error)
	}
	return &node
}

func TestHandleQRCodeFormat_ReturnsPNG(t *testing.T) {
	setupTestDB(t)
	user := createActiveTestUser(t)
	createActiveTestNode(t)

	app := fiber.New()
	app.Get("/api/v1/client/links/:token/qrcode", GetLinksQRCode)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/client/links/%s/qrcode", user.ClientToken), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected Content-Type image/png, got %q", ct)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if len(body) < 24 {
		t.Fatalf("expected a PNG body of at least 24 bytes, got %d", len(body))
	}
	if !bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("expected PNG magic bytes, got %q", body[:8])
	}
	// PNG signature (8) + IHDR chunk length (4) + "IHDR" (4): width/height follow.
	width := binary.BigEndian.Uint32(body[16:20])
	height := binary.BigEndian.Uint32(body[20:24])
	if width != 256 || height != 256 {
		t.Fatalf("expected 256x256 PNG, got %dx%d", width, height)
	}

	// Headers set by handleLinksRequest must be preserved.
	if resp.Header.Get("Subscription-Userinfo") == "" {
		t.Fatal("expected Subscription-Userinfo header to be preserved")
	}
}

func TestCheckRateLimit_PerTokenPerIP(t *testing.T) {
	setupTestDB(t)
	user := createActiveTestUser(t)
	other := createActiveTestUser(t)
	createActiveTestNode(t)

	app := fiber.New()
	app.Get("/api/v1/client/links/:token", GetLinksV2ray)

	path := func(token string) string {
		return fmt.Sprintf("/api/v1/client/links/%s", token)
	}

	// First request for the token is allowed.
	if resp, err := app.Test(httptest.NewRequest("GET", path(user.ClientToken), nil), -1); err != nil {
		t.Fatalf("request failed: %v", err)
	} else if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on first request, got %d", resp.StatusCode)
	}

	// Second request within 10s for the same token+IP is rate limited.
	if resp, err := app.Test(httptest.NewRequest("GET", path(user.ClientToken), nil), -1); err != nil {
		t.Fatalf("request failed: %v", err)
	} else if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("expected 429 on rapid second request, got %d", resp.StatusCode)
	}

	// A different token uses a separate limiter and is not affected.
	if resp, err := app.Test(httptest.NewRequest("GET", path(other.ClientToken), nil), -1); err != nil {
		t.Fatalf("request failed: %v", err)
	} else if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 for a different token, got %d", resp.StatusCode)
	}
}

func TestTTLCacheEvictsExpiredEntries(t *testing.T) {
	cache := newTTLCache[string](20*time.Millisecond, 100)
	if got := cache.GetOrCreate("a", func() string { return "1" }); got != "1" {
		t.Fatalf("expected initial value, got %q", got)
	}
	time.Sleep(40 * time.Millisecond)
	if got := cache.GetOrCreate("b", func() string { return "2" }); got != "2" {
		t.Fatalf("expected value for new key, got %q", got)
	}
	if got := cache.Len(); got != 1 {
		t.Fatalf("expected expired entry evicted, len = %d", got)
	}
	// The expired key is recreated fresh (create runs again).
	if got := cache.GetOrCreate("a", func() string { return "3" }); got != "3" {
		t.Fatalf("expected expired key to be recreated, got %q", got)
	}
}

func TestTTLCacheBoundedSize(t *testing.T) {
	cache := newTTLCache[string](time.Hour, 2)
	cache.GetOrCreate("a", func() string { return "1" })
	cache.GetOrCreate("b", func() string { return "2" })
	cache.GetOrCreate("c", func() string { return "3" })
	if got := cache.Len(); got != 2 {
		t.Fatalf("expected cache bounded to maxEntries, len = %d", got)
	}
	// The oldest entry "a" was evicted to make room for "c".
	if got := cache.GetOrCreate("a", func() string { return "1-again" }); got != "1-again" {
		t.Fatalf("expected evicted \"a\" to be recreated, got %q", got)
	}
}
