package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

// EncodeNodeToURI encodes a node + user into a proxy URI string.
// Credentials come from the per-user random values stored in the DB
// (set at registration time by ensureUserCredentials).
func EncodeNodeToURI(node *model.Node, user *model.User) string {
	switch node.Protocol {
	case "vmess":
		return encodeVmess(node, user)
	case "vless":
		return encodeVless(node, user)
	case "shadowsocks":
		return encodeShadowsocks(node, user)
	case "trojan":
		return encodeTrojan(node, user)
	default:
		return ""
	}
}

func encodeVmess(node *model.Node, user *model.User) string {
	vmessData := map[string]interface{}{
		"add":  node.Address,
		"port": node.Port,
		"id":   user.VlessUUID,
		"aid":  0,
		"net":  "ws",
		"type": "none",
		"v":    "2",
		"ps":   node.Name,
	}

	data, err := json.Marshal(vmessData)
	if err != nil {
		return ""
	}

	return "vmess://" + base64.StdEncoding.EncodeToString(data)
}

func encodeVless(node *model.Node, user *model.User) string {
	params := url.Values{}
	params.Set("type", "tcp")
	params.Set("security", "reality")
	params.Set("flow", "xtls-rprx-vision")
	params.Set("sni", node.Address)
	params.Set("fp", "chrome")
	params.Set("pbk", node.RealtyPublicKey)
	params.Set("sid", node.RealtyShortID)

	return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		user.VlessUUID, node.Address, node.Port, params.Encode(), url.QueryEscape(node.Name))
}

func encodeShadowsocks(node *model.Node, user *model.User) string {
	ssStr := fmt.Sprintf("aes-256-gcm:%s@%s:%d", user.SSPassword, node.Address, node.Port)
	encoded := base64.StdEncoding.EncodeToString([]byte(ssStr))
	return fmt.Sprintf("ss://%s#%s", encoded, url.QueryEscape(node.Name))
}

func encodeTrojan(node *model.Node, user *model.User) string {
	return fmt.Sprintf("trojan://%s@%s:%d?security=tls&sni=%s#%s",
		user.TrojanPassword, node.Address, node.Port, node.Address, url.QueryEscape(node.Name))
}
