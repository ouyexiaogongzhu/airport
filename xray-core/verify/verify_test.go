package verify

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ouyexiaogongzhu/airport/xray-core/ratelimit"
)

// setupTestServer creates a verify.Server pointed at the given mock manager URL.
// The limiter is nil unless specified otherwise by the caller.
func setupTestServer(t *testing.T, mockManagerURL string) *Server {
	t.Helper()
	return &Server{
		ManagerURL: mockManagerURL,
		Limiter:    nil,
	}
}

// TestHandleVerify_InvalidMethod verifies that a non-POST request returns 405.
func TestHandleVerify_InvalidMethod(t *testing.T) {
	server := setupTestServer(t, "http://localhost:9999")

	req := httptest.NewRequest(http.MethodGet, "/verify", nil)
	w := httptest.NewRecorder()

	server.HandleVerify(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// TestHandleVerify_EmptyToken verifies that a POST with an empty token JSON
// returns 200 with {valid:false, message:"token is required"}.
func TestHandleVerify_EmptyToken(t *testing.T) {
	server := setupTestServer(t, "http://localhost:9999")

	body := `{"token":""}`
	req := httptest.NewRequest(http.MethodPost, "/verify", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleVerify(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var vr VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if vr.Valid {
		t.Error("expected Valid=false for empty token")
	}
	if vr.Message != "token is required" {
		t.Errorf("expected message 'token is required', got %q", vr.Message)
	}
}

// TestHandleVerify_InvalidJSON verifies that malformed JSON body returns 400.
func TestHandleVerify_InvalidJSON(t *testing.T) {
	server := setupTestServer(t, "http://localhost:9999")

	body := `{not-json}`
	req := httptest.NewRequest(http.MethodPost, "/verify", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleVerify(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// TestHandleVerify_ManagerUnreachable verifies that when the manager is down,
// the server returns 200 with {valid:false, message:"verification service unavailable"}.
func TestHandleVerify_ManagerUnreachable(t *testing.T) {
	// Point at a port where nothing is listening
	server := setupTestServer(t, "http://localhost:1")

	body := `{"token":"some-token"}`
	req := httptest.NewRequest(http.MethodPost, "/verify", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleVerify(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var vr VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if vr.Valid {
		t.Error("expected Valid=false when manager is unreachable")
	}
	if vr.Message != "verification service unavailable" {
		t.Errorf("expected message 'verification service unavailable', got %q", vr.Message)
	}
}

// TestHandleVerify_Success verifies that a valid token, when the mock manager
// returns valid=true, produces a 200 with {valid:true, user_id:N}.
func TestHandleVerify_Success(t *testing.T) {
	// Create a mock manager server that returns a valid response
	mockManager := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// callManagerVerify sends GET, reportTraffic sends POST — both hit this handler
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := VerifyResponse{
			Valid:  true,
			UserID: 42,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockManager.Close()

	server := setupTestServer(t, mockManager.URL)

	body := `{"token":"valid-token-12345"}`
	req := httptest.NewRequest(http.MethodPost, "/verify", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleVerify(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var vr VerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !vr.Valid {
		t.Error("expected Valid=true for successful verification")
	}
	if vr.UserID != 42 {
		t.Errorf("expected UserID=42, got %d", vr.UserID)
	}
}

// TestHandleVerify_RateLimited verifies that when the rate limiter is configured,
// exceeding the limit returns 200 with {valid:false, message:"rate limit exceeded"}.
func TestHandleVerify_RateLimited(t *testing.T) {
	// Create a mock manager server that always returns valid
	mockManager := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := VerifyResponse{
			Valid:  true,
			UserID: 7,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockManager.Close()

	// Rate limiter: 1 token per second, burst 1
	limiter := ratelimit.NewUserRateLimiter(1, 1)

	server := &Server{
		ManagerURL: mockManager.URL,
		Limiter:    limiter,
	}

	sendVerify := func(t *testing.T) *VerifyResponse {
		t.Helper()
		body := `{"token":"rate-limited-token"}`
		req := httptest.NewRequest(http.MethodPost, "/verify", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.HandleVerify(w, req)
		var vr VerifyResponse
		if err := json.NewDecoder(w.Result().Body).Decode(&vr); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		return &vr
	}

	// First request should succeed (burst=1)
	vr1 := sendVerify(t)
	if !vr1.Valid {
		t.Error("expected first request to be valid (within rate limit)")
	}

	// Second immediate request should be rate limited (burst exhausted)
	vr2 := sendVerify(t)
	if vr2.Valid {
		t.Error("expected second request to be rate limited")
	}
	if vr2.Message != "rate limit exceeded" {
		t.Errorf("expected message 'rate limit exceeded', got %q", vr2.Message)
	}
	if vr2.UserID != 7 {
		t.Errorf("expected UserID=7 in rate-limited response, got %d", vr2.UserID)
	}
}
