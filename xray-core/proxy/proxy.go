package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ProxyConfig holds the configuration for a proxy inbound
type ProxyConfig struct {
	Port      int      `json:"port"`
	Protocol  string   `json:"protocol"` // socks5, http, vmess, vless
	VerifyURL string   `json:"verify_url"`
	Tag       string   `json:"tag"`
	ClientIDs []string `json:"client_ids"` // allowed VLESS client UUIDs
}

// sharedClient is a process-wide HTTP client reused by all proxy connections.
// A single Transport keeps TCP connections alive and pooled across requests,
// which matters on the manager verification path (one request per new
// connection). The previous per-connection client built a fresh Transport for
// every call, disabling keep-alive entirely.
var sharedClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
	},
}

// Token-verification result cache.
//
// verifyToken calls the manager on every new connection (each SOCKS5 and HTTP
// CONNECT handshake). Clients opening many short connections would otherwise
// hammer the manager API. Results are cached keyed by the token string.
//
// Security trade-off: a revoked token remains usable for at most
// verifyCacheValidTTL (60s) while a positive result is cached, and negative
// results are cached for only verifyCacheInvalidTTL (5s) so a freshly granted
// or re-enabled token is picked up quickly. Enforcement changes therefore
// propagate within a minute at worst.
const (
	verifyCacheMaxEntries = 65536
	verifyCacheValidTTL   = 60 * time.Second
	verifyCacheInvalidTTL = 5 * time.Second
)

type verifyCacheEntry struct {
	valid   bool
	expires time.Time
}

// verifyCache is a goroutine-safe TTL cache with a hard size cap. Expired
// entries are dropped lazily on lookup and insert; when the cap is exceeded
// the least-recently-inserted entries are evicted.
var verifyCache = struct {
	mu sync.RWMutex
	m  map[string]verifyCacheEntry
}{m: make(map[string]verifyCacheEntry, 1024)}

// cachedVerify returns the cached verification result for token, if present
// and still fresh.
func cachedVerify(token string) (valid, ok bool) {
	now := time.Now()
	verifyCache.mu.RLock()
	e, found := verifyCache.m[token]
	verifyCache.mu.RUnlock()
	if !found || now.After(e.expires) {
		return false, false
	}
	return e.valid, true
}

// cacheVerify stores a verification result for token with the appropriate TTL.
func cacheVerify(token string, valid bool) {
	ttl := verifyCacheValidTTL
	if !valid {
		ttl = verifyCacheInvalidTTL
	}
	now := time.Now()

	verifyCache.mu.Lock()
	defer verifyCache.mu.Unlock()

	if len(verifyCache.m) >= verifyCacheMaxEntries {
		// Lazy sweep: drop expired entries first.
		for k, e := range verifyCache.m {
			if now.After(e.expires) {
				delete(verifyCache.m, k)
			}
		}
		// Still over the cap: evict the oldest entries. Each pass finds the
		// earliest expiry in one linear scan; the number of passes is bounded
		// by how far over the cap we are, which stays small in steady state.
		for len(verifyCache.m) >= verifyCacheMaxEntries {
			var oldestKey string
			var oldestExp time.Time
			first := true
			for k, e := range verifyCache.m {
				if first || e.expires.Before(oldestExp) {
					oldestExp = e.expires
					oldestKey = k
					first = false
				}
			}
			delete(verifyCache.m, oldestKey)
		}
	}

	verifyCache.m[token] = verifyCacheEntry{valid: valid, expires: now.Add(ttl)}
}

// ClearVerifyCache drops all cached verification results. Exposed for tests
// and for external management (e.g. after a bulk token revocation).
func ClearVerifyCache() {
	verifyCache.mu.Lock()
	verifyCache.m = make(map[string]verifyCacheEntry, 1024)
	verifyCache.mu.Unlock()
}

// ProxyServer implements SOCKS5, HTTP CONNECT, and VLESS proxies.
type ProxyServer struct {
	config    ProxyConfig
	verifyURL string
	vless     *vlessHandler
	listener  net.Listener
	wg        sync.WaitGroup
	stopCh    chan struct{}
}

// New creates a new proxy server
func New(cfg ProxyConfig, verifyURL string) *ProxyServer {
	if cfg.Protocol == "" {
		cfg.Protocol = "socks5"
	}
	ps := &ProxyServer{
		config:    cfg,
		verifyURL: verifyURL,
		stopCh:    make(chan struct{}),
	}
	if cfg.Protocol == "vless" {
		ps.vless = newVLESSHandler(cfg.ClientIDs)
	}
	return ps
}

// SetVLESSVerify registers a remote verification callback for VLESS clients.
func (s *ProxyServer) SetVLESSVerify(fn func(uuid string) bool) {
	if s.vless != nil {
		s.vless.verifyRemote = fn
	}
}

