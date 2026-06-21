package handler

import (
	"net/http"
	"os"
	"strconv"

	"github.com/labstack/echo/v4"
)

func (h *Handler) DownloadMCPImage(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid image ID"})
	}

	userID := c.Get("user_id").(int)
	imageFile, err := h.shareImages.GetOwnedImage(c.Request().Context(), userID, id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "MCP image not found"})
	}

	data, err := os.ReadFile(imageFile.Path)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "MCP image file not found"})
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+imageFile.Filename+`"`)
	c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(int64(len(data)), 10))
	return c.Blob(http.StatusOK, imageFile.ContentType, data)
}
