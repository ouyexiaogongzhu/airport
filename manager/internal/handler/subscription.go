package handler

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto/rand"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"github.com/skip2/go-qrcode"
)

// linkRateLimiterTTL is how long a token's rate limiter stays cached after
// its last request. Idle tokens are evicted so memory stays bounded.
const linkRateLimiterTTL = 10 * time.Minute

// linkRateLimiterMaxEntries caps how many token rate limiters are kept in
// memory; the least-recently-used tokens are evicted first when exceeded.
const linkRateLimiterMaxEntries = 4096

var linkRateLimiters = newTTLCache[*rateLimiter](linkRateLimiterTTL, linkRateLimiterMaxEntries)

type rateLimiter struct {
	mu      sync.Mutex
	lastReq map[string]time.Time // ip -> last request time
}

// ttlCache is a goroutine-safe, bounded cache keyed by string. Every
// GetOrCreate call refreshes the entry's lastAccess time, opportunistically
// sweeps entries idle for longer than ttl, and evicts the least-recently
// accessed entries when maxEntries would be exceeded.
type ttlCache[V any] struct {
	mu         sync.Mutex
	items      map[string]*ttlCacheEntry[V]
	ttl        time.Duration
	maxEntries int
}

type ttlCacheEntry[V any] struct {
	value      V
	lastAccess time.Time
}

func newTTLCache[V any](ttl time.Duration, maxEntries int) *ttlCache[V] {
	return &ttlCache[V]{
		items:      make(map[string]*ttlCacheEntry[V]),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

// GetOrCreate returns the cached value for key, calling create if the key is
// absent. Active keys (repeatedly accessed) never expire because every call
// refreshes lastAccess.
func (c *ttlCache[V]) GetOrCreate(key string, create func() V) V {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if e, ok := c.items[key]; ok {
		e.lastAccess = now
		return e.value
	}

	e := &ttlCacheEntry[V]{value: create(), lastAccess: now}
	c.items[key] = e
	c.sweepLocked(now)
	return e.value
}

// Len returns the number of cached entries (used by tests).
func (c *ttlCache[V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// sweepLocked removes entries idle for longer than ttl and, if the cache is
// still over maxEntries, evicts the least recently accessed entries.
// Callers must hold c.mu.
func (c *ttlCache[V]) sweepLocked(now time.Time) {
	for k, e := range c.items {
		if now.Sub(e.lastAccess) > c.ttl {
			delete(c.items, k)
		}
	}
	for over := len(c.items) - c.maxEntries; over > 0; over-- {
		var oldestKey string
		var oldestAccess time.Time
		for k, e := range c.items {
			if oldestKey == "" || e.lastAccess.Before(oldestAccess) {
				oldestKey = k
				oldestAccess = e.lastAccess
			}
		}
		delete(c.items, oldestKey)
	}
}

func getRateLimiter(token string) *rateLimiter {
	return linkRateLimiters.GetOrCreate(token, func() *rateLimiter {
		return &rateLimiter{lastReq: make(map[string]time.Time)}
	})
}

func checkRateLimit(c *fiber.Ctx, token string) bool {
	rl := getRateLimiter(token)
	rl.mu.Lock()
	defer rl.mu.Unlock()

	ip := c.IP()
	last, ok := rl.lastReq[ip]
	now := time.Now()

	if len(rl.lastReq) > 100 {
		for k, v := range rl.lastReq {
			if now.Sub(v) > 30*time.Second {
				delete(rl.lastReq, k)
			}
		}
	}

	if ok && now.Sub(last) < 10*time.Second {
		return false
	}
	rl.lastReq[ip] = now
	return true
}

// SubscriptionUserResponse for subscription endpoint
type SubscriptionUserResponse struct {
	ID                    uint   `json:"id"`
	Tier                  string `json:"tier"`
	TrafficRemainingBytes int64  `json:"traffic_remaining_bytes"`
	ExpireTime            int64  `json:"expire_time"`
}

type RoutingResponse struct {
	GeoipURL    string `json:"geoip_url"`
	GeositeURL  string `json:"geosite_url"`
	GeoipEtag   string `json:"geoip_etag"`
	GeositeEtag string `json:"geosite_etag"`
}

type SubscriptionResponse struct {
	User                SubscriptionUserResponse `json:"user"`
	Nodes               []string                 `json:"nodes"`
	Routing             RoutingResponse          `json:"routing"`
	SubscriptionVersion int                      `json:"subscription_version"`
}

// GetClientConfig returns public client configuration (no auth required)
// GET /api/v1/client/config
func GetClientConfig(c *fiber.Ctx) error {
	portalURL := os.Getenv("PORTAL_URL")
	if portalURL == "" {
		portalURL = "http://localhost:5173"
	}
	return c.JSON(fiber.Map{
		"portal_url":   portalURL,
		"renewal_path": "/plans",
	})
}

// GetSubscription returns subscription info with node list (JWT required)
// GET /api/v1/client/subscription
func GetSubscription(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var user model.User
	if result := db.DB.First(&user, userID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	if user.SubscriptionStatus != "active" {
		if user.SubscriptionStatus == "pending" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "SUBSCRIPTION_PENDING",
			})
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "SUBSCRIPTION_EXPIRED",
		})
	}

	var nodes []model.Node
	db.DB.Where("status = ?", "active").Find(&nodes)

	nodeURIs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		uri := EncodeNodeToURI(&node, &user)
		if uri != "" {
			nodeURIs = append(nodeURIs, uri)
		}
	}

	trafficRemaining := user.TrafficLimitBytes - user.TrafficUsedBytes
	if trafficRemaining < 0 {
		trafficRemaining = 0
	}

	return c.JSON(SubscriptionResponse{
		User: SubscriptionUserResponse{
			ID:                    user.ID,
			Tier:                  user.SubscriptionTier,
			TrafficRemainingBytes: trafficRemaining,
			ExpireTime:            user.ExpireTime,
		},
		Nodes: nodeURIs,
		Routing: RoutingResponse{
			GeoipURL:    "https://api.rfplay.uk/assets/geoip.dat",
			GeositeURL:  "https://api.rfplay.uk/assets/geosite.dat",
			GeoipEtag:   "",
			GeositeEtag: "",
		},
		SubscriptionVersion: 1,
	})
}

