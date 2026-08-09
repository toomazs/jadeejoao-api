package server

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jadeejoao/jadeejoao-api/internal/content"
	"github.com/jadeejoao/jadeejoao-api/internal/gifts"
	"github.com/jadeejoao/jadeejoao-api/internal/guests"
	"github.com/jadeejoao/jadeejoao-api/internal/messages"
)

// Register mounts every operation of every module. It must stay deterministic:
// cmd/openapi calls it with zero Deps to export the spec.
func Register(api huma.API, deps Deps) {
	registerHealth(api, deps)
	content.RegisterPublic(api, deps.Content)
	guests.RegisterPublic(api, deps.Guests)
	gifts.RegisterPublic(api, deps.Gifts)
	messages.RegisterPublic(api, deps.Messages)
}

// HealthOutput is the healthcheck response body.
type HealthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok" doc:"Overall service status."`
	}
}

func registerHealth(api huma.API, deps Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "health-check",
		Method:      http.MethodGet,
		Path:        "/healthz",
		Summary:     "Health check",
		Description: "Railway healthcheck endpoint: returns 200 when the service and its database connection are healthy.",
		Tags:        []string{"system"},
	}, func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		if deps.Pool != nil {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err := deps.Pool.Ping(pingCtx); err != nil {
				return nil, huma.Error503ServiceUnavailable("database unreachable", err)
			}
		}
		out := &HealthOutput{}
		out.Body.Status = "ok"
		return out, nil
	})
}
