package api

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"annet-oil/internal/annet"
	"annet-oil/internal/api/handlers"
	apimiddleware "annet-oil/internal/api/middleware"
	"annet-oil/internal/audit"
	"annet-oil/internal/auth"
	"annet-oil/internal/cache"
	"annet-oil/internal/checkeast"
	"annet-oil/internal/config"
	"annet-oil/internal/gnetcli"
	"annet-oil/internal/router"
)

type Server struct {
	config         *config.Config
	annetService   *annet.Service
	router         *router.Router
	gnetcliClient  *gnetcli.Client
	checkeastStore *checkeast.Store
	recorder       audit.Recorder
	cache          *cache.MemoryCache
	rbac           *auth.RBAC
}

func NewServer(cfg *config.Config, annetSvc *annet.Service, router *router.Router, gnetcliClient *gnetcli.Client, checkeastStore *checkeast.Store, recorder audit.Recorder) (*Server, error) {
	return &Server{
		config:         cfg,
		annetService:   annetSvc,
		router:         router,
		gnetcliClient:  gnetcliClient,
		checkeastStore: checkeastStore,
		recorder:       recorder,
		cache:          cache.New(cfg.Cache),
		rbac:           auth.NewRBAC(cfg.Auth, cfg.Server.API.AuthToken),
	}, nil
}

func (s *Server) Router() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(apimiddleware.LoggingMiddleware)
	r.Use(middleware.Recoverer)
	reqTimeout := s.config.Server.API.RequestTimeoutSec
	if reqTimeout <= 0 {
		reqTimeout = config.DefaultRequestTimeoutSec
	}
	r.Use(middleware.Timeout(time.Duration(reqTimeout) * time.Second))

	r.Route("/api/v0", func(r chi.Router) {
		r.Use(apimiddleware.AuthMiddleware(s.rbac))
		r.Use(apimiddleware.AuditContextMiddleware)
		r.Use(apimiddleware.RBACMiddleware(s.rbac))
		r.Use(apimiddleware.CacheMiddleware(s.cache))
		r.Use(apimiddleware.InvalidateCacheOnDeploy(s.cache))

		r.Mount("/gen", handlers.NewGenHandler(s.annetService))
		r.Mount("/diff", handlers.NewDiffHandler(s.annetService))
		r.Mount("/checkeast", handlers.NewCheckeastHandler(s.annetService, s.checkeastStore))
		r.Mount("/patch", handlers.NewPatchHandler(s.annetService))
		r.Mount("/deploy", handlers.NewDeployHandler(s.annetService))
		r.Mount("/containers", handlers.NewContainersHandler(s.annetService))
		r.Mount("/routing", handlers.NewRoutingHandler(s.router))
		r.Mount("/execute", handlers.NewExecuteHandler(s.gnetcliClient))
		r.Mount("/diag", handlers.NewDiagHandler(s.gnetcliClient, handlers.DiagConfig{}))
		r.Mount("/inventory", handlers.NewInventoryHandler(s.config.Storage.InventoryFile))
		r.Mount("/check", handlers.NewCheckHandler())
		r.Mount("/featureset", handlers.NewFeatureSetHandler())
		r.Mount("/state", handlers.NewStateHandler(s.gnetcliClient, stateCacheTTL(s.config)))
		r.Mount("/topology", handlers.NewTopologyHandler(s.gnetcliClient, "data/topology.json"))
		r.Mount("/rfc", handlers.NewRFCHandler(s.config.Integrations, s.recorder))
		r.Mount("/audit", handlers.NewAuditHandler(s.recorder))
		r.Get("/health", handlers.HealthHandler)
		r.Get("/health/extended", handlers.ExtendedHealthHandler)
	})

	return r
}

// stateCacheTTL derives the operational-state cache TTL from the cache config,
// defaulting to 60s when unset or unparseable.
func stateCacheTTL(cfg *config.Config) time.Duration {
	if cfg != nil && cfg.Cache.TTL != "" {
		if d, err := time.ParseDuration(cfg.Cache.TTL); err == nil {
			return d
		}
	}
	return 60 * time.Second
}
