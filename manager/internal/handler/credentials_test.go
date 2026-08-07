package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

func TestEnsureUserCredentials(t *testing.T) {
	user := &model.User{ID: 1}
	ensureUserCredentials(user)

	if user.VlessUUID == "" {
		t.Fatal("expected vless_uuid to be generated")
	}
	if user.SSPassword == "" {
		t.Fatal("expected ss_password to be generated")
	}
	if user.TrojanPassword == "" {
		t.Fatal("expected trojan_password to be generated")
	}

	// Re-running must not overwrite existing credentials.
	first := user.VlessUUID
	ensureUserCredentials(user)
	if user.VlessUUID != first {
		t.Fatalf("expected credentials to be stable, got %s then %s", first, user.VlessUUID)
	}
}

func TestEncodeNodeToURI_UsesStoredCredentials(t *testing.T) {
	user := &model.User{
		ID:             42,
		VlessUUID:      "11111111-2222-3333-4444-555555555555",
		SSPassword:     "ss-secret-password",
		TrojanPassword: "trojan-secret-password",
	}
	node := &model.Node{
		Name:     "HK-01",
		Address:  "hk.example.com",
		Port:     443,
		Protocol: "vless",
	}

	uri := EncodeNodeToURI(node, user)
	if uri == "" {
		t.Fatal("expected non-empty vless uri")
	}
	if !strings.Contains(uri, user.VlessUUID) {
		t.Fatalf("expected uri to embed stored vless_uuid, got: %s", uri)
	}
	if strings.Contains(uri, "2a000000") { // derived-from-id pattern must be gone
		t.Fatalf("expected uri not to derive uuid from user id: %s", uri)
	}

	ssNode := &model.Node{Address: "ss.example.com", Port: 8388, Protocol: "shadowsocks"}
	ssURI := EncodeNodeToURI(ssNode, user)
	ssPart := strings.TrimPrefix(ssURI, "ss://")
	if idx := strings.Index(ssPart, "#"); idx >= 0 {
		ssPart = ssPart[:idx]
	}
	ssDecoded, err := base64.StdEncoding.DecodeString(ssPart)
	if err != nil {
		t.Fatalf("failed to decode ss uri: %v", err)
	}
	if !strings.Contains(string(ssDecoded), "ss-secret-password") {
		t.Fatalf("expected ss uri to embed stored password, got: %s", ssURI)
	}

	trojanNode := &model.Node{Address: "tr.example.com", Port: 443, Protocol: "trojan"}
	trURI := EncodeNodeToURI(trojanNode, user)
	if !strings.Contains(trURI, "trojan-secret-password") {
		t.Fatalf("expected trojan uri to embed stored password, got: %s", trURI)
	}
}

func TestGenerateNodeConfig_AdminEndpoint(t *testing.T) {
	setupTestDB(t)
	user := createTestUserWithCreds(t)
	activeUser := createActiveTestUser(t)
	node := model.Node{
		Name:     "Test-Node",
		Type:     "xray",
		Address:  "node.example.com",
		Port:     443,
		Protocol: "vless",
		Status:   "active",
		UserID:   user.ID,
		Network:  "ws",
		WSPath:   "/ws",
	}
	if result := db.DB.Create(&node); result.Error != nil {
		t.Fatalf("failed to create node: %v", result.Error)
	}

	app := fiber.New()
	app.Get("/api/v1/admin/nodes/:id/config", GenerateNodeConfig)
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/admin/nodes/%d/config", node.ID), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var config map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	inbounds, ok := config["inbounds"].([]interface{})
	if !ok || len(inbounds) != 1 {
		t.Fatalf("expected 1 inbound, got %v", config["inbounds"])
	}
	inbound := inbounds[0].(map[string]interface{})
	if inbound["protocol"] != "vless" {
		t.Fatalf("expected vless protocol, got %v", inbound["protocol"])
	}

	settings := inbound["settings"].(map[string]interface{})
	clients := settings["clients"].([]interface{})
	if len(clients) != 1 {
		t.Fatalf("expected 1 active user client, got %d", len(clients))
	}
	client := clients[0].(map[string]interface{})
	if client["id"] != activeUser.VlessUUID {
		t.Fatalf("expected client uuid %s, got %v", activeUser.VlessUUID, client["id"])
	}

	stream := inbound["streamSettings"].(map[string]interface{})
	if stream["network"] != "ws" {
		t.Fatalf("expected ws network, got %v", stream["network"])
	}

	meta := config["_meta"].(map[string]interface{})
	userIDs, ok := meta["user_ids"].([]interface{})
	if !ok || len(userIDs) != 1 {
		t.Fatalf("expected _meta.user_ids to contain the active user, got %v", meta["user_ids"])
	}
}

func createTestUserWithCreds(t *testing.T) *model.User {
	t.Helper()
	user := model.User{
		Username:     "credsuser" + randomHex(4),
		PasswordHash: "hash",
		Role:         "user",
		Status:       "active",
		ClientToken:  "rf_" + randomHex(16),
	}
	ensureUserCredentials(&user)
	if result := db.DB.Create(&user); result.Error != nil {
		t.Fatalf("failed to create user: %v", result.Error)
	}
	return &user
}

func createActiveTestUser(t *testing.T) *model.User {
	t.Helper()
	user := model.User{
		Username:           "activeuser" + randomHex(4),
		PasswordHash:       "hash",
		Role:               "user",
		Status:             "active",
		ClientToken:        "rf_" + randomHex(16),
		SubscriptionStatus: "active",
		ExpireTime:         time.Now().Unix() + 86400,
	}
	ensureUserCredentials(&user)
	if result := db.DB.Create(&user); result.Error != nil {
		t.Fatalf("failed to create user: %v", result.Error)
	}
	return &user
}
