package handlers

import "github.com/labstack/echo"

// HandleHealth is the health check response
func HandleHealth(c echo.Context) error {
	return c.String(200, "All is well")
}
