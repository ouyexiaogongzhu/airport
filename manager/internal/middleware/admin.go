package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// AdminOnly checks that the authenticated user has role=admin.
// Must be placed after JWTProtected().
func AdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok || role != "admin" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "admin access required",
			})
		}
		return c.Next()
	}
}
