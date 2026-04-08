package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/stherrien/git-pkg/internal/cache"
	"github.com/stherrien/git-pkg/internal/github"
)

var (
	// Valid GitHub username/org and repo name pattern
	validName = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`)
)

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	gh     *github.Client
	cache  *cache.Cache
	logger *slog.Logger
}

// New creates a Handler with the given dependencies.
func New(gh *github.Client, c *cache.Cache, logger *slog.Logger) *Handler {
	return &Handler{gh: gh, cache: c, logger: logger}
}

// RegisterRoutes sets up the HTTP routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.handleHealthz)
	mux.HandleFunc("GET /{$}", h.handleLanding)
	mux.HandleFunc("GET /{owner}/{$}", h.handleOwnerIndex)
	mux.HandleFunc("GET /{owner}/{package}/{$}", h.handlePackageIndex)
	mux.HandleFunc("POST /{owner}/{package}/-/refresh", h.handleRefresh)
}

// extractAuthToken pulls a GitHub token from the request.
// Supports both Basic Auth (pip's standard: username is ignored, password is the token)
// and Bearer tokens.
func extractAuthToken(r *http.Request) string {
	// Basic Auth — pip sends this when credentials are in the URL
	// e.g. https://x:ghp_abc123@git-pkg.dev/owner/
	if _, password, ok := r.BasicAuth(); ok && password != "" {
		return password
	}
	// Bearer token fallback
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func (h *Handler) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "ok")
}

func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	pkg := r.PathValue("package")

	if !validName.MatchString(owner) || !validName.MatchString(pkg) {
		http.Error(w, "invalid owner or package name", http.StatusBadRequest)
		return
	}

	h.cache.DeletePrefix(owner + "/" + pkg)
	h.logger.Info("cache invalidated", "owner", owner, "package", pkg)
	w.WriteHeader(http.StatusNoContent)
}
