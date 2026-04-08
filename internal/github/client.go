package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Asset represents a file attached to a GitHub Release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Release represents a GitHub Release.
type Release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	Prerelease bool    `json:"prerelease"`
	Draft      bool    `json:"draft"`
	Assets     []Asset `json:"assets"`
}

// Repo represents a GitHub repository (minimal fields).
type Repo struct {
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
	HTMLURL     string `json:"html_url"`
}

// Client talks to the GitHub API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string // optional server-side token for rate limits
}

// NewClient creates a GitHub API client.
func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    "https://api.github.com",
		token:      token,
	}
}

// FetchReleases returns all non-draft releases for a repo.
func (c *Client) FetchReleases(owner, repo, authToken string) ([]Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100", c.baseURL, owner, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("repository %s/%s not found", owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases: %w", err)
	}

	// Filter out drafts
	filtered := releases[:0]
	for _, r := range releases {
		if !r.Draft {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

// FetchOwnerPackages returns repos for an owner that have releases with wheel assets.
func (c *Client) FetchOwnerPackages(owner, authToken string) ([]Repo, error) {
	// First fetch repos, then check which have releases with wheels
	repos, err := c.fetchRepos(owner, authToken)
	if err != nil {
		return nil, err
	}

	var packages []Repo
	for _, repo := range repos {
		releases, err := c.FetchReleases(owner, repo.Name, authToken)
		if err != nil {
			continue // skip repos we can't read
		}
		if hasWheelAssets(releases) {
			packages = append(packages, repo)
		}
	}
	return packages, nil
}

func (c *Client) fetchRepos(owner, authToken string) ([]Repo, error) {
	// Try as org first, fall back to user
	repos, err := c.fetchReposList(fmt.Sprintf("%s/orgs/%s/repos?per_page=100&type=public", c.baseURL, owner), authToken)
	if err != nil {
		repos, err = c.fetchReposList(fmt.Sprintf("%s/users/%s/repos?per_page=100&type=public", c.baseURL, owner), authToken)
		if err != nil {
			return nil, err
		}
	}
	return repos, nil
}

func (c *Client) fetchReposList(url, authToken string) ([]Repo, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var repos []Repo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, err
	}
	return repos, nil
}

func (c *Client) setHeaders(req *http.Request, authToken string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	// Prefer user-provided token (for private repos), fall back to server token
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	} else if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

func hasWheelAssets(releases []Release) bool {
	for _, r := range releases {
		for _, a := range r.Assets {
			if IsWheelFile(a.Name) {
				return true
			}
		}
	}
	return false
}

// IsWheelFile returns true if the filename is a Python wheel.
func IsWheelFile(name string) bool {
	return strings.HasSuffix(name, ".whl")
}

// IsDistFile returns true if the filename is a wheel or sdist.
func IsDistFile(name string) bool {
	return strings.HasSuffix(name, ".whl") || strings.HasSuffix(name, ".tar.gz")
}
