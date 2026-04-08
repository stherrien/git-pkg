# git-pkg.dev — Product Requirements Document

**Version:** 1.0
**Date:** 2026-04-07
**Author:** Shawn Therrien
**Status:** Draft

---

## 1. Overview

**git-pkg.dev** is a stateless Go proxy that turns GitHub Releases into a PEP 503-compliant Python package index. It requires zero accounts, zero storage, and zero configuration from package authors — if you can tag a GitHub release and attach a `.whl` file, you're published.

### One-Line Pitch

> GitHub Releases are your package registry. git-pkg.dev just translates.

---

## 2. Problem Statement

Publishing a Python package today requires:

1. Creating and managing a PyPI account (or private registry)
2. Configuring API tokens and upload workflows
3. Trusting a third-party registry to host and serve your artifacts
4. Maintaining separate identity systems across GitHub and PyPI

For internal/private packages, teams either stand up a full registry (Artifactory, CodeArtifact, private PyPI) or resort to `pip install git+https://...` which bypasses versioning, caching, and dependency resolution.

**There is no lightweight path from "I tagged a release on GitHub" to "my users can `pip install` it."**

---

## 3. Solution

A single Go binary that:

1. Receives standard pip index requests
2. Translates them into GitHub API calls
3. Returns PEP 503-compliant HTML pointing at GitHub Release asset download URLs
4. Serves a discovery frontend for browsing available packages

### Architecture

```
pip install pkg --index-url https://git-pkg.dev/owner/
        │
        ▼
┌──────────────────┐
│   git-pkg.dev    │  Go service (stateless)
│                  │
│  /owner/pkg/     │──▶ GitHub API: GET /repos/owner/pkg/releases
│                  │◀── JSON: release assets
│  Renders PEP 503 │
│  HTML on the fly  │
└──────────────────┘
        │
        ▼
pip downloads .whl directly from GitHub's CDN
```

**Key property:** git-pkg.dev never touches the artifact bytes. It is a metadata translator only.

---

## 4. Target Users

| Persona | Need |
|---|---|
| **Solo developer** | Publish a Python package without PyPI ceremony |
| **Internal team** | Share private packages within a GitHub Org |
| **Enterprise (e.g., RegScale)** | Identity-locked package distribution tied to GitHub Org |
| **Open-source maintainer** | Offer an alternative install path alongside PyPI |

---

## 5. Functional Requirements

### 5.1 Package Index API (Core)

**FR-1: Simple Index Root**
- `GET /{owner}/` returns a PEP 503 simple index page listing all repos under `{owner}` that have releases with `.whl` assets
- Content-Type: `text/html`

**FR-2: Package Page**
- `GET /{owner}/{package}/` returns a PEP 503 package page listing all `.whl` and `.tar.gz` assets across all releases
- Each `<a>` tag links directly to the GitHub Release asset `browser_download_url`
- Support `data-requires-python` attribute if extractable from wheel filename or release metadata

**FR-3: Hash Verification**
- Include `#sha256=<hash>` fragment on download links when available from GitHub API or computable at proxy time
- This enables pip's built-in integrity verification

**FR-4: Private Repos**
- Support `Authorization: Bearer <github-token>` header pass-through
- If the user provides a GitHub token, use it for API calls to access private repos
- `pip install pkg --index-url https://git-pkg.dev/owner/ --header "Authorization=Bearer ${GH_TOKEN}"`

**FR-5: Version Filtering**
- Map GitHub release tag names to PEP 440 versions
- Strip leading `v` prefix (e.g., `v1.2.3` → `1.2.3`)
- Skip releases tagged as pre-release unless pip requests them

### 5.2 Discovery Frontend

**FR-6: Landing Page**
- `GET /` serves a clean, fast landing page built with Go `html/template`
- Explains what git-pkg.dev is in one sentence
- Shows a search bar for GitHub owner/org lookup

**FR-7: Owner Browse Page**
- `GET /{owner}` (without trailing slash, browser request via Accept header) renders an HTML page showing:
  - Owner's GitHub avatar and name
  - List of repos with releases containing wheel assets
  - Install command for each package

**FR-8: Content Negotiation**
- Distinguish between pip requests (`Accept: application/vnd.pypi.simple.v1+html` or similar) and browser requests (`Accept: text/html`)
- Serve PEP 503 index to pip, human-friendly page to browsers
- Fall back to PEP 503 if ambiguous

### 5.3 Caching

