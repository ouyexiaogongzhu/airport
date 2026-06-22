package verify

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ouyexiaogongzhu/airport/xray-core/ratelimit"
)

// Server handles inbound connection verification by calling the Manager API.
type Server struct {
	// ManagerURL is the base URL of the Manager API (e.g. http://localhost:8080).
	ManagerURL string
	// Limiter is an optional per-user rate limiter.
	Limiter *ratelimit.UserRateLimiter
	// HTTP client with sensible timeout.
	client *http.Client
}

// VerifyRequest is the payload sent by Xray inbound handler when a new connection arrives.
type VerifyRequest struct {
	// Token is the user's authentication token from the inbound connection.
	Token string `json:"token"`
	// UserID is optional; if empty the server looks it up from the token.
	UserID string `json:"user_id,omitempty"`
	// RemoteAddr is the IP address of the connecting client.
	RemoteAddr string `json:"remote_addr,omitempty"`
	// InboundTag identifies which inbound handler received the connection.
	InboundTag string `json:"inbound_tag,omitempty"`
}

// VerifyResponse is returned by the Manager API.
type VerifyResponse struct {
	Valid   bool   `json:"valid"`
	UserID  uint   `json:"user_id,omitempty"`
	TrafficUp   int64 `json:"traffic_up,omitempty"`
	TrafficDown int64 `json:"traffic_down,omitempty"`
	Message string `json:"message,omitempty"`
}

// NewVerifyRequest is the traffic report sent back to the Manager after verification.
type NewVerifyRequest struct {
	NodeID         uint  `json:"node_id"`
	UserID         uint  `json:"user_id"`
	UploadBytes    int64 `json:"upload_bytes"`
	DownloadBytes  int64 `json:"download_bytes"`
}

func (s *Server) ensureClient() {
	if s.client == nil {
		s.client = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
}

// HandleVerify is called when Xray-core receives a new inbound connection.
// It calls the Manager API to validate the user's token and optionally
// enforces rate limits.
func (s *Server) HandleVerify(w http.ResponseWriter, r *http.Request) {
	s.ensureClient()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req VerifyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Token == "" {
		s.respondJSON(w, VerifyResponse{Valid: false, Message: "token is required"})
		return
	}

	// Step 1: Verify token with Manager API
	verifyResp, err := s.callManagerVerify(req.Token)
	if err != nil {
		log.Printf("[verify] manager verify call failed: %v", err)
		s.respondJSON(w, VerifyResponse{Valid: false, Message: "verification service unavailable"})
		return
	}

	if !verifyResp.Valid {
		log.Printf("[verify] token rejected: %s", req.Token[:min(8, len(req.Token))])
		s.respondJSON(w, verifyResp)
		return
	}

	// Step 2: Rate limit check (if enabled)
	if s.Limiter != nil {
		userID := fmt.Sprintf("%d", verifyResp.UserID)
		if !s.Limiter.Allow(userID) {
			log.Printf("[verify] rate limit exceeded for user %s", userID)
			s.respondJSON(w, VerifyResponse{
				Valid:   false,
				UserID:  verifyResp.UserID,
				Message: "rate limit exceeded",
			})
			return
		}
	}

	// Step 3: Report initial traffic (connection established)
	err = s.reportTraffic(verifyResp.UserID)
	if err != nil {
		// Non-fatal: connection proceeds but log the error
		log.Printf("[verify] traffic report failed: %v", err)
	}

	log.Printf("[verify] connection approved: user=%d token=%.8s", verifyResp.UserID, req.Token)
	s.respondJSON(w, verifyResp)
}

// callManagerVerify sends the token to the Manager API for validation.
func (s *Server) callManagerVerify(token string) (VerifyResponse, error) {
	url := fmt.Sprintf("%s/api/v1/admin/traffic/report", strings.TrimRight(s.ManagerURL, "/"))

	// For verification we send a GET with the token as a query param
	// Alternatively, the Manager could have a dedicated /verify endpoint.
	// Here we use the traffic/report flow: we report and get user status.
	// In a production system, there would be a dedicated /api/v1/auth/verify endpoint.
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return VerifyResponse{}, fmt.Errorf("create request: %w", err)
	}
	q := req.URL.Query()
	q.Set("token", token)
	req.URL.RawQuery = q.Encode()

	resp, err := s.client.Do(req)
	if err != nil {
		return VerifyResponse{}, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return VerifyResponse{Valid: false, Message: "manager rejected token"}, nil
	}

	var verifyResp VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyResp); err != nil {
		return VerifyResponse{}, fmt.Errorf("decode response: %w", err)
	}

	return verifyResp, nil
}

// reportTraffic sends a traffic report to the Manager for the connected user.
func (s *Server) reportTraffic(userID uint) error {
	url := fmt.Sprintf("%s/api/v1/admin/traffic/report", strings.TrimRight(s.ManagerURL, "/"))

	report := NewVerifyRequest{
		UserID:        userID,
		UploadBytes:   0,
		DownloadBytes: 0,
	}

	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := s.client.Post(url, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("manager returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (s *Server) respondJSON(w http.ResponseWriter, resp VerifyResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
