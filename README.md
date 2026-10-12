# select-algorithm-go

Compare **IVF, HNSW, and DiskANN × COS, L2, and IP** using nine isolated Azure DocumentDB collections. The default quickstart uses **precomputed 1536-dimensional vectors**, needs **no Azure OpenAI resource**, and makes no model calls. Each result is checked against an independent exhaustive, metric-specific numeric ranking.

This is a small vector/index contract sample, **not a semantic relevance test, production recall study, or latency benchmark**. Raw scores across different metrics are not comparable. Inner-product top results need not be the query's own document when vector norms differ.

Standalone downstream of the Microsoft sample; see [UPSTREAM.md](UPSTREAM.md), the preserved [MIT license](LICENSE.md), and the evidence status in [TESTED.md](TESTED.md). A local mock pass is not live Azure authentication or search-engine proof.

## Safety and prerequisites

- **Go 1.26.9 or newer**, Git, network access, and an explicitly approved **disposable** Azure DocumentDB cluster. The module rejects older Go toolchains. No Docker or global tool installation is required.
- Choose a service tier/region supporting your intended algorithms. DiskANN and metric availability are server capabilities; a server rejection is reported as a failure, not silently skipped or counted as engine proof. Confirm current Azure requirements rather than treating an example SKU as universal.
- Azure bills for the cluster/storage even when no model is used. This sample creates no Azure resources or firewall rules and performs no automatic cleanup. The operator owns provisioning and eventual resource deletion.
- Use least-privilege database credentials and deliberately scoped network access. Native SCRAM and Microsoft Entra OIDC are separate explicit modes; there is no automatic credential fallback. Entra sign-in alone does not register/grant a database principal.
- Writes require `VECTOR_SEARCH_ALLOW_WRITES=true`. Database names must be `go_disposable` or `vectorsearch_disposable`, optionally suffixed by `_` and a lowercase run ID; maximum 63 characters. Use a run-suffixed name for every live gate.
- Exactly nine collections are used: `<prefix>_{ivf,hnsw,diskann}_{cos,l2,ip}`. `MONGO_COLLECTION` is rejected, preventing accidental reuse of one collection across metrics.
- **No drop, delete, reset, replace, or cleanup operation exists.** All seed batches use deterministic IDs with `$setOnInsert` upserts. Existing documents/edits are preserved. Catalog conflicts fail before normal seeding instead of replacing indexes. Successful index creation is followed by catalog verification, even on populated collections.
- Failures may leave newly inserted documents/indexes behind. Retry safely; do not erase existing data to force a passing fixture. Extra documents or edited vectors may invalidate fixture-quality expectations; choose a fresh dedicated prefix instead.

## Fresh-start quickstart (bash/zsh)

Run from this repository root. Configuration is read **only from the process environment**; `.env` is neither loaded nor executed. `.env.example` documents placeholders. Never commit credentials or paste them into issue/CI logs.

### 1. Retrieve and verify the pinned fixture

Follow [data/README.md](data/README.md) for the upstream source, terms, and PowerShell alternative. These commands refuse to overwrite a local data file and verify SHA-256 before it is used:

```sh
mkdir -p data
(set -C; curl --fail --location \
  'https://raw.githubusercontent.com/Azure-Samples/documentdb-samples/f5a2675bbf349737234d0391e3ebee24b260e06f/ai/data/Hotels_Vector.json' \
  > data/Hotels_Vector.json)
printf '%s  %s\n' \
  60c08c2cdd439444ba8601d94a1f3487f4a9ee56cd48ba201d983c96722de6e4 \
  data/Hotels_Vector.json | shasum -a 256 -c -
# Stop if either download or checksum verification fails.
go mod download
go mod verify
go run ./cmd/query-vector -input data/Hotels_Vector.json -output data/query-vector.json
```

The extractor validates the complete data file, uses the first stored vector, refuses to overwrite its output, and never calls a model/database. The pinned input has 50 hotels with 1536D vectors. This is a controlled numeric fixture, not a newly embedded natural-language query.

### 2. Configure an approved namespace and explicit write opt-in

