package api

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"annet-oil/internal/annet"
	"annet-oil/internal/api/handlers"
	apimiddleware "annet-oil/internal/api/middleware"
	"annet-oil/internal/auth"
	"annet-oil/internal/cache"
	"annet-oil/internal/config"
	"annet-oil/internal/gnetcli"
	"annet-oil/internal/router"
)

type Server struct {
	config        *config.Config
	annetService  *annet.Service
	router        *router.Router
	gnetcliClient *gnetcli.Client
	cache         *cache.MemoryCache
	rbac          *auth.RBAC
}

func NewServer(cfg *config.Config, annetSvc *annet.Service, router *router.Router, gnetcliClient *gnetcli.Client) (*Server, error) {
	return &Server{
		config:        cfg,
		annetService:  annetSvc,
		router:        router,
		gnetcliClient: gnetcliClient,
		cache:         cache.New(cfg.Cache),
		rbac:          auth.NewRBAC(cfg.Auth, cfg.Server.API.AuthToken),
	}, nil
}

func (s *Server) Router() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(apimiddleware.LoggingMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Route("/api/v0", func(r chi.Router) {
		r.Use(apimiddleware.AuthMiddleware(s.rbac))
		r.Use(apimiddleware.RBACMiddleware(s.rbac))
		r.Use(apimiddleware.CacheMiddleware(s.cache))
		r.Use(apimiddleware.InvalidateCacheOnDeploy(s.cache))

		r.Mount("/gen", handlers.NewGenHandler(s.annetService))
		r.Mount("/diff", handlers.NewDiffHandler(s.annetService))
		r.Mount("/patch", handlers.NewPatchHandler(s.annetService))
		r.Mount("/deploy", handlers.NewDeployHandler(s.annetService))
		r.Mount("/containers", handlers.NewContainersHandler(s.annetService))
		r.Mount("/routing", handlers.NewRoutingHandler(s.router))
		r.Mount("/execute", handlers.NewExecuteHandler(s.gnetcliClient))
		r.Mount("/inventory", handlers.NewInventoryHandler(s.config.Storage.InventoryFile))
		r.Mount("/check", handlers.NewCheckHandler())
		r.Mount("/featureset", handlers.NewFeatureSetHandler())
		r.Mount("/rfc", handlers.NewRFCHandler(s.config.Integrations))
		r.Get("/health", handlers.HealthHandler)
		r.Get("/health/extended", handlers.ExtendedHealthHandler)
	})

	return r
}
