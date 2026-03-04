package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	nlog "github.com/vonmutinda/neo/pkg/logger"

	"github.com/vonmutinda/neo/pkg/httputil"
)

// Recovery is middleware that catches panics in downstream handlers, logs
// them with a full stack trace, and returns a clean 500 to the client.
//
// This replaces chi's default Recoverer with structured slog logging.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				log := nlog.FromContext(r.Context())
				log.Error("panic recovered",
					slog.String("error", fmt.Sprintf("%v", rvr)),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("request_id", RequestIDFromContext(r.Context())),
				)
				log.Debug("panic stack trace", slog.String("stack", string(debug.Stack())))

				httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}
