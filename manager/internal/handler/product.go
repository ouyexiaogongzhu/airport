package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ouyexiaogongzhu/airport/manager/internal/db"
	"github.com/ouyexiaogongzhu/airport/manager/internal/model"
)

// ListProducts returns all products ordered by id with pagination
// TODO: Propagate request context via c.Context() for GORM queries.
func ListProducts(c *fiber.Ctx) error {
	offset, limit := parsePagination(c)

	var total int64
	db.DB.Model(&model.Product{}).Count(&total)

	var products []model.Product
	if result := db.DB.Order("id ASC").Offset(offset).Limit(limit).Find(&products); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list products"})
	}
	return c.JSON(fiber.Map{
		"products": products,
		"total":    total,
		"page":     offset/limit + 1,
		"per_page": limit,
	})
}

// ListActiveProducts returns only active products (public) with pagination
func ListActiveProducts(c *fiber.Ctx) error {
	offset, limit := parsePagination(c)

	var total int64
	db.DB.Model(&model.Product{}).Where("status = ?", "active").Count(&total)

	var products []model.Product
	if result := db.DB.Where("status = ?", "active").Order("id ASC").Offset(offset).Limit(limit).Find(&products); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list products"})
	}
	return c.JSON(fiber.Map{
		"products": products,
		"total":    total,
		"page":     offset/limit + 1,
		"per_page": limit,
	})
}

// CreateProduct creates a new product
func CreateProduct(c *fiber.Ctx) error {
	req := new(model.Product)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Name == "" || req.Price <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name and price are required"})
	}

	// Validate product type
	validTypes := map[string]bool{"subscription": true, "one-time": true, "trial": true}
	if req.Type != "" && !validTypes[req.Type] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "type must be one of: subscription, one-time, trial",
		})
	}

	// Validate product status
	validStatuses := map[string]bool{"active": true, "inactive": true, "archived": true}
	if req.Status != "" && !validStatuses[req.Status] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "status must be one of: active, inactive, archived",
		})
	}

	if result := db.DB.Create(req); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create product"})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"product": req})
}

// UpdateProduct updates an existing product
type UpdateProductRequest struct {
	Name   *string  `json:"name"`
	Type   *string  `json:"type"`
	Price  *float64 `json:"price"`
	Stock  *int     `json:"stock"`
	Status *string  `json:"status"`
}

func UpdateProduct(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid product id"})
	}

	var product model.Product
	if result := db.DB.First(&product, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "product not found"})
	}

	req := new(UpdateProductRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Price != nil {
		updates["price"] = *req.Price
	}
	if req.Stock != nil {
		updates["stock"] = *req.Stock
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if result := db.DB.Model(&product).Updates(updates); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update product"})
	}

	db.DB.First(&product, id)
	return c.JSON(fiber.Map{"product": product})
}

// DeleteProduct archives a product (sets status to archived)
func DeleteProduct(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid product id"})
	}

	var product model.Product
	if result := db.DB.First(&product, id); result.Error != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "product not found"})
	}

	if result := db.DB.Model(&product).Update("status", "archived"); result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to archive product"})
	}

	return c.JSON(fiber.Map{"message": "product archived"})
}