```sh
export MONGO_DATABASE=go_disposable_example_20261012
export MONGO_COLLECTION_PREFIX=hotels
export EMBEDDING_MODE=offline
export DATA_FILE_WITH_VECTORS=data/Hotels_Vector.json
export QUERY_VECTOR_FILE=data/query-vector.json
export EMBEDDING_DIMENSIONS=1536
export TOP_K=5
export VECTOR_SEARCH_TIMEOUT=10m
export VECTOR_SEARCH_CASE_TIMEOUT=1m
export VECTOR_SEARCH_ALLOW_WRITES=true
```

`EMBEDDED_FIELD` defaults to `DescriptionVector`; `SEED_ID_FIELD` to `HotelId`; `LOAD_SIZE_BATCH` to 100. All document vectors and IDs must be valid; the sample does not silently filter bad records or invent random IDs. `_id` must be a nonempty string, or a nonempty string `HotelId` generates a deterministic ID. Timeouts are bounded (overall 1s–30m, per case 1s–5m). The per-case deadline prevents one unsupported/stalled case from consuming the entire run; the overall deadline still wins.

### 3. Choose one database authentication mode

**Native SCRAM:** set `MONGO_AUTH_MODE=connection-string` and supply `MONGO_CONNECTION_STRING` through your approved secret mechanism. It must be an authenticated `mongodb+srv://` URI for `*.mongocluster.cosmos.azure.com` (including `.global.`) with no database path; percent-encode reserved username/password characters. Do not put real credentials in shell history. TLS certificate/hostname verification is mandatory; bypass options are rejected. Retryable writes are disabled for service compatibility.

**Microsoft Entra OIDC:**

```sh
export MONGO_AUTH_MODE=oidc
unset MONGO_CONNECTION_STRING
export MONGO_HOST='<your-cluster>.global.mongocluster.cosmos.azure.com'
# Optional principal username if your service registration requires it:
# export MONGO_OIDC_USERNAME='<registered-principal>'
az login
```

`MONGO_CLUSTER_NAME` can derive the standard host if `MONGO_HOST` is unset. The Azure Identity credential chain requests only `https://ossrdbms-aad.database.windows.net/.default`; token expiry is propagated to the MongoDB driver, and empty/expired/near-expiry tokens fail. The client pings before claiming authenticated connectivity. Public Azure DocumentDB hosts only; no arbitrary Mongo endpoints, custom gateways, self-signed TLS bypass, or password fallback.

### 4. Build and compare

```sh
mkdir -p bin
go build -o bin/select-algorithm ./cmd/select-algorithm
./bin/select-algorithm
```

One authenticated connection runs all nine cases sequentially, each with a distinct collection and metric-specific index. Output is sanitized JSON with per-case status, seed counts, hit count, quality status, and a numeric server code when available—never connection URIs, tokens, full documents, or vectors. All nine cases are attempted after common configuration/data/authentication prerequisites succeed. Any failure remains nonzero even when the other eight succeed; subsequent cases still run (or are recorded canceled after the overall deadline).

**Exact built-binary exit codes:**

- `0`: all nine cases passed catalog, seed, result-count, and exact metric top-K checks.
- `1`: runtime/authentication, any partial/unsupported case, deadline, incomplete report, or output failure.
- `2`: arguments, configuration, or local fixture rejected before database mutation.

`go run ./cmd/select-algorithm` is convenient interactively, but the Go launcher can map a nonzero child status to shell code 1; scripts should use the binary.

Quality compares returned IDs/ranks to exhaustive COS/L2/IP calculations over the input fixture, allows numeric ties within relative tolerance `1e-6`, rejects duplicate/unknown results, and checks `min(TOP_K, fixture size)` hits. This small exact-quality gate can legitimately fail for approximate search or modified collections; it is not a promise of perfect ANN recall on arbitrary datasets. No cross-metric score comparison is made.

## Local tests and security gate (no Azure credentials)

