# Docs site

The Paperclip Operator documentation site is built with
[MkDocs](https://www.mkdocs.org/) and the
[Material for MkDocs](https://squidfunk.github.io/mkdocs-material/) theme.

The Markdown sources live in the repository-root `docs/` directory; this
`docs-site/` directory only holds the build configuration and tooling.

## Local development

```sh
# Create the virtualenv and serve at http://127.0.0.1:8000
make docs-serve

# Strict build (fails on broken links / warnings)
make docs-build
```

## API reference

`docs/api-reference.md` is generated from the Go API types in `api/v1alpha1`
using [crd-ref-docs](https://github.com/elastic/crd-ref-docs). Regenerate it
after any API type change:

```sh
make api-docs
```
