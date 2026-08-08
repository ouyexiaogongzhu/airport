package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// signNodeRequest mirrors the daemon's signing logic for test purposes.
func signNodeRequest(token, method, path, ts, body string) string {
	mac := hmac.New(sha256.New, nodeSecret(token))
	mac.Write([]byte(method))
	mac.Write([]byte("\n"))
	mac.Write([]byte(path))
	mac.Write([]byte("\n"))
	mac.Write([]byte(ts))
	mac.Write([]byte("\n"))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestNodeHMAC_Success(t *testing.T) {
	setupTestDB(t)
	const token = "nd_testtoken1234567890abcdef"
	createTestNode(t, token)

	app := fiber.New()
	app.Post("/node/:token/traffic/report", NodeHMAC(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	ts := time.Now().Unix()
	body := `{"node_id":1,"traffic":[{"user_id":2,"upload_bytes":10,"download_bytes":5}]}`
	path := "/node/" + token + "/traffic/report"
	sig := signNodeRequest(token, "POST", path, itoa(ts), body)

	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("X-Node-Timestamp", itoa(ts))
	req.Header.Set("X-Node-Signature", sig)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		bodyBytes := make([]byte, 256)
		n, _ := resp.Body.Read(bodyBytes)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes[:n]))
	}
}

func TestNodeHMAC_InvalidToken(t *testing.T) {
	setupTestDB(t)

	app := fiber.New()
	app.Get("/node/:token/config", NodeHMAC(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/node/nonexistent/config", nil)
	req.Header.Set("X-Node-Timestamp", itoa(time.Now().Unix()))
	req.Header.Set("X-Node-Signature", "deadbeef")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestNodeHMAC_WrongSignature(t *testing.T) {
	setupTestDB(t)
	const token = "nd_testtoken1234567890abcdef"
	createTestNode(t, token)

	app := fiber.New()
	app.Get("/node/:token/config", NodeHMAC(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	path := "/node/" + token + "/config"
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("X-Node-Timestamp", itoa(time.Now().Unix()))
	req.Header.Set("X-Node-Signature", "deadbeef")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestNodeHMAC_StaleTimestamp(t *testing.T) {
	setupTestDB(t)
	const token = "nd_testtoken1234567890abcdef"
	createTestNode(t, token)

	app := fiber.New()
	app.Get("/node/:token/config", NodeHMAC(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	ts := time.Now().Add(-30 * time.Minute).Unix()
	path := "/node/" + token + "/config"
	sig := signNodeRequest(token, "GET", path, itoa(ts), "")

	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("X-Node-Timestamp", itoa(ts))
	req.Header.Set("X-Node-Signature", sig)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestNodeHMAC_DisabledNode(t *testing.T) {
	setupTestDB(t)
	const token = "nd_testtoken1234567890abcdef"
	node := createTestNode(t, token)
	// An admin disabled the node; its token should no longer be usable even
	// with a perfectly valid timestamp + HMAC signature.
	if err := db.DB.Model(node).Update("status", "inactive").Error; err != nil {
		t.Fatalf("failed to update node status: %v", err)
	}

	app := fiber.New()
	app.Get("/node/:token/config", NodeHMAC(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	ts := time.Now().Unix()
	path := "/node/" + token + "/config"
	sig := signNodeRequest(token, "GET", path, itoa(ts), "")

	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("X-Node-Timestamp", itoa(ts))
	req.Header.Set("X-Node-Signature", sig)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload.Error != "NODE_DISABLED" {
		t.Fatalf("expected error NODE_DISABLED, got %q", payload.Error)
	}
}

func TestNodeHMAC_DisabledNode_RejectsTrafficReport(t *testing.T) {
	setupTestDB(t)
	const token = "nd_testtoken1234567890abcdef"
	node := createTestNode(t, token)
	if err := db.DB.Model(node).Update("status", "disabled").Error; err != nil {
		t.Fatalf("failed to update node status: %v", err)
	}

	app := fiber.New()
	app.Post("/node/:token/traffic/report", NodeHMAC(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	ts := time.Now().Unix()
	body := `{"node_id":1,"traffic":[{"user_id":2,"upload_bytes":10,"download_bytes":5}]}`
	path := "/node/" + token + "/traffic/report"
	sig := signNodeRequest(token, "POST", path, itoa(ts), body)

	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("X-Node-Timestamp", itoa(ts))
	req.Header.Set("X-Node-Signature", sig)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestNodeTokenCache_ServesCachedCopy(t *testing.T) {
	setupTestDB(t)
	ResetNodeTokenCache()
	const token = "nd_cache_stale_token"
	node := createTestNode(t, token)

	if _, err := lookupNodeByToken(token); err != nil {
		t.Fatalf("expected initial lookup to succeed: %v", err)
	}

	// Change the DB row behind the cache (e.g. admin disables the node). The
	// cached value copy keeps the previously-resolved node until the TTL
	// expires or the entry is invalidated — the accepted ≤30s propagation lag.
	if err := db.DB.Model(&model.Node{}).Where("id = ?", node.ID).Update("status", "inactive").Error; err != nil {
		t.Fatalf("failed to disable node: %v", err)
	}
	n, err := lookupNodeByToken(token)
	if err != nil {
		t.Fatalf("expected the cached node to be served before TTL expiry: %v", err)
	}
	if n.Status != "active" {
		t.Fatalf("expected cached status active, got %s", n.Status)
	}
}

func TestNodeTokenCache_InvalidationOnRotate(t *testing.T) {
	setupTestDB(t)
	ResetNodeTokenCache()
	const token = "nd_cache_rotate_token"
	node := createTestNode(t, token)

	// Prime the cache with the original token.
	if _, err := lookupNodeByToken(token); err != nil {
		t.Fatalf("expected initial lookup to succeed: %v", err)
	}

	// Simulate an admin rotating the token via GenerateNodeToken: only the DB
	// column changes, then the middleware cache entry for the old token is
	// dropped so it stops working immediately.
	if err := db.DB.Model(&model.Node{}).Where("id = ?", node.ID).Update("token", "nd_rotated").Error; err != nil {
		t.Fatalf("failed to rotate token: %v", err)
	}
	InvalidateNodeToken(token)

	if _, err := lookupNodeByToken(token); err == nil {
		t.Fatal("expected the old token to be rejected after rotation + invalidation")
	}
	n, err := lookupNodeByToken("nd_rotated")
	if err != nil {
		t.Fatalf("expected the rotated token to resolve: %v", err)
	}
	if n.ID != node.ID {
		t.Fatalf("expected rotated token to map to node %d, got %d", node.ID, n.ID)
	}
}
