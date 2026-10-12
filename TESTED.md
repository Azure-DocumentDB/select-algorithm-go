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

## Real Azure precomputed fixture — NOT YET RUN for this change

The parent/operator owns the approved cluster, credential delivery, execution, evidence, and cleanup. Fill the following after actual execution; do not infer readiness from the template.

| Field | Actual value |
| --- | --- |
| Exact source SHA | PENDING |
| UTC date/time, runtime and OS | PENDING |
| Azure region / SKU / server version | PENDING |
| Dedicated DB and auth-specific collection prefixes | PENDING (no credential values) |
| Fixture SHA-256, dimensions, document count, query origin | PENDING |
| Native SCRAM all-nine test command/exit | NOT RUN |
| Entra OIDC all-nine test command/exit | NOT RUN |
| Per-case COS/L2/IP exact numeric top-K assertion | NOT RUN |
| Catalog verification and idempotent rerun | NOT RUN |
| Existing document/edit preservation | NOT RUN |
| Unsupported/server-rejected cases | PENDING; failures must remain nonzero |
| Sanitized evidence locations | PENDING |
| Resource cleanup confirmation/cost caveat | Operator-owned, PENDING |

Commands (after local gates, explicit approval, and correctly configured process environment):

```sh
VECTOR_SEARCH_LIVE_TEST=1 go test -tags=integration -count=1 -v -timeout=20m \
  ./internal/documentdb -run '^TestLiveComparisons$'
# Repeat under independently configured native and OIDC modes/prefixes.
go build -o bin/select-algorithm ./cmd/select-algorithm
./bin/select-algorithm
```

Report each of IVF/HNSW/DiskANN × COS/L2/IP, authentication mode, exit code and actual assertions separately. The test uses an exact metric-specific reference; do not assert IP self-match or compare raw COS/L2/IP scores. Partial success, unsupported service cases, skipped tests, and model-unavailable runs must never be represented as all-nine PASS.

## Optional Azure OpenAI — explicitly unvalidated

No live model request was made during this change. Validation remains blocked by S0 account eligibility; optional HTTP/auth/redirect behavior is covered only by local mocks. Neither database OIDC proof nor an offline query vector proves Azure OpenAI authentication, semantic quality, or deployment compatibility. A future live model test requires separate permission, available model eligibility, exact endpoint/API contract, dimensions, costs, and sanitized evidence.