**FR-9: Response Caching**
- Cache GitHub API responses in-memory with configurable TTL (default: 5 minutes)
- Respect GitHub API `ETag` / `If-None-Match` for conditional requests
- Cache key: `{owner}/{repo}/{endpoint}`

**FR-10: Cache Invalidation**
- `POST /{owner}/{package}/-/refresh` forces cache eviction for a package
- Optional: GitHub webhook endpoint to invalidate on new release events

### 5.4 Rate Limiting & Resilience

**FR-11: GitHub API Rate Management**
- Use a configured GitHub App or PAT for authenticated API requests (5,000 req/hr vs 60)
- Track rate limit headers and return `503 Retry-After` when exhausted
- Support multiple tokens with round-robin rotation

**FR-12: Request Rate Limiting**
- Per-IP rate limiting on incoming requests (configurable, default: 100 req/min)
- Return `429 Too Many Requests` with `Retry-After` header

---

## 6. Non-Functional Requirements

### 6.1 Performance

- **NFR-1:** P99 response latency < 200ms for cached requests
- **NFR-2:** P99 response latency < 1s for uncached requests (GitHub API dependent)
- **NFR-3:** Support 1,000+ concurrent connections

### 6.2 Reliability

- **NFR-4:** Zero persistent state — the service can be restarted or redeployed at any time without data loss
- **NFR-5:** Graceful degradation: if GitHub API is down, serve stale cache with warning header

### 6.3 Security

- **NFR-6:** TLS only (`.dev` enforces HSTS)
- **NFR-7:** No credentials stored server-side — tokens are pass-through only
- **NFR-8:** All GitHub API calls use HTTPS
- **NFR-9:** Input validation on owner/package path segments (alphanumeric, hyphens, underscores only)
- **NFR-10:** Content-Security-Policy headers on frontend pages

### 6.4 Observability

- **NFR-11:** Structured JSON logging (request path, status, latency, cache hit/miss, GitHub rate remaining)
- **NFR-12:** Prometheus metrics endpoint at `/metrics`
- **NFR-13:** Health check at `/healthz`

---

## 7. Tech Stack

| Component | Choice | Rationale |
|---|---|---|
| Language | Go 1.22+ | Single binary, fast HTTP, excellent concurrency |
| HTTP Router | `net/http` (stdlib) | No dependencies for core routing |
| Templates | `html/template` (stdlib) | Secure HTML rendering, no JS framework needed |
| Caching | In-memory (sync.Map or groupcache) | Stateless deploys, no Redis dependency |
| Deployment | Fly.io or Cloud Run | Edge deployment, scale-to-zero, global presence |
| Domain | `git-pkg.dev` | HSTS-enforced, developer-audience TLD |
| TLS | Automatic via platform | Fly.io / Cloud Run handle cert provisioning |
| CI/CD | GitHub Actions | Dogfooding — deploy on tag, same as users |

---

## 8. URL Routing & Content Negotiation

| URL Pattern | pip (PEP 503) | Browser (text/html) |
|---|---|---|
| `GET /` | N/A | Landing page + search |
| `GET /{owner}/` | Simple index: list of packages | Owner browse page |
| `GET /{owner}/{package}/` | Package page: list of wheels | Package detail + install instructions |
| `POST /{owner}/{package}/-/refresh` | Cache invalidation | Cache invalidation |
| `GET /healthz` | Health check | Health check |
| `GET /metrics` | Prometheus metrics | Prometheus metrics |

---

## 9. Example User Workflow

### Package Author (Zero Config Publishing)

```yaml
# .github/workflows/release.yml — add to your repo
name: Build & Release
on:
  push:
    tags: ["v*"]

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: "3.12"

      - name: Build wheel
        run: pip install build && python -m build --wheel

      - name: Upload to GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: dist/*.whl
```

Then:
```bash
git tag v1.0.0
git push origin v1.0.0
```

Done. Package is now installable.

### Package Consumer

```bash
# Install from a specific owner
pip install bytewire --index-url https://git-pkg.dev/shawn/

# Install from a GitHub Org
pip install internal-tool --index-url https://git-pkg.dev/regscale/

# Pin in requirements.txt
--index-url https://git-pkg.dev/regscale/
regscale-cli==1.5.0
internal-tool>=2.0

# Private repo (pass GitHub token)
pip install secret-pkg \
  --index-url https://git-pkg.dev/regscale/ \
  --header "Authorization=Bearer ${GH_TOKEN}"
```

---

## 10. Scope & Milestones