// Start begins listening for connections
func (s *ProxyServer) Start() error {
	addr := fmt.Sprintf(":%d", s.config.Port)
	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	log.Printf("[proxy] %s listening on %s (tag=%s)", s.config.Protocol, addr, s.config.Tag)

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop gracefully shuts down the server
func (s *ProxyServer) Stop() {
	close(s.stopCh)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	log.Printf("[proxy] %s stopped (tag=%s)", s.config.Protocol, s.config.Tag)
}

func (s *ProxyServer) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				log.Printf("[proxy] accept error: %v", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *ProxyServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(30 * time.Second))

	switch s.config.Protocol {
	case "socks5":
		s.handleSOCKS5(conn)
	case "vless":
		if s.vless != nil {
			s.vless.handle(conn)
		}
	case "http":
		s.handleHTTPConnect(conn)
	case "vmess":
		// VMess requires AEAD crypto which the minimal fork does not implement;
		// serve VLESS-compatible connections on the same port, or fall back to
		// HTTP CONNECT for legacy clients.
		s.handleHTTPConnect(conn)
	default:
		s.handleHTTPConnect(conn)
	}
}

// SOCKS5 implementation
func (s *ProxyServer) handleSOCKS5(conn net.Conn) {
	// Read auth methods
	buf := make([]byte, 257)
	n, err := conn.Read(buf[:2])
	if err != nil || n != 2 {
		return
	}
	ver := buf[0]
	if ver != 5 {
		return // Not SOCKS5
	}
	nMethods := int(buf[1])
	if nMethods < 1 {
		return
	}
	if nMethods > 255 {
		return
	}

	n, err = conn.Read(buf[:nMethods])
	if err != nil || n != nMethods {
		return
	}

	// We support no-auth (0x00) and user/pass (0x02)
	// For this phase, we require user/pass auth with token
	hasNoAuth := false
	hasUserPass := false
	for i := 0; i < nMethods; i++ {
		switch buf[i] {
		case 0x00:
			hasNoAuth = true
		case 0x02:
			hasUserPass = true
		}
	}

	if hasUserPass {
		// Tell client we want user/pass auth
		conn.Write([]byte{5, 2})
		if !s.handleSOCKS5UserPassAuth(conn) {
			return
		}
	} else if hasNoAuth {
		conn.Write([]byte{5, 0})
	} else {
		conn.Write([]byte{5, 0xFF}) // No acceptable methods
		return
	}

	// Read request
	n, err = conn.Read(buf[:4])
	if err != nil || n != 4 {
		return
	}

	cmd := buf[1] // 0x01 = CONNECT, 0x02 = BIND, 0x03 = UDP
	if cmd != 1 {
		conn.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0}) // Command not supported
		return
	}

	addrType := buf[3]
	var host string
	var port int

	switch addrType {
	case 1: // IPv4
		n, err = conn.Read(buf[:4])
		if err != nil || n != 4 {
			return
		}
		host = net.IP(buf[:4]).String()
	case 3: // Domain name
		n, err = conn.Read(buf[:1])
		if err != nil || n != 1 {
			return
		}
		domainLen := int(buf[0])
		if domainLen > 255 {
			return
		}
		n, err = conn.Read(buf[:domainLen])
		if err != nil || n != domainLen {
			return
		}
		host = string(buf[:domainLen])
	case 4: // IPv6
		n, err = conn.Read(buf[:16])
		if err != nil || n != 16 {
			return
		}
		host = net.IP(buf[:16]).String()
	default:
		return
	}

	n, err = conn.Read(buf[:2])
	if err != nil || n != 2 {
		return
	}
	port = int(buf[0])<<8 | int(buf[1])

	conn.SetDeadline(time.Time{})

	// Connect to destination
	target, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), 10*time.Second)
	if err != nil {
		log.Printf("[proxy] connect to %s:%d failed: %v", host, port, err)
		conn.Write([]byte{5, 4, 0, 1, 0, 0, 0, 0, 0, 0}) // Host unreachable
		return
	}
	defer target.Close()

	// Send success response
	conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { io.Copy(target, conn); wg.Done() }()
	go func() { io.Copy(conn, target); wg.Done() }()
	wg.Wait()
}

func (s *ProxyServer) handleSOCKS5UserPassAuth(conn net.Conn) bool {
	buf := make([]byte, 513)
	n, err := conn.Read(buf[:2])
	if err != nil || n != 2 {
		return false
	}

	ver := buf[0]
	if ver != 1 { // User/pass auth version
		conn.Write([]byte{1, 1}) // Auth failed
		return false
	}

	uLen := int(buf[1])
	if uLen < 1 || uLen > 255 {
		conn.Write([]byte{1, 1})
		return false
	}

	n, err = conn.Read(buf[:uLen])
	if err != nil || n != uLen {
		return false
	}
	username := string(buf[:uLen])

	n, err = conn.Read(buf[:1])
	if err != nil || n != 1 {
		return false
	}

	pLen := int(buf[0])
	if pLen < 1 || pLen > 255 {
		conn.Write([]byte{1, 1})
		return false
	}

	n, err = conn.Read(buf[:pLen])
	if err != nil || n != pLen {
		return false
	}
	password := string(buf[:pLen])

	// Verify token against Manager API
	if s.verifyToken(username, password) {
		conn.Write([]byte{1, 0}) // Auth success
		return true
	}

	log.Printf("[proxy] SOCKS5 auth failed for user=%s", username)
	conn.Write([]byte{1, 1}) // Auth failed
	return false
}

