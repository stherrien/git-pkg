package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/stherrien/git-pkg/internal/github"
)

func (h *Handler) handleOwnerIndex(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	if !validName.MatchString(owner) {
		http.Error(w, "invalid owner name", http.StatusBadRequest)
		return
	}

	authToken := extractAuthToken(r)
	cacheKey := "owner:" + owner

	// Check cache
	if cached, ok := h.cache.Get(cacheKey); ok {
		repos := cached.([]github.Repo)
		h.renderOwnerIndex(w, r, owner, repos)
		return
	}

	repos, err := h.gh.FetchOwnerPackages(owner, authToken)
	if err != nil {
		h.logger.Error("failed to fetch owner packages", "owner", owner, "error", err)
		http.Error(w, "failed to fetch packages", http.StatusBadGateway)
		return
	}

	h.cache.Set(cacheKey, repos)
	h.renderOwnerIndex(w, r, owner, repos)
}

func (h *Handler) renderOwnerIndex(w http.ResponseWriter, r *http.Request, owner string, repos []github.Repo) {
	if wantsPEP503(r) {
		h.renderOwnerPEP503(w, owner, repos)
		return
	}
	h.renderOwnerHTML(w, owner, repos)
}

func (h *Handler) renderOwnerPEP503(w http.ResponseWriter, owner string, repos []github.Repo) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, "<!DOCTYPE html>\n<html><body>\n")
	for _, repo := range repos {
		normalized := normalizeName(repo.Name)
		fmt.Fprintf(w, "<a href=\"/simple/%s/%s/\">%s</a>\n", owner, normalized, normalized)
	}
	fmt.Fprint(w, "</body></html>")
}

func (h *Handler) renderOwnerHTML(w http.ResponseWriter, owner string, repos []github.Repo) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := struct {
		Owner string
		Repos []github.Repo
	}{Owner: owner, Repos: repos}

	if err := templates.ExecuteTemplate(w, "owner.html", data); err != nil {
		h.logger.Error("template render failed", "template", "owner", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) handlePackageIndex(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	pkg := r.PathValue("package")

	if !validName.MatchString(owner) || !validName.MatchString(pkg) {
		http.Error(w, "invalid owner or package name", http.StatusBadRequest)
		return
	}

	authToken := extractAuthToken(r)
	cacheKey := owner + "/" + pkg + ":releases"

	// Check cache
	if cached, ok := h.cache.Get(cacheKey); ok {
		releases := cached.([]github.Release)
		h.renderPackageIndex(w, r, owner, pkg, releases)
		return
	}

	releases, err := h.gh.FetchReleases(owner, pkg, authToken)
	if err != nil {
		h.logger.Error("failed to fetch releases", "owner", owner, "package", pkg, "error", err)
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "package not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to fetch releases", http.StatusBadGateway)
		}
		return
	}

	h.cache.Set(cacheKey, releases)
	h.renderPackageIndex(w, r, owner, pkg, releases)
}

func (h *Handler) renderPackageIndex(w http.ResponseWriter, r *http.Request, owner, pkg string, releases []github.Release) {
	if wantsPEP503(r) {
		h.renderPackagePEP503(w, releases)
		return
	}
	h.renderPackageHTML(w, owner, pkg, releases)
}

func (h *Handler) renderPackagePEP503(w http.ResponseWriter, releases []github.Release) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, "<!DOCTYPE html>\n<html><body>\n")
	for _, rel := range releases {
		for _, asset := range rel.Assets {
			if github.IsDistFile(asset.Name) {
				fmt.Fprintf(w, "<a href=\"%s\">%s</a>\n", asset.BrowserDownloadURL, asset.Name)
			}
		}
	}
	fmt.Fprint(w, "</body></html>")
}

func (h *Handler) renderPackageHTML(w http.ResponseWriter, owner, pkg string, releases []github.Release) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	type assetInfo struct {
		Name string
		URL  string
		Size int64
	}
	type releaseInfo struct {
		Tag    string
		Assets []assetInfo
	}

	var rels []releaseInfo
	for _, rel := range releases {
		var assets []assetInfo
		for _, a := range rel.Assets {
			if github.IsDistFile(a.Name) {
				assets = append(assets, assetInfo{Name: a.Name, URL: a.BrowserDownloadURL, Size: a.Size})
			}
		}
		if len(assets) > 0 {
			rels = append(rels, releaseInfo{Tag: rel.TagName, Assets: assets})
		}
	}

	data := struct {
		Owner    string
		Package  string
		Releases []releaseInfo
	}{Owner: owner, Package: pkg, Releases: rels}

	if err := templates.ExecuteTemplate(w, "package.html", data); err != nil {
		h.logger.Error("template render failed", "template", "package", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// wantsPEP503 returns true if the request looks like it comes from pip.
func wantsPEP503(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	ua := r.Header.Get("User-Agent")
	return strings.Contains(accept, "application/vnd.pypi.simple") ||
		strings.Contains(ua, "pip/") ||
		strings.Contains(ua, "setuptools") ||
		strings.Contains(ua, "pep691")
}

// normalizeName converts a repo name to PEP 503 normalized form.
func normalizeName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", "-"))
}
