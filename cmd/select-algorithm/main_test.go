package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Azure-DocumentDB/select-algorithm-go/internal/documentdb"
)

func goodReport() documentdb.ComparisonReport {
	r := documentdb.ComparisonReport{Succeeded: 9}
	for _, c := range documentdb.Cases() {
		r.Cases = append(r.Cases, documentdb.CaseResult{Case: c, Status: "passed", Quality: "exact-metric-top-k-verified"})
	}
	return r
}
func TestCommandExitCodes(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		loadErr error
		report  documentdb.ComparisonReport
		runErr  error
		want    int
	}{
		{name: "success", report: goodReport(), want: 0},
		{name: "partial", report: documentdb.ComparisonReport{Succeeded: 6, Failed: 3, Cases: goodReport().Cases}, runErr: errors.New("secret password"), want: 1},
		{name: "all_failure", runErr: errors.New("secret password"), want: 1},
		{name: "incomplete_nil_error", report: documentdb.ComparisonReport{Succeeded: 6}, want: 1},
		{name: "invalid_config", loadErr: errors.New("safe config error"), want: 2},
		{name: "unknown_args", args: []string{"reset"}, want: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			calls := 0
			code := command(context.Background(), tc.args, &out, &stderr, func() (documentdb.Config, error) { return documentdb.Config{}, tc.loadErr }, func(context.Context, documentdb.Config) (documentdb.ComparisonReport, error) {
				calls++
				return tc.report, tc.runErr
			})
			if code != tc.want {
				t.Fatalf("want %d got %d", tc.want, code)
			}
			if strings.Contains(stderr.String(), "secret password") {
				t.Fatal("raw runtime error leaked")
			}
			if tc.want == 2 && calls != 0 {
				t.Fatal("config/args reached runner")
			}
		})
	}
}
func TestActualProcessExitCodes(t *testing.T) {
	if mode := os.Getenv("SELECT_TEST_EXIT_HELPER"); mode != "" {
		load := func() (documentdb.Config, error) {
			if mode == "2" {
				return documentdb.Config{}, errors.New("invalid config")
			}
			return documentdb.Config{}, nil
		}
		run := func(context.Context, documentdb.Config) (documentdb.ComparisonReport, error) {
			if mode == "1" {
				return goodReport(), errors.New("private runtime error")
			}
			return goodReport(), nil
		}
		os.Exit(command(context.Background(), nil, io.Discard, io.Discard, load, run))
	}
	for _, want := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(want), func(t *testing.T) {
			child := exec.Command(os.Args[0], "-test.run=^TestActualProcessExitCodes$")
			child.Env = append(os.Environ(), fmt.Sprintf("SELECT_TEST_EXIT_HELPER=%d", want))
			err := child.Run()
			got := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				got = exit.ExitCode()
			}
			if got != want {
				t.Fatalf("actual process exit %d != %d", got, want)
			}
		})
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("private path") }
func TestOutputFailureNonzero(t *testing.T) {
	if got := command(context.Background(), nil, failedWriter{}, io.Discard, func() (documentdb.Config, error) { return documentdb.Config{}, nil }, func(context.Context, documentdb.Config) (documentdb.ComparisonReport, error) {
		return goodReport(), nil
	}); got != 1 {
		t.Fatalf("code %d", got)
	}
}
func TestRealBinaryRejectsArgumentsAndMissingConfigWithoutNetwork(t *testing.T) {
	bin := t.TempDir() + "/compare"
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	for _, args := range [][]string{nil, {"reset"}} {
		cmd := exec.Command(bin, args...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 {
			t.Fatalf("expected binary code2: %v %s", err, out)
		}
	}
}

func TestRealInputErrorCodeWithoutNetwork(t *testing.T) {
	t.Setenv("MONGO_AUTH_MODE", "oidc")
	t.Setenv("MONGO_HOST", "sample.mongocluster.cosmos.azure.com")
	t.Setenv("MONGO_CONNECTION_STRING", "")
	t.Setenv("MONGO_DATABASE", "go_disposable_test")
	t.Setenv("MONGO_COLLECTION", "")
	t.Setenv("EMBEDDING_MODE", "offline")
	t.Setenv("QUERY_VECTOR_FILE", "missing-query.json")
	t.Setenv("DATA_FILE_WITH_VECTORS", t.TempDir()+"/missing.json")
	t.Setenv("VECTOR_SEARCH_ALLOW_WRITES", "true")
	var out, stderr bytes.Buffer
	if got := command(context.Background(), nil, &out, &stderr, documentdb.LoadConfig, documentdb.RunCompareAll); got != 2 || !strings.Contains(stderr.String(), "read data file") {
		t.Fatalf("input not rejected: code=%d %s", got, stderr.String())
	}
}
