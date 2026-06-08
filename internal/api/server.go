package api

import (
	"context"
	"net/http"
	"time"

	"astro-scheduler/internal/scheduler"
	"astro-scheduler/internal/node"
	"astro-scheduler/internal/archiver"
	"astro-scheduler/internal/notifier"
	"astro-scheduler/pkg/utils"

	"github.com/gin-gonic/gin"
)

type Server struct {
	router       *gin.Engine
	httpServer   *http.Server
	taskHandler  *TaskHandler
	nodeHandler  *NodeHandler
	dataHandler  *DataHandler
	alertHandler *AlertHandler
	port         string
}

func NewServer(
	scheduler *scheduler.Scheduler,
	nodeManager *node.NodeManager,
	archiver *archiver.Archiver,
	notifier *notifier.Notifier,
	port string,
) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(requestLogger())

	server := &Server{
		router:       router,
		taskHandler:  NewTaskHandler(scheduler),
		nodeHandler:  NewNodeHandler(nodeManager),
		dataHandler:  NewDataHandler(archiver),
		alertHandler: NewAlertHandler(notifier),
		port:         port,
	}

	server.setupRoutes()
	return server
}

func (s *Server) setupRoutes() {
	v1 := s.router.Group("/api/v1")

	v1.GET("/health", s.healthCheck)
	v1.GET("/stats", s.getAllStats)

	tasks := v1.Group("/tasks")
	{
		tasks.POST("", s.taskHandler.CreateTask)
		tasks.GET("", s.taskHandler.ListTasks)
		tasks.GET("/stats", s.taskHandler.GetStats)
		tasks.GET("/:id", s.taskHandler.GetTask)
		tasks.PUT("/:id/status", s.taskHandler.UpdateTaskStatus)
		tasks.GET("/:id/events", s.taskHandler.GetTaskEvents)
	}

	nodes := v1.Group("/nodes")
	{
		nodes.POST("", s.nodeHandler.RegisterNode)
		nodes.DELETE("/:id", s.nodeHandler.DeregisterNode)
		nodes.GET("", s.nodeHandler.ListNodes)
		nodes.GET("/stats", s.nodeHandler.GetStats)
		nodes.GET("/:id", s.nodeHandler.GetNode)
		nodes.POST("/heartbeat", s.nodeHandler.Heartbeat)
	}

	data := v1.Group("/data")
	{
		data.POST("", s.dataHandler.AddData)
		data.GET("", s.dataHandler.ListData)
		data.GET("/stats", s.dataHandler.GetStats)
		data.GET("/:id", s.dataHandler.GetData)
		data.GET("/:id/download", s.dataHandler.DownloadData)
		data.GET("/:id/presigned-url", s.dataHandler.GetPresignedURL)
		data.GET("/task/:task_id", s.dataHandler.GetDataByTask)

		policies := data.Group("/policies")
		{
			policies.POST("", s.dataHandler.CreatePolicy)
			policies.GET("", s.dataHandler.ListPolicies)
			policies.DELETE("/:id", s.dataHandler.DeletePolicy)
		}
	}

	alerts := v1.Group("/alerts")
	{
		alerts.POST("", s.alertHandler.CreateAlert)
		alerts.GET("", s.alertHandler.ListAlerts)
		alerts.GET("/stats", s.alertHandler.GetStats)
		alerts.GET("/:id", s.alertHandler.GetAlert)
		alerts.PUT("/:id/resolve", s.alertHandler.ResolveAlert)
	}
}

func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:    ":" + s.port,
		Handler: s.router,
	}

	utils.Sugar.Infof("API server starting on port %s", s.port)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Sugar.Errorf("API server error: %v", err)
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	utils.Sugar.Info("API server shutting down...")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "astro-scheduler",
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (s *Server) getAllStats(c *gin.Context) {
	stats := map[string]interface{}{
		"tasks":   nil,
		"nodes":   nil,
		"data":    nil,
		"alerts":  nil,
	}

	if s.taskHandler != nil {
		stats["tasks"] = s.taskHandler.scheduler.GetStats()
	}
	if s.nodeHandler != nil {
		stats["nodes"] = s.nodeHandler.manager.GetStats()
	}
	if s.dataHandler != nil {
		stats["data"] = s.dataHandler.archiver.GetStats()
	}
	if s.alertHandler != nil {
		stats["alerts"] = s.alertHandler.notifier.GetStats()
	}

	c.JSON(http.StatusOK, stats)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		utils.Sugar.Debugf("%s %s - %d - %v", method, path, statusCode, latency)
	}
}
