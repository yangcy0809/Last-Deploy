package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"last-deploy/internal/config"
	"last-deploy/internal/jobs"
	"last-deploy/internal/store"
)

type Server struct {
	st    *store.Store
	queue *jobs.Queue
	cfg   config.Config
}

func NewRouter(st *store.Store, q *jobs.Queue, cfg config.Config) *gin.Engine {
	s := &Server{st: st, queue: q, cfg: cfg}

	r := gin.New()
	r.Use(gin.Recovery())

	staticDir := os.Getenv("LAST_DEPLOY_STATIC_DIR")
	if staticDir == "" {
		staticDir = "./static"
	}

	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	api.GET("/projects", s.listProjects)
	api.POST("/projects", s.createProject)
	api.POST("/projects/detect", s.detectProject)
	api.POST("/projects/from-draft", s.createProjectFromDraft)
	api.GET("/projects/:id", s.getProject)
	api.PUT("/projects/:id/config", s.updateProjectConfig)
	api.GET("/projects/:id/jobs/latest", s.getProjectLatestJob)
	api.POST("/projects/:id/deploy", s.deployProject)
	api.POST("/projects/:id/start", s.startProject)
	api.POST("/projects/:id/stop", s.stopProject)
	api.POST("/projects/:id/pause", s.pauseProject)
	api.POST("/projects/:id/unpause", s.unpauseProject)
	api.DELETE("/projects/:id", s.deleteProject)

	api.GET("/jobs/:id", s.getJob)

	// 静态文件放最后，使用 NoRoute 避免与 API 路由冲突
	r.NoRoute(gin.WrapH(http.FileServer(http.Dir(staticDir))))

	return r
}
