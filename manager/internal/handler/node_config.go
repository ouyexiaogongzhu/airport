package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
	"gorm.io/gorm"
)

// ensureNodeToken generates a random auth token for a node daemon if missing.
func ensureNodeToken(node *model.Node) string {
	if node.Token == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err == nil {
			node.Token = "nd_" + hex.EncodeToString(b)
		}
	}
	return node.Token
}

// GenerateNodeToken explicitly rotates a node's daemon auth token.
// POST /api/v1/admin/nodes/:id/token
func GenerateNodeToken(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid node id"})
	}

	var node model.Node
	if result := db.DB.First(&node, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "node not found"})
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to generate token"})
	}
	node.Token = "nd_" + hex.EncodeToString(b)
	if err := db.DB.Model(&node).Update("token", node.Token).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save token"})
	}

	return c.JSON(fiber.Map{"token": node.Token})
}

// GenerateNodeConfig builds a standard Xray JSON config for a single node.
// The inbound contains every active subscriber as a VLESS/VMess client, so the
// node runs real Xray-core without any per-connection auth round-trip.
// GET /api/v1/admin/nodes/:id/config
func GenerateNodeConfig(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid node id"})
	}

	var node model.Node
	if result := db.DB.First(&node, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "node not found"})
	}
	if node.Status != "active" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "NODE_DISABLED"})
	}

	config, err := BuildNodeXrayConfig(&node, db.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Touch heartbeat so the admin panel can show node liveness.
	now := time.Now()
	if err := db.DB.Model(&node).Update("last_heartbeat", &now).Error; err != nil {
		// non-fatal
	}

	return c.JSON(config)
}

// nodeConfigResult is returned to the daemon. It wraps the xray JSON plus
// metadata the daemon needs (node id, expected version for change detection).
type nodeConfigResult struct {
	NodeID   uint                   `json:"node_id"`
	Name     string                 `json:"name"`
	Protocol string                 `json:"protocol"`
	Config   map[string]interface{} `json:"config"`
}

// GetNodeConfigForDaemon authenticates a node daemon by its token (enforced by
// the NodeHMAC middleware) and returns the node's Xray config. Used by the
// daemon's config pull.
// GET /api/v1/node/:token/config
func GetNodeConfigForDaemon(c *fiber.Ctx) error {
	node, ok := c.Locals("node").(*model.Node)
	if !ok || node == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "INVALID_TOKEN"})
	}
	if node.Status != "active" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "NODE_DISABLED"})
	}

	config, err := BuildNodeXrayConfig(node, db.DB)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	now := time.Now()
	db.DB.Model(node).Update("last_heartbeat", &now)

	return c.JSON(nodeConfigResult{
		NodeID:   node.ID,
		Name:     node.Name,
		Protocol: node.Protocol,
		Config:   config,
	})
}

// BuildNodeXrayConfig assembles a standard Xray JSON document for the given
// node using the passed DB handle (kept parameterised for tests).
func BuildNodeXrayConfig(node *model.Node, gormDB *gorm.DB) (map[string]interface{}, error) {
	var users []model.User
	now := time.Now().Unix()
	// Only include users with an active subscription that has not expired.
	if err := gormDB.Where("subscription_status = ? AND (expire_time = 0 OR expire_time > ?)", "active", now).
		Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to load users: %w", err)
	}

	clients := make([]fiber.Map, 0, len(users))
	userIDs := make([]uint, 0, len(users))
	for _, u := range users {
		ensureUserCredentials(&u)
		if u.VlessUUID == "" {
			continue
		}
		clients = append(clients, fiber.Map{
			"id":  u.VlessUUID,
			"flow": "xtls-rprx-vision",
		})
		userIDs = append(userIDs, u.ID)
	}

	network := node.Network
	if network == "" {
		network = "tcp"
	}

	streamSettings := fiber.Map{
		"network": network,
	}
	switch node.Security {
	case "tls":
		streamSettings["security"] = "tls"
		streamSettings["tlsSettings"] = fiber.Map{
			"certificates": []fiber.Map{{
				"certificateFile": "/etc/xray/tls.crt",
				"keyFile":         "/etc/xray/tls.key",
			}},
		}
	case "reality":
		streamSettings["security"] = "reality"
		streamSettings["realitySettings"] = fiber.Map{
			"show":            false,
			"dest":            fmt.Sprintf("%s:443", firstNonEmpty(node.ServerName, node.Address)),
			"xver":            0,
			"serverNames":     []string{firstNonEmpty(node.ServerName, node.Address)},
			"privateKey":      "", // filled by the node operator via env XRAY_REALITY_PRIVATE_KEY
			"shortIds":        []string{firstNonEmpty(node.RealtyShortID, "")},
			"maxTimeDiff":     0,
			"minClientVer":    "",
			"maxClientVer":    "",
			"handshake":       nil,
			"decryption":      "none",
			"settings":        nil,
		}
	default:
		streamSettings["security"] = "none"
	}

	if network == "ws" {
		path := node.WSPath
		if path == "" {
			path = "/"
		}
		streamSettings["wsSettings"] = fiber.Map{
			"path":    path,
			"headers": fiber.Map{"Host": firstNonEmpty(node.ServerName, node.Address)},
		}
	}

	inbound := fiber.Map{
		"tag":      "in-"+node.Protocol,
		"port":     node.Port,
		"protocol": node.Protocol,
		"settings": fiber.Map{
			"clients":    clients,
			"decryption": "none",
			"fallbacks":  []interface{}{},
		},
		"streamSettings": streamSettings,
	}

	config := fiber.Map{
		"log": fiber.Map{
			"loglevel": "info",
			"access":   "/var/log/xray/access.log",
			"error":    "/var/log/xray/error.log",
		},
		"inbounds":  []fiber.Map{inbound},
		"outbounds": []fiber.Map{{
			"protocol": "freedom",
			"tag":      "direct",
		}},
		"routing": fiber.Map{
			"domainStrategy": "IPIfNonMatch",
			"rules": []fiber.Map{{
				"type":        "field",
				"inboundTag":  []string{"in-"+node.Protocol},
				"outboundTag": "direct",
			}},
		},
		"stats": fiber.Map{},
		"policy": fiber.Map{
			"levels": fiber.Map{
				"0": fiber.Map{
					"handshake":     4,
					"connIdle":      300,
					"uplinkOnly":    1,
					"downlinkOnly":  1,
					"statsUserUplink":   true,
					"statsUserDownlink": true,
				},
			},
		},
	}

	// Embed a comment map so the daemon can verify the config covers the
	// right user set without parsing protocol internals. version is derived
	// from the user id set so it is stable across pulls (the daemon uses it
	// for change detection; a time-based value would force reloads every tick).
	config["_meta"] = fiber.Map{
		"node_id":  node.ID,
		"user_ids": userIDs,
		"version":  userSetVersion(userIDs),
	}

	return config, nil
}

// userSetVersion returns a stable version token for a set of user ids.
func userSetVersion(userIDs []uint) int64 {
	var h uint64 = 14695981039346656037
	for _, id := range userIDs {
		h ^= uint64(id)
		h *= 1099511628211
	}
	return int64(h)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
