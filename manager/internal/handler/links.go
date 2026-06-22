package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

// EncodeNodeToURI encodes a node + user into a proxy URI string
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
	uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", user.ID, 0, 0, 0, user.ID*100)

	vmessData := map[string]interface{}{
		"add":  node.Address,
		"port": node.Port,
		"id":   uuid,
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
	uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", user.ID, 0, 0, 0, user.ID*100)

	params := url.Values{}
	params.Set("type", "tcp")
	params.Set("security", "reality")
	params.Set("flow", "xtls-rprx-vision")
	params.Set("sni", node.Address)
	params.Set("fp", "chrome")
	params.Set("pbk", "")
	params.Set("sid", "")

	return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		uuid, node.Address, node.Port, params.Encode(), url.QueryEscape(node.Name))
}

func encodeShadowsocks(node *model.Node, user *model.User) string {
	ssStr := fmt.Sprintf("aes-256-gcm:rf-%d-pass@%s:%d", user.ID, node.Address, node.Port)
	encoded := base64.StdEncoding.EncodeToString([]byte(ssStr))
	return fmt.Sprintf("ss://%s#%s", encoded, url.QueryEscape(node.Name))
}

func encodeTrojan(node *model.Node, user *model.User) string {
	return fmt.Sprintf("trojan://rf-%d-pass@%s:%d?security=tls&sni=%s#%s",
		user.ID, node.Address, node.Port, node.Address, url.QueryEscape(node.Name))
}
