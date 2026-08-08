package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

const maxClockSkew = 5 * time.Minute

// nodeTokenCacheTTL is how long a node auth token → node mapping is served
// from memory. NodeHMAC runs on every daemon request (config pulls and traffic
// reports arrive on a tight schedule), so caching removes a token lookup per
// request. Trade-off: after an admin disables a node, the change takes effect
// within one TTL (≤30s) instead of immediately; GenerateNodeToken explicitly
// invalidates the rotated token's entry, so a rotated old token stops working
// at once.
const nodeTokenCacheTTL = 30 * time.Second

// nodeTokenCacheMaxEntries bounds the token→node cache. Node counts are tiny;
// this only guards against unbounded growth if tokens are rotated repeatedly.
const nodeTokenCacheMaxEntries = 1024

type nodeTokenCacheEntry struct {
	node      model.Node
	fetchedAt time.Time
}

var nodeTokenCache = struct {
	mu    sync.Mutex
	items map[string]nodeTokenCacheEntry
}{
	items: make(map[string]nodeTokenCacheEntry),
}

// lookupNodeByToken resolves a node by its daemon token, serving a cached copy
// for up to nodeTokenCacheTTL. The node is returned by value so callers can
// hold a copy without racing the cache.
func lookupNodeByToken(token string) (model.Node, error) {
	nodeTokenCache.mu.Lock()
	defer nodeTokenCache.mu.Unlock()

	if e, ok := nodeTokenCache.items[token]; ok && time.Since(e.fetchedAt) <= nodeTokenCacheTTL {
		return e.node, nil
	}

	var node model.Node
	if err := db.DB.Where("token = ?", token).First(&node).Error; err != nil {
		delete(nodeTokenCache.items, token)
		return model.Node{}, err
	}
	if len(nodeTokenCache.items) >= nodeTokenCacheMaxEntries {
		// The map is small and entries self-expire within the TTL, so a full
		// reset is cheaper than tracking LRU order.
		nodeTokenCache.items = make(map[string]nodeTokenCacheEntry)
	}
	nodeTokenCache.items[token] = nodeTokenCacheEntry{node: node, fetchedAt: time.Now()}
	return node, nil
}

// InvalidateNodeToken drops a token's cached node so the next request re-reads
// the DB. Called when a node's daemon token is rotated: the old token must not
// stay usable for the remainder of the TTL.
func InvalidateNodeToken(token string) {
	nodeTokenCache.mu.Lock()
	delete(nodeTokenCache.items, token)
	nodeTokenCache.mu.Unlock()
}

// ResetNodeTokenCache is a test helper: clears the token→node cache so tests
// against a fresh DB never read a node cached for another database.
var ResetNodeTokenCache = func() {
	nodeTokenCache.mu.Lock()
	nodeTokenCache.items = make(map[string]nodeTokenCacheEntry)
	nodeTokenCache.mu.Unlock()
}

// nodeSecret derives the HMAC signing key for a node daemon from its token.
// Using a per-node secret derived from the token means a leaked request can't
// be replayed against other nodes, and rotating the token invalidates it.
func nodeSecret(token string) []byte {
	h := sha256.Sum256([]byte("rfplay-node-hmac-v1:" + token))
	return h[:]
}

// NodeHMAC authenticates a node daemon request with HMAC-SHA256.
//
// Headers:
//   X-Node-Timestamp: unix seconds
//   X-Node-Signature: hex( HMAC_SHA256(secret, "<method>\n<path>\n<timestamp>\n<body>") )
//
// The signature is bound to method, path and a timestamp window to prevent
// replay. The node token in the URL path is still required; the HMAC key is
// derived from it, so a caller must already know the token. This defends
// against tampered/replayed payloads even if the token itself is used with
// plaintext transport and adds defence-in-depth on top of the token check.
func NodeHMAC() fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := c.Params("token")
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "INVALID_TOKEN"})
		}

		// Look up the node (token→node cache) so we only pay HMAC cost for
		// real nodes and can return consistent auth errors.
		node, err := lookupNodeByToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "INVALID_TOKEN"})
		}

		// Block disabled/inactive nodes from pulling configs or reporting
		// traffic. This prevents a leaked or revoked node token (e.g. after an
		// admin disables a node) from still being used against the API.
		if node.Status != "active" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "NODE_DISABLED"})
		}

		tsHeader := c.Get("X-Node-Timestamp")
		if tsHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "MISSING_TIMESTAMP"})
		}
		ts, err := strconv.ParseInt(tsHeader, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "INVALID_TIMESTAMP"})
		}
		if delta := time.Now().Unix() - ts; delta < -int64(maxClockSkew.Seconds()) || delta > int64(maxClockSkew.Seconds()) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "STALE_TIMESTAMP"})
		}

		sigHeader := c.Get("X-Node-Signature")
		if sigHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "MISSING_SIGNATURE"})
		}

		body := c.Body()
		mac := hmac.New(sha256.New, nodeSecret(node.Token))
		mac.Write([]byte(c.Method()))
		mac.Write([]byte("\n"))
		mac.Write([]byte(c.Path()))
		mac.Write([]byte("\n"))
		mac.Write([]byte(tsHeader))
		mac.Write([]byte("\n"))
		mac.Write(body)
		expected := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(sigHeader), []byte(expected)) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "INVALID_SIGNATURE"})
		}

		// Cache the resolved node on the context so handlers don't re-query.
		c.Locals("node", &node)
		return c.Next()
	}
}
