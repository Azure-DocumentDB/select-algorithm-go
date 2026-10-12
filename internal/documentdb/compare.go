package documentdb

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Case identifies an isolated algorithm/metric contract, not a performance trial.
type Case struct {
	Algorithm string `json:"algorithm"`
	Metric    string `json:"metric"`
}

func Cases() []Case {
	result := make([]Case, 0, 9)
	for _, algorithm := range []string{"ivf", "hnsw", "diskann"} {
		for _, metric := range []string{"COS", "L2", "IP"} {
			result = append(result, Case{algorithm, metric})
		}
	}
	return result
}

type CaseResult struct {
	Case
	Collection string      `json:"collection"`
	Status     string      `json:"status"`
	Seed       InsertStats `json:"seed"`
	Hits       int         `json:"hits"`
	Quality    string      `json:"quality"`
	Error      string      `json:"error,omitempty"`
	ServerCode int32       `json:"server_code,omitempty"`
}
type ComparisonReport struct {
	Database  string       `json:"database"`
	AuthMode  string       `json:"auth_mode"`
	Mode      string       `json:"mode"`
	Cases     []CaseResult `json:"cases"`
	Succeeded int          `json:"succeeded"`
	Failed    int          `json:"failed"`
	Notice    string       `json:"notice"`
}

type InputError struct{ cause error }

func (e InputError) Error() string { return e.cause.Error() }
func (e InputError) Unwrap() error { return e.cause }

// RunCompareAll validates all input before credentials/network, then attempts all
// nine cases on separate collections. A failed/unsupported case is never success.
func RunCompareAll(ctx context.Context, c Config) (report ComparisonReport, err error) {
	if e := c.ValidateWriteScope("ivf"); e != nil {
		return report, InputError{e}
	}
	data, e := ReadFileReturnJSON(c.DataFile)
	if e != nil {
		return report, InputError{e}
	}
	docs, e := PrepareSeed(data, c)
	if e != nil {
		return report, InputError{e}
	}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	query, e := QueryVector(ctx, c)
	if e != nil {
		if c.EmbeddingMode == "offline" {
			return report, InputError{e}
		}
		return report, e
	}
	// Validate the oracle before any writes as well.
	for _, metric := range []string{"COS", "L2", "IP"} {
		if _, e := expectedRanks(docs, query, c, metric); e != nil {
			return report, InputError{e}
		}
	}
	client, e := ConnectMongo(ctx, c)
	if e != nil {
		return report, e
	}
	defer func() { err = errors.Join(err, Disconnect(client)) }()
	return compareCases(ctx, c, func(caseCtx context.Context, cc Config, algorithm string) (RunResult, error) {
		return runCheckedCase(caseCtx, client.Database(cc.DatabaseName).Collection(cc.Collection(algorithm)), cc, algorithm, docs, query)
	})
}

type caseRunner func(context.Context, Config, string) (RunResult, error)

func compareCases(ctx context.Context, c Config, run caseRunner) (ComparisonReport, error) {
	report := ComparisonReport{Database: c.DatabaseName, AuthMode: c.AuthMode, Mode: c.EmbeddingMode, Cases: make([]CaseResult, 0, 9), Notice: "Precomputed numeric vector/index contract, not model relevance or a performance benchmark. Scores from different metrics are not comparable. Unsupported cases are failures, not engine proof."}
	if c.EmbeddingMode != "offline" {
		report.Notice = "Optional model path: NOT live-validated. Exact numeric ranking checks are not semantic relevance or benchmark evidence. Scores across metrics are not comparable."
	}
	var failures []error
	for _, testCase := range Cases() {
		cc := c
		cc.Metric = testCase.Metric
		caseCtx, cancel := context.WithTimeout(ctx, c.CaseTimeout)
		result, err := run(caseCtx, cc, testCase.Algorithm)
		// A runner may return nil after exhausting its deadline. Do not certify it.
		if err == nil {
			err = caseCtx.Err()
		}
		cancel()
		row := CaseResult{Case: testCase, Collection: cc.Collection(testCase.Algorithm), Seed: result.Seed, Hits: len(result.Results), Quality: "not-verified"}
		if err != nil {
			row.Status = "failed"
			row.Error = protect("comparison case", err).Error()
			var serverErr mongo.CommandError
			if errors.As(err, &serverErr) {
				row.ServerCode = serverErr.Code
				msg := strings.ToLower(serverErr.Message)
				if strings.Contains(msg, "invalid search index kind") || strings.Contains(msg, "not supported") || strings.Contains(msg, "unsupported") {
					row.Status = "unsupported"
					row.Error = "server reports unsupported capability; NOT engine proof; check tier/service support"
				}
			}
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				row.Status = "deadline-or-canceled"
			}
			failures = append(failures, fmt.Errorf("%s/%s: %w", testCase.Algorithm, testCase.Metric, protect("comparison case", err)))
			report.Failed++
		} else {
			row.Status = "passed"
			row.Quality = "exact-metric-top-k-verified"
			report.Succeeded++
		}
		report.Cases = append(report.Cases, row)
	}
	return report, errors.Join(failures...)
}