```sh
go test -race -count=1 ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

Tests cover configuration rejection, TLS/OIDC scope/expiry, optional-model origin/redirect protection, seed preservation/failure, index conflicts/catalog verification, real driver wire mocks for all nine cases, independent quality oracles, partial failure aggregation, deadlines, and CLI status codes. Driver wire mocks exercise the driver/application contract, **not a real vector engine**. Vulnerability output must distinguish reachable findings (blocking) from unimported-module/unreached-symbol advisories. Dependencies are intentionally patched; MongoDB v1 driver is retained at 1.17.9 for this sample and is deprecated upstream, so a v2 migration is future maintenance rather than an implied current upgrade.

The GitHub workflow uses only GitHub-owned checkout/setup actions, `contents: read`, Go 1.26.9 and `stable`, checksum verification, race tests, vet/build, and the vulnerability gate. It contains no credentials, model calls, deploy, or live integration step.

## Explicit live validation

After a separate approval for a dedicated disposable namespace and the local security gates:

```sh
VECTOR_SEARCH_LIVE_TEST=1 go test -tags=integration -count=1 -v -timeout=20m \
  ./internal/documentdb -run '^TestLiveComparisons$'
```

The live gate requires the flag, write opt-in, a run-suffixed dedicated DB, offline mode, 1536D vectors, and a query exactly equal to a stored seed vector. It uses the correct independent oracle for **each metric**, not an invalid universal IP self-match assumption. Each of nine subtests seeds a preserved marker into a pre-existing fixture document, verifies the catalog and exact ranks, repeats safely with zero insertions, and compares before/after document snapshots. Any unsupported case fails the run. Repeat separately with both auth modes and separate `MONGO_COLLECTION_PREFIX` values such as `native` and `oidc`. Tagged execution without explicit opt-in fails immediately instead of silently skipping green. Operator-owned resource cleanup is still required.

## Optional Azure OpenAI path — NOT live validated

This compatibility path is **not required for the quickstart and is not currently certified**; live model validation was blocked by S0 account eligibility. Do not interpret mock tests as model or Azure OpenAI authentication proof. No model resource is provisioned here.

To deliberately make a billable query-embedding request, set `EMBEDDING_MODE=azure-openai`, unset `QUERY_VECTOR_FILE`, and supply `AZURE_OPENAI_EMBEDDING_ENDPOINT=https://<resource>.openai.azure.com/`, `AZURE_OPENAI_EMBEDDING_MODEL=<compatible-deployment>`, and optional `AZURE_OPENAI_EMBEDDING_KEY` through a secret manager (otherwise Azure Identity). `QUERY_TEXT` supplies the text. The request uses the `/openai/v1/` API and must produce the configured dimensions. Only the public HTTPS Azure OpenAI origin on port 443 is accepted, before credential acquisition; redirects are never followed, including same-host redirects. Database and model authentication are independent. Foreign origins/proxies are unsupported. Exact numeric ranking remains a fixture check, not semantic quality evidence.

## Troubleshooting and maintenance

- **Configuration/code 2:** check required env names, non-overlapping collection prefix, write opt-in, vector dimensions, and input paths. Legacy `AZURE_DOCUMENTDB_DATABASENAME`/`DOCUMENTDB_CLUSTER_NAME` are not the new configuration contract.
- **Authentication/runtime code 1:** inspect the Azure principal registration, RBAC, SCRAM credentials, firewall and DNS outside the sample. SDK details are intentionally withheld from output to protect credentials/data.
- **Unsupported server code / index conflict:** confirm tier/capability or choose a fresh namespace; the program will not drop an incompatible index or relabel partial success.
- **Empty/late search:** only empty successful searches retry, bounded by context; authentication, server, decode, and unsupported errors do not retry indefinitely. Increase an approved timeout only after understanding the cause.
- **Quality mismatch:** verify the pinned data/query and check for preserved edits/additional documents. Do not reset populated collections to hide a failure.

Upstream changes are reviewed and ported manually. Retain Microsoft MIT notices and dataset source terms. Infrastructure remains [upstream](https://github.com/Azure-Samples/documentdb-samples/tree/f5a2675bbf349737234d0391e3ebee24b260e06f/infra); broad defaults require separate review and are not automatically deployed.

## Dated validation

See [TESTED.md](TESTED.md) for the real managed-Azure results, authentication/algorithm matrix, local checks and explicit limits.
