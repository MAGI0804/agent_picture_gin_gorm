package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ContextTimeout sets a request context deadline for normal API calls.
func ContextTimeout(t time.Duration) func(c *gin.Context) {
	return func(c *gin.Context) {
		if shouldSkipContextTimeout(c) {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), t)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func shouldSkipContextTimeout(c *gin.Context) bool {
	path := c.Request.URL.Path

	return c.Request.Method == http.MethodPost &&
		strings.HasPrefix(path, "/api/conversations/") &&
		strings.HasSuffix(path, "/messages")
}
