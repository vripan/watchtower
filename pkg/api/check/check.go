package check

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Result describes the update-availability outcome for a single watched container.
type Result struct {
	// Name is the container name.
	Name string `json:"name"`
	// Image is the image reference with tag (e.g. nginx:latest).
	Image string `json:"image"`
	// CurrentDigest is the registry manifest digest the running image was pulled from
	// (sha256:...), derived from the image's RepoDigests. Empty for locally-built images
	// with no registry reference.
	CurrentDigest string `json:"current_digest"`
	// LatestDigest is the manifest digest currently advertised by the registry
	// (sha256:...). Empty when no registry lookup was performed (locally-built images)
	// or when the lookup failed (see Error).
	LatestDigest string `json:"latest_digest"`
	// UpdateAvailable is true when the registry advertises a digest that differs from
	// the running image's digest.
	UpdateAvailable bool `json:"update_available"`
	// Error holds a human-readable message when the registry check for this container
	// failed. It is omitted when the check succeeded.
	Error string `json:"error,omitempty"`
}

// CheckFunc returns the update-availability results for all watched containers.
type CheckFunc func(ctx context.Context) ([]Result, error)

// Handler serves the /v1/check endpoint.
//
// It holds the check function and endpoint path for the read-only /v1/check endpoint.
type Handler struct {
	check CheckFunc // Update-availability lookup function.
	Path  string    // API endpoint path (e.g., "/v1/check").
}

// New creates a new check Handler backed by the given check function.
//
// Parameters:
//   - check: Function returning the update-availability results for all watched containers.
//
// Returns:
//   - *Handler: Initialized handler serving /v1/check.
func New(check CheckFunc) *Handler {
	return &Handler{
		check: check,
		Path:  "/v1/check",
	}
}

// Handle responds with the JSON update-availability status of every watched container.
//
// It performs no pull and no recreate; it only compares each running image's digest
// against the registry.
//
// Parameters:
//   - w: HTTP response writer for sending the JSON payload or error status.
//   - r: HTTP request; its context is propagated to the registry calls.
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	logrus.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}).Debug("Received HTTP API check request")

	results, err := h.check(r.Context())
	if err != nil {
		logrus.WithError(err).Error("Failed to check containers for updates")
		http.Error(w, "failed to check for updates", http.StatusInternalServerError)

		return
	}

	response := map[string]any{
		"checked":     time.Now().UTC().Format(time.RFC3339),
		"containers":  results,
		"count":       len(results),
		"api_version": "v1",
	}

	var buf bytes.Buffer

	err = json.NewEncoder(&buf).Encode(response)
	if err != nil {
		logrus.WithError(err).Error("Failed to encode check response")
		http.Error(w, "failed to encode response", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(buf.Bytes())
	if err != nil {
		logrus.WithError(err).Error("Failed to write check response")
	}
}