func runCheckedCase(ctx context.Context, collection *mongo.Collection, c Config, algorithm string, docs []bson.M, query []float64) (RunResult, error) {
	report, err := runOnCollection(ctx, collection, c, algorithm, docs, query)
	if err != nil {
		return report, err
	}
	return report, CheckQuality(report.Results, docs, query, c)
}

type ranked struct {
	id    string
	value float64
}

// expectedRanks is an independent exhaustive numeric oracle. No server score
// conversion is assumed: L2 is ascending distance; COS/IP are descending.
func expectedRanks(docs []bson.M, query []float64, c Config, metric string) ([]ranked, error) {
	if err := ValidateVector(query, c.Dimensions); err != nil {
		return nil, err
	}
	if metric != "COS" && metric != "L2" && metric != "IP" {
		return nil, errors.New("invalid oracle metric")
	}
	if len(docs) == 0 {
		return nil, errors.New("empty oracle fixture")
	}
	ranks := make([]ranked, 0, len(docs))
	seen := map[string]bool{}
	for _, doc := range docs {
		id, ok := doc["_id"].(string)
		if !ok || id == "" || seen[id] {
			return nil, errors.New("oracle requires unique string IDs")
		}
		seen[id] = true
		v, err := numericVector(doc[c.VectorField], c.Dimensions)
		if err != nil {
			return nil, err
		}
		var dot, distance, nq, nv float64
		for i, x := range query {
			dot += x * v[i]
			d := x - v[i]
			distance += d * d
			nq += x * x
			nv += v[i] * v[i]
		}
		value := -dot // ascending sort: negative is better for COS/IP
		if metric == "COS" {
			value = -(dot / math.Sqrt(nq) / math.Sqrt(nv))
		}
		if metric == "L2" {
			value = distance
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errors.New("numeric oracle overflow")
		}
		ranks = append(ranks, ranked{id, value})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].value == ranks[j].value {
			return ranks[i].id < ranks[j].id
		}
		return ranks[i].value < ranks[j].value
	})
	return ranks, nil
}

// CheckQuality verifies count, unique known IDs, and each rank against exhaustive
// metric-specific expected ranks (ties tolerate 1e-6 relative error). It does not
// compare scores across metrics or claim approximate recall at production scale.
// Preserved edits/additional documents can cause this fixture check to fail; we
// deliberately do not overwrite/remove them to manufacture a passing result.
func CheckQuality(results []SearchResult, docs []bson.M, query []float64, c Config) error {
	expected, err := expectedRanks(docs, query, c, c.Metric)
	if err != nil {
		return err
	}
	if len(results) != min(c.TopK, len(expected)) {
		return errors.New("quality check: result count differs from fixture top K")
	}
	values := map[string]float64{}
	for _, item := range expected {
		values[item.id] = item.value
	}
	seen := map[string]bool{}
	for i, result := range results {
		id, ok := result.Document["_id"].(string)
		value, known := values[id]
		if !ok || !known || seen[id] {
			return errors.New("quality check: duplicate or unknown fixture result")
		}
		if math.IsNaN(result.Score) || math.IsInf(result.Score, 0) {
			return errors.New("quality check: non-finite score")
		}
		seen[id] = true
		tolerance := 1e-6 * math.Max(1, math.Abs(expected[i].value))
		if math.Abs(value-expected[i].value) > tolerance {
			return errors.New("quality check: result rank differs from exact metric oracle")
		}
	}
	return nil
}
