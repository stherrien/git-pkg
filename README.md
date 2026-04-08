# git-pkg.dev

A stateless Go proxy that turns GitHub Releases into pip-installable Python packages.

No accounts. No uploads. No config. Tag a release, attach a `.whl`, and your users can `pip install` it.

## How it works

git-pkg.dev translates the GitHub Releases API into a [PEP 503](https://peps.python.org/pep-0503/) package index on the fly. It never stores your code or credentials — pip downloads wheels directly from GitHub's CDN.

## Install a package

### pip

```bash
pip install mypackage --index-url https://git-pkg.dev/owner/
```

### uv

```bash
uv pip install mypackage --index-url https://git-pkg.dev/owner/
```

Or configure it in `pyproject.toml` so your whole team gets it automatically:

```toml
[[tool.uv.index]]
name = "git-pkg"
url = "https://git-pkg.dev/owner/"
```

### poetry

```toml
# pyproject.toml
[[tool.poetry.source]]
name = "git-pkg"
url = "https://git-pkg.dev/owner/"
```

### pdm

```toml
# pyproject.toml
[[tool.pdm.source]]
name = "git-pkg"
url = "https://git-pkg.dev/owner/"
```

### Private repos

Pass your GitHub token via standard Basic Auth — works with any tool:

```bash
# pip
pip install mypackage --index-url https://x:${GH_TOKEN}@git-pkg.dev/myorg/

# uv
uv pip install mypackage --index-url https://x:${GH_TOKEN}@git-pkg.dev/myorg/
```

### Pin in requirements.txt

```
--index-url https://git-pkg.dev/myorg/
mypackage==1.2.3
another-pkg>=2.0
```

## Publish a package

1. Add a GitHub Action to build your `.whl` on tagged releases:

```yaml
# .github/workflows/release.yml
name: Release
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
      - run: pip install build && python -m build --wheel
      - uses: softprops/action-gh-release@v2
        with:
          files: dist/*.whl
```

2. Tag a release:

```bash
git tag v1.0.0
git push origin v1.0.0
```

That's it. No registry account. No API tokens. No upload step.

## Why not just use GitHub Pages?

You can host a static PEP 503 index on GitHub Pages yourself. It works. But you take on all the maintenance, and your consumers deal with the inconsistency.

| | GitHub Pages (DIY) | git-pkg.dev |
|---|---|---|
| **Author setup** | Write a CI pipeline to build the index, configure Pages, maintain the generation script | Tag a release, attach a `.whl` — done |
| **Multi-repo index** | You build and maintain an aggregator across repos | Automatic — all repos under an owner are indexed |
| **Consumer experience** | Every author has a different Pages URL | One consistent pattern: `git-pkg.dev/owner/` |
| **Private packages** | Pages are public or require GitHub Enterprise | Pass a token, works with any private repo |
| **New release** | Wait for Pages CI to rebuild the index | Available immediately — the proxy reads releases in real time |
| **Discoverability** | None unless you build a frontend | Browse any owner's packages at `git-pkg.dev/owner/` |
| **Maintenance** | Index script breaks, Pages config drifts, you debug it | Zero — the proxy handles everything |

GitHub Pages is the right choice if you want full control and don't mind the upkeep. git-pkg.dev is for everyone else.

## Security

git-pkg.dev is secure by design — not by policy, but by architecture.

- **No credentials stored.** git-pkg.dev has no user accounts, no passwords, no API tokens. There is nothing to breach.
- **No artifacts stored.** Wheels are never uploaded to or cached by the proxy. pip downloads directly from GitHub's CDN. There is no artifact storage to compromise.
- **No code execution.** The proxy serves static HTML. It does not build, extract, or run any package code.
- **Identity is GitHub.** Package authenticity is tied to the GitHub repo owner. You can verify the source, the commit, and the release in one place. No anonymous uploads.
- **TLS enforced.** The `.dev` TLD requires HTTPS via HSTS preloading in all browsers. Connections cannot be downgraded.
- **Private repos stay private.** Tokens are passed through to the GitHub API per-request and never logged or stored. Without a valid token, private packages return nothing.
- **Rate limited.** Per-IP rate limiting protects against abuse (100 req/min, burst of 20).
- **Fully auditable.** The proxy is open source. The entire codebase is a single Go binary with zero dependencies beyond the standard library.

## License

MIT
