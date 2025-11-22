package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/firstkb/sc-api/internal/httpx/apperr"
)

type TargetFunc[In any, Out any] func(context.Context, *http.Request, In) (Out, error)

type Response struct {
	Status  string `json:"status"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

func HandleJson[In any, Out any](f TargetFunc[In, Out], logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic recovered", "error", err)
				encode(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		var in In
		if r.Body != nil && r.Body != http.NoBody {
			var err error
			in, err = decode[In](r)
			if err != nil {
				encode(w, http.StatusBadRequest, "invalid json")
				return
			}
		}

		// Call out to target function
		out, err := f(r.Context(), r, in)
		if err != nil {
			appErr := apperr.ToHTTP(err)
			if encodeErr := encode(w, appErr.StatusCode, appErr); encodeErr != nil {
				logger.Error("failed to encode error response", "error", encodeErr)
			}
			return
		}

		// Encode response body
		if err := encode(w, http.StatusOK, out); err != nil {
			logger.Error("failed to encode response", "error", err)
		}
	})
}

func encode[Out any](w http.ResponseWriter, status int, out Out) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := Response{
		Status: "ok",
		Data:   out,
	}
	if status != http.StatusOK {
		resp = Response{Status: "error"}
		if ae, ok := any(out).(*apperr.AppError); ok && ae != nil {
			resp.Code = ae.Code
			resp.Message = ae.Message
		} else {
			resp.Message = fmt.Sprintf("%v", out)
		}
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

func decode[In any](r *http.Request) (In, error) {
	var in In
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		return in, fmt.Errorf("decode json: %w", err)
	}
	return in, nil
}