// GetClientToken returns masked client token (JWT required)
// GET /api/v1/web/client-token
func GetClientToken(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var user model.User
	if result := db.DB.First(&user, userID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	if user.ClientToken == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "no client token",
		})
	}

	token := user.ClientToken
	masked := token
	if len(token) > 12 {
		masked = token[:7] + "***" + token[len(token)-4:]
	}

	return c.JSON(fiber.Map{
		"token":      masked,
		"created_at": user.CreatedAt.Unix(),
	})
}

// RegenerateClientToken generates a new client token (JWT required)
// POST /api/v1/web/client-token/regenerate
func RegenerateClientToken(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var user model.User
	if result := db.DB.First(&user, userID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	newToken := "rf_" + hex.EncodeToString(tokenBytes)
	db.DB.Model(&user).Update("client_token", newToken)

	return c.JSON(fiber.Map{
		"token": newToken,
	})
}

// GetLinksV2ray returns V2ray base64 subscription format
// GET /api/v1/client/links/:token
func GetLinksV2ray(c *fiber.Ctx) error {
	token := c.Params("token")
	return handleLinksRequest(c, token, "v2ray")
}

// GetLinksClash returns Clash YAML format
// GET /api/v1/client/links/:token/clash
func GetLinksClash(c *fiber.Ctx) error {
	token := c.Params("token")
	return handleLinksRequest(c, token, "clash")
}

// GetLinksSingbox returns Sing-box JSON format
// GET /api/v1/client/links/:token/singbox
func GetLinksSingbox(c *fiber.Ctx) error {
	token := c.Params("token")
	return handleLinksRequest(c, token, "singbox")
}

// GetLinksQRCode returns QR code PNG
// GET /api/v1/client/links/:token/qrcode
func GetLinksQRCode(c *fiber.Ctx) error {
	token := c.Params("token")
	return handleLinksRequest(c, token, "qrcode")
}

func handleLinksRequest(c *fiber.Ctx, token string, format string) error {
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "INVALID_TOKEN",
		})
	}

	if !checkRateLimit(c, token) {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "rate limited",
		})
	}

	var user model.User
	if result := db.DB.Where("client_token = ?", token).First(&user); result.Error != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "INVALID_TOKEN",
		})
	}

	if user.SubscriptionStatus == "expired" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "SUBSCRIPTION_EXPIRED",
		})
	}
	if user.SubscriptionStatus == "pending" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "SUBSCRIPTION_PENDING",
		})
	}

	var nodes []model.Node
	db.DB.Where("status = ?", "active").Find(&nodes)

	if len(nodes) == 0 {
		return c.Status(fiber.StatusNoContent).Send(nil)
	}

	trafficRemaining := user.TrafficLimitBytes - user.TrafficUsedBytes
	if trafficRemaining < 0 {
		trafficRemaining = 0
	}

	c.Set("Subscription-Userinfo", fmt.Sprintf("upload=0; download=%d; total=%d; expire=%d",
		user.TrafficUsedBytes, user.TrafficLimitBytes, user.ExpireTime))
	c.Set("X-UltraUsage-Remaining", strconv.FormatInt(trafficRemaining, 10))
	c.Set("X-UltraUsage-Total", strconv.FormatInt(user.TrafficLimitBytes, 10))
	c.Set("X-UltraUsage-Expiry", strconv.FormatInt(user.ExpireTime, 10))

	switch format {
	case "v2ray":
		return handleV2rayFormat(c, &user, &nodes)
	case "clash":
		return handleClashFormat(c, &user, &nodes)
	case "singbox":
		return handleSingboxFormat(c, &user, &nodes)
	case "qrcode":
		return handleQRCodeFormat(c, &user, &nodes)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid format",
		})
	}
}

