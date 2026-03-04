package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vonmutinda/neo/pkg/cache"
	"github.com/vonmutinda/neo/pkg/httputil"
)

type HealthHandler struct {
	pool  *pgxpool.Pool
	cache cache.Cache
}

func NewHealthHandler(pool *pgxpool.Pool, c cache.Cache) *HealthHandler {
	return &HealthHandler{pool: pool, cache: c}
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := h.pool.Ping(ctx); err != nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "database not reachable")
		return
	}
	if err := h.cache.Ping(ctx); err != nil {
		httputil.WriteError(w, http.StatusServiceUnavailable, "cache not reachable")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
