package httpapi

import "context"

type contextKey string

const (
	actorContextKey   contextKey = "actor"
	requestContextKey contextKey = "request-id"
)

func actorFromContext(ctx context.Context) Actor {
	actor, _ := ctx.Value(actorContextKey).(Actor)
	return actor
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestContextKey).(string)
	return requestID
}
