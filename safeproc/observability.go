package safeproc

import (
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type statusRecorder struct {
	w      http.ResponseWriter
	status int
	n      int
}

func (r *statusRecorder) Header() http.Header {
	return r.w.Header()
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.w.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.w.Write(b)
	r.n += n
	return n, err
}

func Tracing(serviceName string) Middleware {
	if serviceName == "" {
		serviceName = "safe-ms-go"
	}
	tracer := otel.Tracer(serviceName)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.path", r.URL.Path),
				),
			)
			defer span.End()

			rec := &statusRecorder{w: w}
			next.ServeHTTP(rec, r.WithContext(ctx))
			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			span.SetAttributes(attribute.Int("http.status_code", status))
			if status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(status))
			}
		})
	}
}

func AccessLog(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{w: w}
			next.ServeHTTP(rec, r)

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			spanCtx := trace.SpanContextFromContext(r.Context())
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"bytes", rec.n,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFromContext(r.Context()),
				"client_ip", clientIPKey(r),
			}
			if spanCtx.IsValid() {
				attrs = append(attrs, "trace_id", spanCtx.TraceID().String(), "span_id", spanCtx.SpanID().String())
			}
			logger.Info("http_request", attrs...)
		})
	}
}
