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

At initial extraction, no dependency versions or vector/agent algorithms were intentionally changed. The Go module namespace and local data/environment-file paths were adjusted where necessary. Subsequent downstream security and behavior changes are recorded below; the initial extraction statement does not describe current dependency versions.

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

### Safe comparison and security revision (2026-10-12)

Starting from downstream `63c04f9c7097bf438f5d2b7862581f8cbe0290af`:

- Replaced destructive one-collection reset/create/search/drop orchestration with nine separate algorithm/metric namespaces, validated write opt-in, insert-only deterministic seeding, and catalog compatibility/visibility checks. No drop/delete/reset path remains.
- Added default precomputed query-vector mode and a local extractor. Optional model requests are explicit and remain live-unvalidated (S0 eligibility blocked).
- Added independent exhaustive COS/L2/IP expected-rank checks, truthful aggregate nonzero failures for partial/unsupported cases, context-bound per-case/global deadlines, sanitized JSON output, and exact CLI status semantics. This is not a cross-metric score comparison or benchmark.
- Ported and adapted safe configuration, native SCRAM/Entra OIDC authentication, TLS/error handling, data primitives, model-origin/redirect protection, and regression tests from the public sibling [`Azure-DocumentDB/vector-search-go`](https://github.com/Azure-DocumentDB/vector-search-go) revision `ad093a18a1372661af054e3446f08d104b0a069a` (verified identical tree to the read-only reference checkout `b62e1d43ddcfb42b2b6c88a24c79ec1bca94b40c`) (same preserved Microsoft MIT licensing); no upstream baseline revision is being relabeled by this reuse.
- Require Go 1.26.9; patch MongoDB driver to 1.17.9, x/net to 0.60.0, x/text to 0.42.0, and compatible transitive dependencies. Keep an explicit reachable-vulnerability gate and distinguish module-only advisories.
- Added unit/driver-wire tests for all nine cases, security/failure/quality/deadline/CLI contracts, an explicit opt-in 1536D live integration gate, and GitHub-owned read-only CI. Historical unkeyed BSON vet warnings are corrected in the rewritten implementation.
- Rewrote README/configuration and added TESTED evidence/status template. Preserve all original MIT notices and pinned dataset attribution/checksums; do not redistribute fixture data or credentials.

## Maintenance

There is no automatic synchronization. Review upstream changes against this pinned baseline, apply only relevant fixes, retain notices, and record the imported revision and local adaptations here.
Issue/PR history is not transferred. The maintainers of the repository hosting this extraction own its downstream fixes; contribute reusable fixes upstream under its contribution policy.
