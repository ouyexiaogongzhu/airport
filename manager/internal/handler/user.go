package handler

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

// GetProfile returns the current authenticated user's profile.
func GetProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	var user model.User
	if result := db.DB.First(&user, userID); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	return c.JSON(user)
}

// UpdateProfile updates the current user's profile (password change etc.).
func UpdateProfile(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Only allow updating certain fields
	allowed := map[string]bool{}
	// (Future: add password change with old-password verification)
	_ = allowed

	if result := db.DB.Model(&model.User{}).Where("id = ?", userID).Updates(updates); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update profile",
		})
	}

	var user model.User
	db.DB.First(&user, userID)
	return c.JSON(user)
}

// ListUsers (admin only) returns all users.
func ListUsers(c *fiber.Ctx) error {
	var users []model.User
	if result := db.DB.Find(&users); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list users",
		})
	}
	return c.JSON(users)
}

// GetUser (admin only) returns a single user by ID.
func GetUser(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid user id",
		})
	}

	var user model.User
	if result := db.DB.First(&user, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found",
		})
	}
	return c.JSON(user)
}

// UpdateUserRequest holds optional fields for updating a user (admin only).
type UpdateUserRequest struct {
	ClientToken *string `json:"client_token"`
	Status      *string `json:"status"`
}

// UpdateUser (admin only) updates a user's client_token and/or status.
// If client_token is not provided or is empty, a new random token is generated.
func UpdateUser(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid user id",
		})
	}

	var user model.User
	if result := db.DB.First(&user, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "user not found",
		})
	}

	req := new(UpdateUserRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	updates := map[string]interface{}{}

	// If client_token is explicitly provided, use it; otherwise generate a new token
	if req.ClientToken != nil && *req.ClientToken != "" {
		updates["client_token"] = *req.ClientToken
	} else {
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err == nil {
			updates["client_token"] = "rf_" + hex.EncodeToString(tokenBytes)
		}
	}

	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if result := db.DB.Model(&user).Updates(updates); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update user",
		})
	}

	// Reload the updated user
	db.DB.First(&user, id)
	return c.JSON(user)
}
