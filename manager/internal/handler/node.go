package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

type CreateNodeRequest struct {
	Name     string `json:"name" validate:"required"`
	Type     string `json:"type" validate:"required"`     // v2ray / xray
	Address  string `json:"address" validate:"required"`
	Port     int    `json:"port" validate:"required"`
	Protocol string `json:"protocol" validate:"required"` // vmess, shadowsocks, trojan
	UserID   uint   `json:"user_id" validate:"required"`
}

type UpdateNodeRequest struct {
	Name     *string `json:"name"`
	Type     *string `json:"type"`
	Address  *string `json:"address"`
	Port     *int    `json:"port"`
	Protocol *string `json:"protocol"`
	Status   *string `json:"status"`
}

// CreateNode adds a new proxy node.
func CreateNode(c *fiber.Ctx) error {
	req := new(CreateNodeRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Name == "" || req.Type == "" || req.Address == "" || req.Port == 0 || req.Protocol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name, type, address, port and protocol are required",
		})
	}

	node := model.Node{
		Name:     req.Name,
		Type:     req.Type,
		Address:  req.Address,
		Port:     req.Port,
		Protocol: req.Protocol,
		Status:   "inactive",
		UserID:   req.UserID,
	}

	if result := db.DB.Create(&node); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create node",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(node)
}

// GetNode returns a single node by ID.
func GetNode(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid node id",
		})
	}

	var node model.Node
	if result := db.DB.First(&node, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "node not found",
		})
	}

	return c.JSON(node)
}

// ListNode returns all nodes with optional status filter.
func ListNode(c *fiber.Ctx) error {
	var nodes []model.Node
	query := db.DB

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if result := query.Find(&nodes); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list nodes",
		})
	}

	return c.JSON(nodes)
}

// UpdateNode updates a node's fields.
func UpdateNode(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid node id",
		})
	}

	var node model.Node
	if result := db.DB.First(&node, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "node not found",
		})
	}

	req := new(UpdateNodeRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Address != nil {
		updates["address"] = *req.Address
	}
	if req.Port != nil {
		updates["port"] = *req.Port
	}
	if req.Protocol != nil {
		updates["protocol"] = *req.Protocol
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) > 0 {
		if result := db.DB.Model(&node).Updates(updates); result.Error != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to update node",
			})
		}
	}

	db.DB.First(&node, id)
	return c.JSON(node)
}

// DeleteNode removes a node by ID.
func DeleteNode(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid node id",
		})
	}

	if result := db.DB.Delete(&model.Node{}, id); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to delete node",
		})
	}

	return c.JSON(fiber.Map{
		"message": "node deleted",
	})
}
