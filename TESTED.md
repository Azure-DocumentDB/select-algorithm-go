# Validation evidence and live-run template

Evidence is scoped to a specific revision, environment, and mode. **Do not fill a PASS row from mock output or a planned run.** Pending live rows are not public-readiness certification.

## Local validation — 2026-10-12

Source: the downstream safe-comparison change containing this file. Exact committed source SHA, final test counts, and machine evidence are recorded in the accompanying release-review report (a file cannot embed its own final commit hash). No cloud credentials or Azure resources were used for these local tests.

- Platform: macOS arm64, Go **1.26.9**.
- `go mod verify`, `go build ./...`, `go vet ./...`, and `go test -race -count=1 ./...`: **PASS**. The race run recorded **45 top-level tests / 181 passing test events including subtests**, zero failed and zero skipped; package event counts: query extractor 1, comparison CLI 14, documentdb 166.
- A separate coverage run before the catalog review correction measured query extractor 55.6%, comparison CLI 83.3%, and documentdb 79.4% of statements. Coverage is not live proof.
- The real local extractor read the pinned **50-document / 1536D** upstream fixture (SHA-256 `60c08c2cdd439444ba8601d94a1f3487f4a9ee56cd48ba201d983c96722de6e4`); extracted vector equals the first stored vector. No network/model/database operation occurs in the extractor.
- `govulncheck@v1.9.0`: **zero reachable vulnerabilities** for the sample's imported package/call graph. The scanner also reports **module-only** advisories `GO-2026-5932` (`golang.org/x/crypto`) and `GO-2026-5841` (`github.com/klauspost/compress`) for vulnerable packages **not imported** by this application. This is not a claim that every package in the dependency modules is advisory-free. Re-run the gate when dependencies or imports change.
- All nine algorithm/metric wire contracts are exercised using the real MongoDB driver and mock wire responses; independent COS/L2/IP quality-oracle tests include an IP counterexample where self-match is not best.
- Independent review correction: preserve full BSON catalog metadata; reject partial/sparse/unique/TTL/hidden, malformed/unknown catalog semantics and extra vector options before seeding, while tolerating descriptive metadata/default false flags and unrelated ordinary indexes. Eight raw wire regressions prove rejection occurs after only `listIndexes`. GitHub-owned actions are SHA-pinned to verified official releases (checkout v4.2.2, setup-go v5.5.0).
- Tests cover insert-only preservation, failures, unverified/conflicting indexes, env safety, strict TLS/OIDC audience/expiry, model origin/redirect rejection, partial failure aggregation, deadlines, and exact CLI status contracts.
- Explicit tagged live tests without opt-in are required to exit nonzero. No untagged test connects to Azure or calls a live model.

These are **application/driver-contract checks, not real-engine or live authentication evidence**. The historical six local engine combinations are not evidence for this revised source, and unsupported local DiskANN is not a pass.


## Real managed Azure validation — 2026-10-12

Tested on one disposable Azure DocumentDB M30 in West US 2, one shard, MongoDB 8.0 compatibility, 128 GiB storage and HA disabled. Native SCRAM-SHA-256 and Microsoft Entra OIDC were exercised separately with verified TLS and a client-IP firewall. No OpenAI/model calls were made.

Fixture: 50 public pinned records with 1,536-dimensional vectors; the query is the first stored vector. This is a numerical vector/database contract test, not semantic relevance, performance, production-readiness or a tier/version support matrix. No real credentials or infrastructure identifiers are published.

IVF/HNSW/DiskANN × COS/L2/IP passed under both native and OIDC authentication: **18/18 cases**. Each case verified the actual index catalog, five results against an independent metric-specific rank calculation, creation on an already populated collection, and idempotent repeated seeding. Existing fixture edits remained unchanged. Both real comparison CLI executions returned all nine cases passed and exit 0.

Wrong password, disabled write opt-in, unsafe database name and foreign host were rejected with nonzero process exit and sanitized messages. The full catalog-semantic review fix was included. The local final suite has 45 top-level tests / 181 passing events; race/build/vet and govulncheck pass with zero reachable findings (module-only unimported advisories remain disclosed).

Tested implementation: `900cbd70e41e43b1feef88288459addf8873c419`.

## Optional Azure OpenAI — not live-validated

The optional model path has local contract tests only. Subscription OpenAI S0 eligibility is blocked. No live model call, semantic quality, or Azure OpenAI authorization is implied by the precomputed-vector Azure tests.

## Scope and cleanup

These tests used only separately approved synthetic namespaces on a disposable cluster. The sample never automatically deletes databases, collections or indexes. Resource cleanup is an explicit environment-owner responsibility. Publication is conditioned on verifying that the test resource group is gone, hosted CI passes, and an anonymous clean checkout works. MIT licensing and pinned upstream data provenance are retained.
