//go:build integration

package documentdb

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Opt-in tagged tests write only dedicated disposable collections. No cleanup or
// infrastructure is created. Any failed/unsupported case fails the Go test run.
func TestLiveComparisons(t *testing.T) {
	if os.Getenv("VECTOR_SEARCH_LIVE_TEST") != "1" {
		t.Fatal("live integration requires VECTOR_SEARCH_LIVE_TEST=1; no network calls made")
	}
	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.EmbeddingMode != "offline" || c.Dimensions != 1536 {
		t.Fatal("live contract requires EMBEDDING_MODE=offline and EMBEDDING_DIMENSIONS=1536; no model proof")
	}
	if c.DatabaseName == "go_disposable" || c.DatabaseName == "vectorsearch_disposable" {
		t.Fatal("live gate requires a dedicated run-suffixed disposable database")
	}
	if err := c.ValidateWriteScope("ivf"); err != nil {
		t.Fatal(err)
	}
	data, err := ReadFileReturnJSON(c.DataFile)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := PrepareSeed(data, c)
	if err != nil {
		t.Fatal(err)
	}
	query, err := ReadQueryVector(c.QueryVectorFile, c.Dimensions)
	if err != nil {
		t.Fatal(err)
	}
	self := false
	for _, doc := range docs {
		if reflect.DeepEqual(doc[c.VectorField], query) {
			self = true
		}
	}
	if !self {
		t.Fatal("live fixture query must exactly equal a seed vector; rank expectation remains metric-specific (IP need not select self)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	client, err := ConnectMongo(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := Disconnect(client); err != nil {
			t.Error(err)
		}
	}()
	t.Logf("REAL AZURE numeric fixture contract: auth=%s dimensions=%d documents=%d; NOT semantic/model/benchmark evidence", c.AuthMode, c.Dimensions, len(docs))
	for _, tc := range Cases() {
		t.Run(tc.Algorithm+"_"+tc.Metric, func(t *testing.T) {
			cc := c
			cc.Metric = tc.Metric
			caseCtx, caseCancel := context.WithTimeout(ctx, c.CaseTimeout)
			defer caseCancel()
			coll := client.Database(c.DatabaseName).Collection(cc.Collection(tc.Algorithm))
			before := snapshot(t, caseCtx, coll)
			// Seed a pre-existing fixture with an additional field. The normal source
			// lacks that field, so a replacement implementation would fail preservation.
			preserved := bson.M{}
			for k, v := range docs[0] {
				preserved[k] = v
			}
			preserved["sample_preservation_marker"] = "keep-existing-edit"
			if _, err := SeedData(caseCtx, coll, []bson.M{preserved}, c.BatchSize); err != nil {
				t.Fatal(err)
			}
			populated := snapshot(t, caseCtx, coll)
			first, err := runCheckedCase(caseCtx, coll, cc, tc.Algorithm, docs, query)
			if err != nil {
				t.Fatal(err)
			}
			after := snapshot(t, caseCtx, coll)
			for _, prior := range []map[string][]byte{before, populated} {
				for id, raw := range prior {
					if !reflect.DeepEqual(after[id], raw) {
						t.Fatal("existing document changed or disappeared")
					}
				}
			}
			second, err := runCheckedCase(caseCtx, coll, cc, tc.Algorithm, docs, query)
			if err != nil {
				t.Fatal(err)
			}
			if second.Seed.Inserted != 0 || second.Seed.Existing != int64(len(docs)) {
				t.Fatal("repeat seed was not idempotent")
			}
			if !reflect.DeepEqual(after, snapshot(t, caseCtx, coll)) {
				t.Fatal("repeat changed existing documents")
			}
			indexes, err := ListIndexes(caseCtx, coll)
			if err != nil {
				t.Fatal(err)
			}
			found, err := CompatibleIndex(indexes, cc, tc.Algorithm)
			if err != nil || !found {
				t.Fatal("catalog not verified")
			}
			t.Logf("%s/%s: exact metric top-%d verified; index verified; first inserted=%d existing=%d; repeat inserted=0 existing=%d; existing edits preserved", tc.Algorithm, tc.Metric, len(second.Results), first.Seed.Inserted, first.Seed.Existing, second.Seed.Existing)
		})
	}
}
func snapshot(t *testing.T, ctx context.Context, coll *mongo.Collection) map[string][]byte {
	t.Helper()
	cursor, err := coll.Find(ctx, bson.M{})
	if err != nil {
		t.Fatal(protect("live snapshot", err))
	}
	defer cursor.Close(ctx)
	result := map[string][]byte{}
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			t.Fatal(protect("snapshot decode", err))
		}
		id, err := json.Marshal(doc["_id"])
		if err != nil {
			t.Fatal("invalid snapshot ID")
		}
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal("invalid snapshot document")
		}
		result[string(id)] = raw
	}
	if err := cursor.Err(); err != nil {
		t.Fatal(protect("snapshot cursor", err))
	}
	return result
}
