package middleware

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// WebAuth authenticates web sessions from an httpOnly cookie instead of a
// Bearer header. The validation logic mirrors JWTProtected: HS256 signature,
// exp claim, HMAC secret from JWT_SECRET. Only the cookie is trusted, so a
// Flutter client (Bearer on /client/*) is never accepted on web routes.
func WebAuth(cookieName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenStr := c.Cookies(cookieName)
		if tokenStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "SESSION_EXPIRED",
			})
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fiber.ErrUnauthorized
			}
			return getJWTSecret(), nil
		})
		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "SESSION_EXPIRED",
			})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "SESSION_EXPIRED",
			})
		}

		if userID, ok := claims["user_id"].(float64); ok {
			c.Locals("user_id", uint(userID))
		}
		if username, ok := claims["username"].(string); ok {
			c.Locals("username", username)
		}
		if role, ok := claims["role"].(string); ok {
			c.Locals("role", role)
		}

		return c.Next()
	}
}

// WebCSRF enforces double-submit cookie protection for unsafe methods: the
// X-CSRF-Token header must equal the csrf cookie value, compared in
// constant time. GET/HEAD/OPTIONS requests are skipped.
func WebCSRF(cookieName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}

		cookieVal := c.Cookies(cookieName)
		headerVal := c.Get("X-CSRF-Token")
		if cookieVal == "" || headerVal == "" ||
			subtle.ConstantTimeCompare([]byte(cookieVal), []byte(headerVal)) != 1 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "CSRF_INVALID",
			})
		}
		return c.Next()
	}
}
