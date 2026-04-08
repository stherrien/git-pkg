# git-pkg.dev

A stateless Go proxy that turns GitHub Releases into pip-installable Python packages.

No accounts. No uploads. No config. Tag a release, attach a `.whl`, and your users can `pip install` it.

## How it works

git-pkg.dev translates the GitHub Releases API into a [PEP 503](https://peps.python.org/pep-0503/) package index on the fly. It never stores your code or credentials — pip downloads wheels directly from GitHub's CDN.

## Install a package

```bash
pip install mypackage --index-url https://git-pkg.dev/owner/
```

### Private repos

```bash
pip install mypackage --index-url https://x:${GH_TOKEN}@git-pkg.dev/myorg/
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
