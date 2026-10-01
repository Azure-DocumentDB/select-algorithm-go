# Upstream provenance

- Source repository: https://github.com/Azure-Samples/documentdb-samples
- Exact source revision: [f5a2675bbf349737234d0391e3ebee24b260e06f](https://github.com/Azure-Samples/documentdb-samples/commit/f5a2675bbf349737234d0391e3ebee24b260e06f)
- Source directory: [ai/select-algorithm-go](https://github.com/Azure-Samples/documentdb-samples/tree/f5a2675bbf349737234d0391e3ebee24b260e06f/ai/select-algorithm-go)
- Extraction date: 2026-09-30
- License: original Microsoft MIT notice preserved verbatim in [LICENSE.md](LICENSE.md)
- History model: independently maintained source snapshot, not a GitHub fork or ownership transfer

## Standalone adaptations

Source is at repository root. The README is rewritten for that layout, and placeholder-only configuration/data setup replaces reliance on the parent checkout.
- Adapted source file: `.gitignore`
- Adapted source file: `README.md`

Hotel fixtures are not redistributed: [data/README.md](data/README.md) pins the original input locations and checksums. Saved output, Learn-only quickstart pages, and devcontainer automation are omitted. Shared infrastructure and root monorepo CI are not copied.

No dependency versions or vector/agent algorithms are intentionally changed. The Go agent module namespace and local data/environment-file paths are adjusted where necessary. Invalid monorepo-only npm entry points are removed rather than advertised as working commands.

### Go layout adjustment (2026-10-01)

Upstream places the program in a directory named `src/`. In a standalone repository
`go build ./...` fails there with `build output "src" already exists and is a directory`,
because the default binary name collides with the directory name, so standard Go tooling
and CI cannot build the repository.

The package directory was renamed `src/` to `cmd/select-algorithm/`. The package is
unchanged and still a single `package main`; no files were split, no identifiers were
renamed, and no Go source content was modified. `go.mod` and `go.sum` are untouched.

`go build ./...` now exits 0. `go vet ./...` still reports 14 pre-existing
`primitive.E struct literal uses unkeyed fields` warnings inherited from upstream, left
unchanged.

## Maintenance

There is no automatic synchronization. Review upstream changes against this pinned baseline, apply only relevant fixes, retain notices, and record the imported revision and local adaptations here.
Issue/PR history is not transferred. The maintainers of the repository hosting this extraction own its downstream fixes; contribute reusable fixes upstream under its contribution policy.
