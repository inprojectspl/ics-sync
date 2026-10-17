# The binding file

The binding file, `.inprojects.ics` by default, tells ics-sync which binding to use and what to
publish. This page lists its fields and gives recipes for the common layouts.

ICS shows the file filled in for your document when you create a binding (step 2 of the setup
screen, see [Set up a repository](setup.md#3-commit-the-binding-file)). It holds no secret:
commit it to the repository.

## Fields

```yaml
version: 1
binding_id: 1eb8ec80-927a-4087-9c1f-262db49441bb
public_key: |
  -----BEGIN PUBLIC KEY-----
  MCowBQYDK2VwAyEA...
  -----END PUBLIC KEY-----
publish:
  source: .
  kind: md_collection
  exclude:
    - "node_modules/**"
metadata:
  title: "Engineering handbook"
```

| Field | Required | Meaning |
|---|---|---|
| `version` | yes | The file format. Must be `1`. |
| `binding_id` | yes | The binding's ID from ICS. It is part of the URL the CLI posts to and part of the signed manifest. Do not edit it. |
| `public_key` | no | The public key of the pair. ICS verifies against the key it stores, not this one. Keep it so that anyone can compare it with the key shown under **Active bindings**. |
| `publish.source` | no, default `.` | The folder to publish, relative to the directory the job runs in (the repository root in a normal GitLab job). Must be a directory. |
| `publish.kind` | no | The document type: `md_single`, `md_collection` or `html_bundle`. The CLI does not send it; ICS checks the content against the type of the document itself. Keep it equal to what the dialog filled in, as a note for readers. |
| `publish.exclude` | no | A list of patterns for files to leave out. See [Exclude files](#exclude-files). |
| `metadata.title` | no | For people reading the file. The CLI does not send it, and the document keeps the title set in ICS. |

An older binding file may still have a `publish.entry` field. The CLI ignores it: ICS picks the
entry file itself (see [below](#what-the-content-must-look-like)), so you can delete the line.

The CLI stops with an error if the file cannot be read, is not valid YAML, has a key it does not
know (a misspelt `source` or `exclude` would otherwise publish the whole working directory), has
a `version` other than `1` (a missing `version` counts as `0`), has no `binding_id` or has a
malformed exclude pattern. The messages are listed in
[Troubleshooting](troubleshooting.md#errors-from-the-binding-file).

## What the content must look like

ICS checks the bundle against the document's type, not against `publish.kind`:

| Document type | The bundle must contain | The entry file ICS opens |
|---|---|---|
| Markdown document (`md_single`) | exactly one `.md` file | that file |
| Markdown collection (`md_collection`) | at least one `.md` file | the shallowest `README.md`, else the shallowest `index.md`, else the shallowest `.md` file (alphabetically first) |
| HTML bundle (`html_bundle`) | at least one `.html` file | the shallowest `index.html`, else the first `.html` or `.htm` file at the shallowest level |

A bundle that breaks the rule is refused and the document stays as it was. ICS also caps the
bundle; the defaults are 500 files and 100 MiB of unpacked content, and an HTML bundle must also
fit the document owner's HTML bundle limit (5 MB unless an administrator changed it). Your
instance may use other values.

## Exclude files

Each pattern in `publish.exclude` is matched against every file's path relative to
`publish.source`, and also against the file name alone. A file that matches any pattern is left
out.

| Pattern | Leaves out |
|---|---|
| `drafts/**` | the folder `drafts` at the top of the source, and everything in it |
| `*.test.*` | any file whose name matches, in any folder, for example `api.test.md` |
| `docs/draft.md` | that one file |
| `docs/*.tmp` | `.tmp` files directly in `docs` (`*` does not cross a `/`) |
| `.gitlab-ci.yml` | every file with that name (a pattern without a `/` also matches file names in subfolders) |

Rules worth knowing:

- `**` works only as a trailing `/**`. `dir/**` is anchored at the top of `publish.source`: it
  does not match a `dir` folder deeper down. `**/name` is not supported.
- A folder name alone does not exclude the folder: `drafts` matches a file called `drafts`, not
  the files inside a folder of that name. Use `drafts/**`.
- Other patterns follow Go's [`path.Match`](https://pkg.go.dev/path#Match) syntax: `*`, `?`,
  `[abc]` and `[a-z]`.
- Quote patterns that start with `*`, as YAML treats an unquoted `*` as an alias.
- A malformed pattern, such as an unclosed `[`, stops the CLI before anything is uploaded.

Some files are always left out whatever you write: `.git` metadata, symbolic links, other
non-regular files and empty directories. See
[How it works](how-it-works.md#what-is-always-left-out).

## Recipes

### Publish the whole repository as a collection

For a repository that is a Markdown knowledge base. Leave out the files that are not content:

```yaml
version: 1
binding_id: 1eb8ec80-927a-4087-9c1f-262db49441bb
publish:
  source: .
  kind: md_collection
  exclude:
    - ".gitlab-ci.yml"
    - ".inprojects.ics"
    - ".gitignore"
    - "node_modules/**"
```

Without the excludes, those files are stored in the document along with the Markdown.

### Publish one folder

For a code repository whose documentation is in `docs/`. The binding file still sits in the
repository root; only `source` changes:

```yaml
version: 1
binding_id: 1eb8ec80-927a-4087-9c1f-262db49441bb
publish:
  source: docs
  kind: md_collection
  exclude:
    - "drafts/**"
```

Paths in the document are relative to `docs`, so `docs/guides/release.md` appears as
`guides/release.md`, and `drafts/**` means `docs/drafts`.

### Publish a single Markdown file

A Markdown document holds exactly one `.md` file, so point `source` at a folder that contains
only that file, or exclude the rest:

```yaml
version: 1
binding_id: 1eb8ec80-927a-4087-9c1f-262db49441bb
publish:
  source: policy
  kind: md_single
```

`source` must be a folder, not a file.

### Publish a built static site as an HTML bundle

For a site or report produced by a build step (a static site generator, a notebook export, a
dashboard build). The build writes to `public/`, and the sync job publishes that folder:

```yaml
version: 1
binding_id: 1eb8ec80-927a-4087-9c1f-262db49441bb
publish:
  source: public
  kind: html_bundle
  exclude:
    - "*.map"
```

The folder must exist when ics-sync runs, so the sync job needs the build output as an artifact
of an earlier job; see [After a build job](ci-recipes.md#publish-the-output-of-a-build-job). Keep
an `index.html` at the top of the folder so ICS opens the right page, and keep the bundle under
the owner's HTML bundle limit; leaving out source maps (`*.map`) often helps.

### Publish two folders into two documents

One repository can feed several documents. Create a binding for each document, give each its own
binding file and its own private key variable, and run the CLI once per file:

```yaml
# docs/handbook.ics
version: 1
binding_id: 1eb8ec80-927a-4087-9c1f-262db49441bb
publish:
  source: handbook
  kind: md_collection
```

```yaml
# docs/runbooks.ics
version: 1
binding_id: 7c0d5a3e-2f41-4a8b-9e6d-0b1c2d3e4f50
publish:
  source: runbooks
  kind: md_collection
```

`source` is relative to the directory the job runs in, not to the binding file, so both files
name folders from the repository root. The matching job is in
[CI job recipes](ci-recipes.md#publish-several-documents-from-one-repository).

## Related

- [Set up a repository](setup.md)
- [CI job recipes](ci-recipes.md)
- [How it works](how-it-works.md)
- [Troubleshooting](troubleshooting.md)