### MVP (v0.1) — Target: 2 weeks

- [ ] Core proxy: `/{owner}/{package}/` → PEP 503 HTML from GitHub Releases API
- [ ] Owner index: `/{owner}/` → list packages with wheel releases
- [ ] In-memory caching with 5-min TTL
- [ ] Landing page with Go templates (static, no search yet)
- [ ] Health check endpoint
- [ ] Dockerfile + Fly.io deployment
- [ ] GitHub Actions CI for the proxy itself
- [ ] `git-pkg.dev` domain registered and pointed

### v0.2 — Discovery & Polish

- [ ] Content negotiation (pip vs browser)
- [ ] Owner browse page with GitHub avatar, repo list
- [ ] Package detail page with install instructions, release history
- [ ] Search functionality on landing page (client-side GitHub API)
- [ ] `data-requires-python` support
- [ ] SHA256 hash fragments on download links

### v0.3 — Production Hardening

- [ ] GitHub token pass-through for private repos
- [ ] Rate limiting (inbound + GitHub API budget)
- [ ] Prometheus metrics
- [ ] Structured logging
- [ ] Cache invalidation webhook
- [ ] Stale-cache fallback on GitHub API errors

### v1.0 — Public Launch

- [ ] Documentation site (hosted on git-pkg.dev/docs or separate)
- [ ] Example repos with pre-built GitHub Actions
- [ ] "Add to your repo" one-click workflow template
- [ ] Blog post / announcement
- [ ] Optional: support for other artifact types (.tar.gz, conda)

---

## 11. Domain Decision

| Factor | `.org` | `.dev` |
|---|---|---|
| Price | ~$10/yr | ~$12-15/yr |
| HSTS (forced HTTPS) | No | Yes (preloaded) |
| Audience signal | Community/nonprofit | Developer tooling |
| Trust for package registry | Neutral | Strong (security built-in) |
| Availability | `git-pkg.org` likely available | `git-pkg.dev` likely available |

**Recommendation: `git-pkg.dev`**

The $3-5/yr premium buys mandatory HTTPS (critical for a package registry in the software supply chain), instant developer recognition, and a trust signal that `.org` simply doesn't carry for this use case.

---

## 12. Competitive Landscape

| Solution | Storage | Auth Required | Self-Hostable | Cost |
|---|---|---|---|---|
| PyPI | Centralized | Yes (tokens) | No | Free |
| Private PyPI (Artifactory, CodeArtifact) | Self-managed | Yes | Yes | $$$ |
| `pip install git+https://` | None | Optional | N/A | Free |
| **git-pkg.dev** | **None (GitHub CDN)** | **No** | **Yes** | **Free** |

### Unique Value Proposition

git-pkg.dev is the only solution where:
1. The author never creates an account on the registry
2. The registry stores zero bytes of package data
3. Identity is inherited from GitHub (no separate auth system)
4. The entire service is stateless and can be self-hosted as a single binary

---

## 13. Risks & Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| GitHub API rate limits | Service degradation | Authenticated requests (5k/hr), aggressive caching, token rotation |
| GitHub API changes | Breaking changes | Pin to API version, integration tests against live API |
| GitHub outage | Total service outage | Stale cache serving, clear error messaging |
| Abuse (typosquatting) | Trust erosion | Owner-scoped URLs prevent cross-owner confusion |
| Large orgs with many repos | Slow index pages | Pagination, parallel API calls, streaming HTML |
| pip compatibility edge cases | Install failures | Comprehensive PEP 503 compliance tests, test against pip versions |

---

## 14. Success Metrics

| Metric | Target (6 months post-launch) |
|---|---|
| Unique owners serving packages | 100+ |
| Monthly pip installs proxied | 10,000+ |
| P99 latency (cached) | < 200ms |
| Uptime | 99.9% |
| GitHub stars | 500+ |
| Zero security incidents | 0 |

---

## 15. Open Questions

1. **Should we support npm/cargo/other ecosystems?** The proxy pattern generalizes — GitHub Releases → registry index. Defer to post-v1.0.
2. **Should we offer a GitHub App?** Could auto-discover repos with releases, provide webhook-based cache invalidation. Adds complexity.
3. **Monetization?** The public service is free. Revenue could come from: hosted enterprise instances, priority caching, SLA guarantees, analytics dashboards.
4. **Should we vendor/proxy the actual `.whl` downloads?** Currently pip downloads directly from GitHub CDN. Proxying would enable download counting and offline resilience but adds bandwidth cost and liability.
