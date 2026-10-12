package documentdb

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func qualityFixture(t *testing.T) (Config, []bson.M, []float64) {
	c := configFixture(t)
	c.Dimensions = 2
	c.TopK = 3
	docs := []bson.M{
		{"_id": "self", c.VectorField: []float64{1, 0}},
		{"_id": "long", c.VectorField: []float64{2, .1}},
		{"_id": "near", c.VectorField: []float64{.8, .1}},
	}
	return c, docs, []float64{1, 0}
}
func rankedResults(ids ...string) []SearchResult {
	results := make([]SearchResult, len(ids))
	for i, id := range ids {
		results[i] = SearchResult{Document: bson.M{"_id": id}, Score: float64(i)}
	}
	return results
}
func TestIndependentMetricQualityAllNine(t *testing.T) {
	for _, tc := range Cases() {
		t.Run(tc.Algorithm+"/"+tc.Metric, func(t *testing.T) {
			c, docs, query := qualityFixture(t)
			c.Metric = tc.Metric
			ids := map[string][]string{"COS": {"self", "long", "near"}, "L2": {"self", "near", "long"}, "IP": {"long", "self", "near"}}[tc.Metric]
			if err := CheckQuality(rankedResults(ids...), docs, query, c); err != nil {
				t.Fatal(err)
			}
			wrong := append([]string(nil), ids...)
			wrong[0], wrong[1] = wrong[1], wrong[0]
			if err := CheckQuality(rankedResults(wrong...), docs, query, c); err == nil {
				t.Fatal("incorrect metric ranking certified")
			}
		})
	}
}
func TestQualityRejectsMissingDuplicateUnknownAndMalformed(t *testing.T) {
	c, docs, query := qualityFixture(t)
	for _, ids := range [][]string{nil, {"self"}, {"self", "self", "near"}, {"other", "long", "near"}, {"long", "self", "near"}} {
		if err := CheckQuality(rankedResults(ids...), docs, query, c); err == nil {
			t.Fatal("invalid quality result certified")
		}
	}
	c.Metric = "bad"
	if _, err := expectedRanks(docs, query, c, c.Metric); err == nil {
		t.Fatal("bad metric")
	}
	c.Metric = "COS"
	docs[0][c.VectorField] = []float64{0, 0}
	if _, err := expectedRanks(docs, query, c, c.Metric); err == nil {
		t.Fatal("zero vector")
	}
}
func TestQualityAcceptsMetricTiesNotUniversalSelfMatch(t *testing.T) {
	c, docs, query := qualityFixture(t)
	c.TopK = 1
	c.Metric = "IP"
	if err := CheckQuality(rankedResults("self"), docs, query, c); err == nil {
		t.Fatal("IP self-match is not generally best")
	}
	docs = append(docs, bson.M{"_id": "same", c.VectorField: []float64{1, 0}})
	c.Metric = "COS"
	if err := CheckQuality(rankedResults("same"), docs, query, c); err != nil {
		t.Fatal(err)
	}
}
func TestCompareAllAggregatesPartialFailureAndSanitizes(t *testing.T) {
	c := configFixture(t)
	var attempted []string
	sentinel := mongo.CommandError{Code: 67, Message: "Invalid search index kind vector-diskann secret://credential"}
	report, err := compareCases(context.Background(), c, func(ctx context.Context, cc Config, algorithm string) (RunResult, error) {
		attempted = append(attempted, cc.Collection(algorithm))
		if algorithm == "diskann" {
			return RunResult{}, protect("index", sentinel)
		}
		return RunResult{Results: rankedResults("self")}, nil
	})
	if err == nil || report.Succeeded != 6 || report.Failed != 3 || len(attempted) != 9 || len(report.Cases) != 9 {
		t.Fatalf("partial success misreported: %+v %v", report, err)
	}
	if strings.Contains(err.Error(), "credential") && strings.Contains(err.Error(), "secret://") {
		t.Fatal("raw driver secret exposed")
	}
	unique := map[string]bool{}
	for _, name := range attempted {
		unique[name] = true
	}
	if len(unique) != 9 {
		t.Fatal("collections are not isolated")
	}
	for _, row := range report.Cases[6:] {
		if row.Status != "unsupported" || row.Quality != "not-verified" || row.ServerCode != 67 || !strings.Contains(row.Error, "NOT engine proof") {
			t.Fatalf("unsupported misreported: %+v", row)
		}
	}
	var commandErr mongo.CommandError
	if !errors.As(err, &commandErr) {
		t.Fatal("cause lost")
	}
}
func TestCompareAllContinuesAfterMiddleFailure(t *testing.T) {
	c := configFixture(t)
	calls := 0
	cause := errors.New("private error")
	report, err := compareCases(context.Background(), c, func(context.Context, Config, string) (RunResult, error) {
		calls++
		if calls == 2 {
			return RunResult{}, cause
		}
		return RunResult{}, nil
	})
	if calls != 9 || report.Failed != 1 || report.Succeeded != 8 || !errors.Is(err, cause) {
		t.Fatal("middle failure swallowed or stopped later cases")
	}
}
func TestCompareCaseDeadlineAndCanceledRemainingCases(t *testing.T) {
	c := configFixture(t)
	c.CaseTimeout = time.Millisecond
	calls := 0
	start := time.Now()
	report, err := compareCases(context.Background(), c, func(ctx context.Context, _ Config, _ string) (RunResult, error) {
		calls++
		<-ctx.Done()
		return RunResult{}, nil
	})
	if calls != 9 || report.Failed != 9 || !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatal("deadlines not bounded/aggregated")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	report, err = compareCases(ctx, c, func(ctx context.Context, _ Config, _ string) (RunResult, error) {
		calls++
		return RunResult{}, ctx.Err()
	})
	if calls != 9 || report.Failed != 9 || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled cases hidden")
	}
}
func TestAllNineWireContractsAndActualQuality(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock).CreateCollection(false))
	for _, tc := range Cases() {
		mt.Run(tc.Algorithm+"_"+tc.Metric, func(mt *mtest.T) {
			c, docs, query := qualityFixture(mt.T)
			c.Metric = tc.Metric
			coll := mt.Client.Database(c.DatabaseName).Collection(c.Collection(tc.Algorithm))
			ns := c.DatabaseName + "." + coll.Name()
			ids := map[string][]string{"COS": {"self", "long", "near"}, "L2": {"self", "near", "long"}, "IP": {"long", "self", "near"}}[tc.Metric]
			responses := []bson.D{}
			for _, id := range ids {
				responses = append(responses, bson.D{{Key: "document", Value: bson.M{"_id": id}}, {Key: "score", Value: 1.0}})
			}
			mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 26, Message: "not found"}), mtest.CreateSuccessResponse(), mtest.CreateCursorResponse(0, ns, mtest.FirstBatch, indexDoc(mt.T, c, tc.Algorithm)), mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 3}, bson.E{Key: "nModified", Value: 0}), mtest.CreateCursorResponse(0, ns, mtest.FirstBatch, responses...))
			report, err := runCheckedCase(context.Background(), coll, c, tc.Algorithm, docs, query)
			if err != nil || report.Seed.Existing != 3 {
				mt.Fatalf("case failed: %v", err)
			}
			var commands []string
			for _, e := range mt.GetAllStartedEvents() {
				commands = append(commands, e.CommandName)
				if e.DatabaseName != c.DatabaseName {
					mt.Fatal("wrong database")
				}
				if e.CommandName == "update" {
					var cmd bson.M
					if err := bson.Unmarshal(e.Command, &cmd); err != nil {
						mt.Fatal(err)
					}
					updates := cmd["updates"].(bson.A)
					for _, v := range updates {
						u := v.(bson.M)
						if u["upsert"] != true || len(u["u"].(bson.M)) != 1 || u["u"].(bson.M)["$setOnInsert"] == nil {
							mt.Fatal("seed replaces existing data")
						}
					}
				}
			}
			if !reflect.DeepEqual(commands, []string{"listIndexes", "createIndexes", "listIndexes", "update", "aggregate"}) {
				mt.Fatalf("unsafe commands: %v", commands)
			}
		})
	}
}
func TestAcknowledgedIndexMustExistAndMatchBeforeSeed(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock).CreateCollection(false))
	for _, mode := range []string{"missing", "wrong-metric", "create-already-exists"} {
		mt.Run(mode, func(mt *mtest.T) {
			c := configFixture(mt.T)
			coll := mt.Client.Database(c.DatabaseName).Collection(c.Collection("ivf"))
			ns := c.DatabaseName + "." + coll.Name()
			mt.AddMockResponses(mtest.CreateCursorResponse(0, ns, mtest.FirstBatch))
			if mode == "create-already-exists" {
				mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 85, Message: "already exists"}))
			} else {
				mt.AddMockResponses(mtest.CreateSuccessResponse())
				if mode == "missing" {
					mt.AddMockResponses(mtest.CreateCursorResponse(0, ns, mtest.FirstBatch))
				} else {
					c2 := c
					c2.Metric = "IP"
					mt.AddMockResponses(mtest.CreateCursorResponse(0, ns, mtest.FirstBatch, indexDoc(mt.T, c2, "ivf")))
				}
			}
			if _, err := runOnCollection(context.Background(), coll, c, "ivf", []bson.M{{"_id": "test"}}, []float64{1}); err == nil {
				mt.Fatal("unverified index accepted")
			}
			for _, e := range mt.GetAllStartedEvents() {
				if e.CommandName == "update" || e.CommandName == "aggregate" {
					mt.Fatal("continued after unverified index")
				}
			}
		})
	}
}

