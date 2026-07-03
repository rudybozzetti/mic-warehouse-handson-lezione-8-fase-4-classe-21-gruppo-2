package handlers

import (
	"github.com/labstack/echo/v4"
)

// Router registers the Article routes on the Echo instance.
type Router struct {
	articleHandler *ArticleHandler
}

func NewRouter(h *ArticleHandler) *Router {
	return &Router{articleHandler: h}
}

func (r *Router) Register(e *echo.Echo) {
	// GIVEN — the worked-example route.
	e.POST("/articles", r.articleHandler.CreateArticle)

	e.GET("/articles/:id", r.articleHandler.GetArticle)

	e.PUT("/articles/:id/price", r.articleHandler.ChangeArticlePrice)
}
