package main

import (
	"github.com/vonmutinda/neo/internal/services/permissions"
	"github.com/vonmutinda/neo/internal/services/regulatory"
	"github.com/vonmutinda/neo/internal/transport/http/handlers"
	adminh "github.com/vonmutinda/neo/internal/transport/http/handlers/admin"
	bizh "github.com/vonmutinda/neo/internal/transport/http/handlers/business"
	persh "github.com/vonmutinda/neo/internal/transport/http/handlers/personal"
	webhookh "github.com/vonmutinda/neo/internal/transport/http/handlers/webhooks"
)

// HandlerList aggregates all HTTP handler facades. The router references
// these fields to wire endpoints to handler methods.
type HandlerList struct {
	Health   *handlers.HealthHandler
	FXRates  *handlers.FXRateHandler
	Personal *persh.Handlers
	Business *bizh.Handlers
	Admin    *adminh.Handlers
	Webhooks *webhookh.WiseWebhookHandler

	PermissionService *permissions.Service
	RegulatoryService *regulatory.Service
}