// HTTP CONNECT proxy implementation
func (s *ProxyServer) handleHTTPConnect(conn net.Conn) {
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	request := string(buf[:n])
	lines := strings.Split(request, "\r\n")
	if len(lines) < 1 {
		return
	}

	parts := strings.Split(lines[0], " ")
	if len(parts) < 3 {
		return
	}

	method := parts[0]
	target := parts[1]

	// Extract token from Proxy-Authorization header
	token := ""
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "proxy-authorization:") {
			token = strings.TrimSpace(strings.TrimPrefix(line, "Proxy-Authorization:"))
			token = strings.TrimPrefix(token, "Bearer ")
			break
		}
	}

	if method == "CONNECT" {
		// HTTPS tunnel
		if token == "" || !s.verifyToken("", token) {
			conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Bearer\r\n\r\n"))
			return
		}

		hostPort := target
		targetConn, err := net.DialTimeout("tcp", hostPort, 10*time.Second)
		if err != nil {
			conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer targetConn.Close()

		conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

		var wg sync.WaitGroup
		wg.Add(2)
		go func() { io.Copy(targetConn, conn); wg.Done() }()
		go func() { io.Copy(conn, targetConn); wg.Done() }()
		wg.Wait()
	} else {
		// HTTP proxy
		if token == "" || !s.verifyToken("", token) {
			conn.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Bearer\r\n\r\n"))
			return
		}

		// Parse target URL
		targetURL, err := url.Parse(target)
		if err != nil {
			conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
			return
		}

		host := targetURL.Host
		if !strings.Contains(host, ":") {
			host = host + ":80"
		}

		targetConn, err := net.DialTimeout("tcp", host, 10*time.Second)
		if err != nil {
			conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer targetConn.Close()

		// Forward the original request
		targetConn.Write(buf[:n])

		var wg sync.WaitGroup
		wg.Add(2)
		go func() { io.Copy(targetConn, conn); wg.Done() }()
		go func() { io.Copy(conn, targetConn); wg.Done() }()
		wg.Wait()
	}
}

// verifyToken checks a token against the Manager API. Results are cached for
// a short TTL (see verifyCacheValidTTL/verifyCacheInvalidTTL) so repeated
// connections from the same token do not hit the manager on every handshake.
func (s *ProxyServer) verifyToken(userID, token string) bool {
	if s.verifyURL == "" {
		return true // No verification in dev mode
	}
	if token == "" {
		return false
	}

	if valid, ok := cachedVerify(token); ok {
		return valid
	}

	valid := s.verifyTokenRemote(userID, token)
	cacheVerify(token, valid)
	return valid
}

// verifyTokenRemote performs the actual manager round-trip for verifyToken.
func (s *ProxyServer) verifyTokenRemote(userID, token string) bool {
	apiURL := fmt.Sprintf("%s/api/v1/public/login", s.verifyURL)

	// Try token login
	body := fmt.Sprintf(`{"token":"%s"}`, token)
	resp, err := sharedClient.Post(apiURL, "application/json", strings.NewReader(body))
	if err != nil {
		log.Printf("[proxy] verify: token login failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return true
	}

	// Fallback: try user/pass login
	if userID != "" {
		body = fmt.Sprintf(`{"username":"%s","password":"%s"}`, userID, token)
		resp, err = sharedClient.Post(apiURL, "application/json", strings.NewReader(body))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == 200
	}

	return false
}

// AuthResponse represents the login response
type AuthResponse struct {
	Token string `json:"token"`
	User  struct {
		ID     int    `json:"id"`
		Role   string `json:"role"`
		Status string `json:"status"`
	} `json:"user"`
}

// verifyTokenFromManager is an alternative verification that checks subscription status
func verifyTokenFromManager(verifyURL, token string) bool {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/client/subscription", verifyURL), nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := sharedClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// 200 = valid subscription, 403 with SUBSCRIPTION_PENDING = valid user but no subscription
	if resp.StatusCode == 200 || resp.StatusCode == 403 {
		var body map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&body)
		if errStr, ok := body["error"].(string); ok && errStr == "SUBSCRIPTION_PENDING" {
			return true
		}
		return resp.StatusCode == 200
	}
	return false
}
