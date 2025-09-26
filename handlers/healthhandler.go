package handlers

import "github.com/gofiber/fiber/v2"

// HandleHealth is the health check response
func HandleHealth(c *fiber.Ctx) error {
	return c.Status(200).SendString("All is well")
}
