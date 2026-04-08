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

## Run locally

```bash
go run .
# Server starts on :9090
```

Set `GITHUB_TOKEN` for higher API rate limits and private repo access:

```bash
GITHUB_TOKEN=ghp_xxx go run .
```

## Deploy

```bash
fly launch
fly secrets set GITHUB_TOKEN=ghp_xxx
fly deploy
```

## License

MIT
