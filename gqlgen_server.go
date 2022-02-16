package october

import (
	"context"
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/handler"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"net/http"
	"sync"
	"time"
)

type GQLGenServer struct {

	mode Mode

	address string
	port    int

	server *http.Server
	serverLock *sync.Mutex

	healthChecks               HealthChecks
	schema graphql.ExecutableSchema
	options []handler.Option
	ginMiddleware []gin.HandlerFunc
}

func (g *GQLGenServer) playgroundHandler() gin.HandlerFunc {
	h := handler.Playground("GraphQL", "/query")

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func (g *GQLGenServer) graphqlHandler() gin.HandlerFunc {
	h := handler.GraphQL(g.schema, g.options...)

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func (g *GQLGenServer) Name() string {
	return "gql-gen"
}

func (g *GQLGenServer) WithExecutableSchema(schema graphql.ExecutableSchema) {
	g.schema = schema
}

func (g *GQLGenServer) WithOptions(options ...handler.Option) {
	g.options = options
}

func (g *GQLGenServer) WithGinMiddleware(middleware ...gin.HandlerFunc) {
	g.ginMiddleware = middleware
}


func (g *GQLGenServer) Start() (bool, error) {
	if g.schema == nil {
		zap.L().Named("OCTOBER").Fatal("Missing gqlgen executable schema, call WithExecutableSchema before Start ")
	}

	if g.mode == LOCAL {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	g.serverLock.Lock()

	if g.server != nil {
		return false, errors.New("Server already running")
	}

	engine := gin.New()

	middleware := []gin.HandlerFunc{
		Ginzap(zap.L(), time.RFC3339, true),
		RecoveryWithZap(zap.L(), true),
	}

	middleware = append(middleware, g.ginMiddleware...)

	engine.Use(middleware...)

	if g.mode == LOCAL {
		engine.GET("/", g.playgroundHandler())
		zap.L().Info("Starting with GraphQL playground")
	}

	engine.POST("/query", g.graphqlHandler())

	g.server = &http.Server{
		Addr: fmt.Sprintf("%s:%d", g.address, g.port),
		Handler: engine,
	}

	g.serverLock.Unlock()

	address := fmt.Sprintf("%s:%d", g.address, g.port)

	zap.S().Named("OCTOBER").Infof("Starting GraphQL server (%s)...", address)

	err := g.server.ListenAndServe()

	return err == http.ErrServerClosed, err
}

func (g *GQLGenServer) Shutdown(ctx context.Context) error {

	g.serverLock.Lock()

	if g.server == nil {
		return nil
	}

	err := g.server.Shutdown(ctx)

	g.serverLock.Unlock()

	return err

}