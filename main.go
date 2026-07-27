package main

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Cortalo/digitalgarden-backend/internal/markdown"
)

// helloWorldMarkdown is temporary, hardcoded content standing in for a
// real stored note — there's no database yet. Once notes are actually
// stored, this endpoint's handler should read from that instead.
const helloWorldMarkdown = `# Hello World

This is a paragraph.

- First item
- Second item
`

func main() {
	r := gin.Default()

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "digitalgarden-backend"})
	})

	// Proves the Step 1 parser works behind a real HTTP boundary, not
	// just inside a Go test. Still hardcoded content — no DB, no auth.
	r.GET("/api/notes/hello-world", func(c *gin.Context) {
		tree := markdown.Parse([]byte(helloWorldMarkdown))
		c.JSON(http.StatusOK, tree)
	})

	r.Run(":8080")
}
