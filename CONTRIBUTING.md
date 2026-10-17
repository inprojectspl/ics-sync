# Contributing to ics-sync

Thanks for your interest. Security problems go through
[SECURITY.md](SECURITY.md), not the issue tracker.

## Development

You need Go 1.26. Before opening a pull request, run:

```sh
gofmt -l .        # must print nothing
go vet ./...
go test ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

## Conventions

- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/),
  for example `fix(bundle): ...` or `feat: ...`.
- Code, comments and documentation are in English.
- Record user-visible changes under `[Unreleased]` in [CHANGELOG.md](CHANGELOG.md).
- The manifest is a contract with the ICS backend (`Manifest` in
  `backend/app/routes/bindings/_models.py`). A change to its fields, the
  signature encoding or the multipart field names is a breaking change and
  needs a new major version (`:2`).

## Documentation

The site at <https://inprojectspl.github.io/ics-sync/> is built with
[MkDocs](https://www.mkdocs.org) and [Material for MkDocs](https://squidfunk.github.io/mkdocs-material/)
from `docs/` and `mkdocs.yml`. The Docs workflow builds it for every pull request that touches
them and publishes it to GitHub Pages from `main`. To preview it locally:

```sh
pip install -r docs/requirements.txt
mkdocs serve
```

`mkdocs build --strict` must pass: a broken link or anchor fails the build. Link to files outside
`docs/` by their full GitHub URL. In a fork, set **Settings > Pages > Source** to
**GitHub Actions** before the first deploy.

## Releases

Tag `main` with `vX.Y.Z`. CI publishes the image to
`ghcr.io/inprojectspl/ics-sync` as `:X.Y.Z` and moves `:1` for `v1.*` tags.