func TestCatalogRejectsNumericLookalikeStrings(t *testing.T) {
	c := configFixture(t)
	for _, field := range []string{"dimensions", "numLists"} {
		index := indexInfo(t, c, "ivf")
		index.CosmosSearchOptions[field] = "1"
		if _, err := CompatibleIndex([]IndexInfo{index}, c, "ivf"); err == nil {
			t.Fatal("string catalog setting accepted")
		}
	}
}
func TestComparisonRejectsInputBeforeNetwork(t *testing.T) {
	c := configFixture(t)
	c.AllowWrites = false
	_, err := RunCompareAll(context.Background(), c)
	var input InputError
	if !errors.As(err, &input) {
		t.Fatal("missing opt-in did not reject as input")
	}
	c.AllowWrites = true
	c.DataFile = t.TempDir() + "/missing.json"
	_, err = RunCompareAll(context.Background(), c)
	if !errors.As(err, &input) || !strings.Contains(err.Error(), "read data file") {
		t.Fatal("missing fixture did not reject before connection")
	}
}

func TestFullCatalogSemanticsWireRejectsBeforeSeed(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock).CreateCollection(false))
	for _, option := range []bson.E{
		{Key: "partialFilterExpression", Value: bson.M{"HotelId": "one"}},
		{Key: "sparse", Value: true}, {Key: "unique", Value: true}, {Key: "hidden", Value: true},
		{Key: "expireAfterSeconds", Value: int32(0)},
		{Key: "collation", Value: bson.M{"locale": "en"}},
		{Key: "unknownFutureOption", Value: false}, {Key: "sparse", Value: "false"},
	} {
		mt.Run(option.Key+"_"+fmt.Sprint(option.Value), func(mt *mtest.T) {
			c := configFixture(mt.T)
			coll := mt.Client.Database(c.DatabaseName).Collection(c.Collection("ivf"))
			ns := c.DatabaseName + "." + coll.Name()
			catalog := append(indexDoc(mt.T, c, "ivf"), option)
			mt.AddMockResponses(mtest.CreateCursorResponse(0, ns, mtest.FirstBatch, catalog))
			if _, err := runOnCollection(context.Background(), coll, c, "ivf", []bson.M{{"_id": "1"}}, []float64{1}); err == nil {
				mt.Fatal("incompatible full catalog accepted")
			}
			events := mt.GetAllStartedEvents()
			if len(events) != 1 || events[0].CommandName != "listIndexes" {
				mt.Fatal("mutation/search occurred after catalog rejection")
			}
		})
	}
}
func TestCatalogDefaultsAndUnrelatedIndexesPreserved(t *testing.T) {
	c := configFixture(t)
	index := indexInfo(t, c, "ivf")
	index.Extra = bson.M{"v": int32(2), "ns": "metadata", "background": true, "sparse": false, "unique": false, "hidden": false}
	ordinary := IndexInfo{Name: "unrelated_ttl", Key: bson.M{"other": int32(1)}, Extra: bson.M{"expireAfterSeconds": int32(1)}}
	found, err := CompatibleIndex([]IndexInfo{ordinary, index}, c, "ivf")
	if err != nil || !found {
		t.Fatalf("harmless metadata or unrelated index rejected: %v", err)
	}
	index.CosmosSearchOptions["unknownFutureOption"] = true
	if _, err := CompatibleIndex([]IndexInfo{index}, c, "ivf"); err == nil {
		t.Fatal("extra vector semantics accepted")
	}
}