func handleV2rayFormat(c *fiber.Ctx, user *model.User, nodes *[]model.Node) error {
	var lines []string
	for _, node := range *nodes {
		uri := EncodeNodeToURI(&node, user)
		if uri != "" {
			lines = append(lines, uri)
		}
	}
	if len(lines) == 0 {
		return c.Status(fiber.StatusNoContent).Send(nil)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))
	c.Set("Content-Type", "text/plain; charset=utf-8")
	return c.SendString(encoded)
}

func handleClashFormat(c *fiber.Ctx, user *model.User, nodes *[]model.Node) error {
	var sb strings.Builder
	sb.WriteString("port: 7890\n")
	sb.WriteString("socks-port: 7891\n")
	sb.WriteString("mode: Rule\n")
	sb.WriteString("log-level: info\n\n")

	sb.WriteString("proxies:\n")
	for _, node := range *nodes {
		switch node.Protocol {
		case "vmess":
			sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", node.Name))
			sb.WriteString("    type: vmess\n")
			sb.WriteString(fmt.Sprintf("    server: %s\n", node.Address))
			sb.WriteString(fmt.Sprintf("    port: %d\n", node.Port))
			uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", user.ID, 0, 0, 0, user.ID*100)
			sb.WriteString(fmt.Sprintf("    uuid: %s\n", uuid))
			sb.WriteString("    alterId: 0\n")
			sb.WriteString("    cipher: auto\n")
			sb.WriteString("    tls: true\n")
			sb.WriteString("    network: ws\n")
			sb.WriteString("    ws-path: /ws\n")
			sb.WriteString(fmt.Sprintf("    ws-headers:\n      Host: %s\n\n", node.Address))
		case "vless":
			sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", node.Name))
			sb.WriteString("    type: vless\n")
			sb.WriteString(fmt.Sprintf("    server: %s\n", node.Address))
			sb.WriteString(fmt.Sprintf("    port: %d\n", node.Port))
			uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", user.ID, 0, 0, 0, user.ID*100)
			sb.WriteString(fmt.Sprintf("    uuid: %s\n", uuid))
			sb.WriteString("    flow: xtls-rprx-vision\n")
			sb.WriteString("    tls: true\n")
			sb.WriteString("    network: tcp\n\n")
		case "shadowsocks":
			sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", node.Name))
			sb.WriteString("    type: ss\n")
			sb.WriteString(fmt.Sprintf("    server: %s\n", node.Address))
			sb.WriteString(fmt.Sprintf("    port: %d\n", node.Port))
			sb.WriteString("    cipher: aes-256-gcm\n")
			sb.WriteString(fmt.Sprintf("    password: \"rf-%d-pass\"\n\n", user.ID))
		case "trojan":
			sb.WriteString(fmt.Sprintf("  - name: \"%s\"\n", node.Name))
			sb.WriteString("    type: trojan\n")
			sb.WriteString(fmt.Sprintf("    server: %s\n", node.Address))
			sb.WriteString(fmt.Sprintf("    port: %d\n", node.Port))
			sb.WriteString(fmt.Sprintf("    password: \"rf-%d-pass\"\n", user.ID))
			sb.WriteString("    udp: true\n\n")
		}
	}

	sb.WriteString("proxy-groups:\n")
	sb.WriteString("  - name: Proxy\n")
	sb.WriteString("    type: url-test\n")
	sb.WriteString("    proxies:\n")
	for _, node := range *nodes {
		sb.WriteString(fmt.Sprintf("      - %s\n", node.Name))
	}
	sb.WriteString("    url: http://www.gstatic.com/generate_204\n")
	sb.WriteString("    interval: 300\n\n")

	sb.WriteString("rules:\n")
	sb.WriteString("  - GEOIP,CN,DIRECT\n")
	sb.WriteString("  - MATCH,Proxy\n")

	c.Set("Content-Type", "text/yaml; charset=utf-8")
	return c.SendString(sb.String())
}

