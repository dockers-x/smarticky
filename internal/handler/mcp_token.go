package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/mcptoken"
	"smarticky/ent/user"
	mcpserver "smarticky/internal/mcp"

	"github.com/labstack/echo/v4"
)

type MCPTokenResponse struct {
	ID         int        `json:"id"`
	Name       string     `json:"name"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type CreateMCPTokenResponse struct {
	MCPTokenResponse
	Token string `json:"token"`
}

func (h *Handler) ListMCPTokens(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Get("user_id").(int)

	rows, err := h.client.MCPToken.Query().
		Where(mcptoken.HasUserWith(user.IDEQ(userID))).
		Order(ent.Desc(mcptoken.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch MCP tokens"})
	}

	response := make([]MCPTokenResponse, 0, len(rows))
	for _, row := range rows {
		response = append(response, mcpTokenResponse(row))
	}

	return c.JSON(http.StatusOK, response)
}

func (h *Handler) CreateMCPToken(c echo.Context) error {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "MCP Token"
	}
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}

	plaintext, err := mcpserver.GenerateToken()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
	}

	userID := c.Get("user_id").(int)
	row, err := h.client.MCPToken.Create().
		SetName(name).
		SetTokenHash(mcpserver.HashToken(plaintext)).
		SetUserID(userID).
		Save(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create MCP token"})
	}

	return c.JSON(http.StatusCreated, CreateMCPTokenResponse{
		MCPTokenResponse: mcpTokenResponse(row),
		Token:            plaintext,
	})
}

func (h *Handler) DeleteMCPToken(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid token ID"})
	}

	userID := c.Get("user_id").(int)
	count, err := h.client.MCPToken.Delete().
		Where(mcptoken.IDEQ(id), mcptoken.HasUserWith(user.IDEQ(userID))).
		Exec(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete MCP token"})
	}
	if count == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "MCP token not found"})
	}

	return c.NoContent(http.StatusNoContent)
}

func mcpTokenResponse(row *ent.MCPToken) MCPTokenResponse {
	return MCPTokenResponse{
		ID:         row.ID,
		Name:       row.Name,
		LastUsedAt: row.LastUsedAt,
		CreatedAt:  row.CreatedAt,
	}
}
