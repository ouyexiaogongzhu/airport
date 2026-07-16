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
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	// TODO: Propagate request context via c.Context() for GORM queries.
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
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var updates map[string]interface{}
	if err := c.BodyParser(&updates); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Allowlist: only these fields can be updated by the user themselves
	allowed := map[string]bool{
		"username": true,
	}
	filtered := map[string]interface{}{}
	for key, val := range updates {
		if allowed[key] {
			filtered[key] = val
		}
	}

	if len(filtered) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "no valid fields to update",
		})
	}

	if result := db.DB.Model(&model.User{}).Where("id = ?", userID).Updates(filtered); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update profile",
		})
	}

	var user model.User
	db.DB.First(&user, userID)
	return c.JSON(user)
}

// parsePagination extracts page and per_page query params with defaults.
func parsePagination(c *fiber.Ctx) (offset, limit int) {
	page := c.QueryInt("page", 1)
	perPage := c.QueryInt("per_page", 20)
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return (page - 1) * perPage, perPage
}

// ListUsers (admin only) returns all users with pagination.
func ListUsers(c *fiber.Ctx) error {
	offset, limit := parsePagination(c)

	var total int64
	db.DB.Model(&model.User{}).Count(&total)

	var users []model.User
	if result := db.DB.Offset(offset).Limit(limit).Find(&users); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list users",
		})
	}
	return c.JSON(fiber.Map{
		"data":      users,
		"total":     total,
		"page":      offset/limit + 1,
		"per_page":  limit,
	})
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
		validStatuses := map[string]bool{"active": true, "suspended": true, "banned": true}
		if !validStatuses[*req.Status] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid status, must be 'active', 'suspended', or 'banned'",
			})
		}
		updates["status"] = *req.Status
	}

	if len(updates) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "no valid fields to update",
		})
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
