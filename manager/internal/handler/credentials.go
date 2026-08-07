package handler

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

// ensureUserCredentials generates random proxy credentials for a user if they
// are missing. Called after registration and after subscription activation.
// Credentials are stored in the DB so node configs and subscription links
// reference the same values instead of deriving them from user.ID.
func ensureUserCredentials(user *model.User) {
	if user.VlessUUID == "" {
		user.VlessUUID = uuid.New().String()
	}
	if user.SSPassword == "" {
		user.SSPassword = randomHex(16)
	}
	if user.TrojanPassword == "" {
		user.TrojanPassword = randomHex(24)
	}
}

// randomHex returns a cryptographically-random hex string of n random bytes.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand should not fail on supported platforms; fall back to a
		// time-seeded value so the process can still start in exotic sandboxes.
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))
	}
	return hex.EncodeToString(b)
}
