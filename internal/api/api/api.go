package api

import (
	"fifthOne/cmd/middleware"
	"fifthOne/internal/service"
	"github.com/gin-contrib/cors"
	"github.com/wb-go/wbf/ginext"
)

type Routers struct {
	Service service.Service
}

func NewRouters(r *Routers) *ginext.Engine {
	app := ginext.New("release")

	app.Use(middleware.LoggingMiddleware())
	app.Use(cors.Default())
	apiGroup := app.Group("/v1")

	apiGroup.POST("/events", r.Service.CreateEvent)
	apiGroup.POST("/events/:id/book", r.Service.Book)
	apiGroup.POST("/events/:id/confirm", r.Service.Confirm)
	apiGroup.GET("/events/:id", r.Service.GetInfo)
	apiGroup.GET("/events", r.Service.GetAllEvents)

	app.GET("/", func(c *ginext.Context) {
		c.File("./frontend/index.html")
	})
	app.GET("/user", func(c *ginext.Context) {
		c.File("./frontend/user.html")
	})
	app.GET("/adm", func(c *ginext.Context) {
		c.File("./frontend/adm.html")
	})
	app.Static("/frontend", "./frontend")

	return app
}