func handleSingboxFormat(c *fiber.Ctx, user *model.User, nodes *[]model.Node) error {
	type singOutbound struct {
		Tag      string      `json:"tag"`
		Protocol string      `json:"protocol"`
		Settings interface{} `json:"settings,omitempty"`
	}

	var outbounds []singOutbound
	for _, node := range *nodes {
		outbounds = append(outbounds, singOutbound{
			Tag:      node.Name,
			Protocol: node.Protocol,
		})
	}

	config := fiber.Map{
		"outbounds": outbounds,
		"route": fiber.Map{
			"rules": []fiber.Map{
				{"geoip": "cn", "outbound": "direct"},
				{"geosite": "cn", "outbound": "direct"},
			},
			"final": "select",
		},
	}

	c.Set("Content-Type", "application/json; charset=utf-8")
	return c.JSON(config)
}

// apiBaseURL returns the public origin used to build subscription URLs.
// API_PUBLIC_URL (documented in manager.env.example) takes precedence so
// links stay correct behind a TLS-terminating proxy: Fiber does not trust
// X-Forwarded-* headers in this app, so c.BaseURL() would report the
// internal http origin there. c.BaseURL() is used as a fallback so local
// smoke tests work without extra config, and the final default matches the
// hardcoded geoip/geosite origins in GetSubscription.
func apiBaseURL(c *fiber.Ctx) string {
	if base := strings.TrimRight(os.Getenv("API_PUBLIC_URL"), "/"); base != "" {
		return base
	}
	if origin := c.BaseURL(); origin != "" {
		return origin
	}
	return "https://api.rfplay.uk"
}

func handleQRCodeFormat(c *fiber.Ctx, user *model.User, nodes *[]model.Node) error {
	var lines []string
	for _, node := range *nodes {
		uri := EncodeNodeToURI(&node, user)
		if uri != "" {
			lines = append(lines, uri)
		}
	}
	if len(lines) == 0 {
		return c.Status(fiber.StatusNoContent).Send(nil)
	}

	// Encode the subscription URL so clients can import it by scanning.
	// The Subscription-Userinfo etc. headers set in handleLinksRequest
	// are intentionally kept for compatibility with clients that read them.
	subURL := fmt.Sprintf("%s/api/v1/client/links/%s", apiBaseURL(c), c.Params("token"))
	png, err := qrcode.Encode(subURL, qrcode.Medium, 256)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate QR code",
		})
	}

	c.Set("Content-Type", "image/png")
	return c.Send(png)
}
