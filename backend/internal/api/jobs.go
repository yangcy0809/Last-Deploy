package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"last-deploy/internal/store"
)

func (s *Server) getJob(c *gin.Context) {
	id := c.Param("id")
	job, err := s.st.GetJob(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job})
}
