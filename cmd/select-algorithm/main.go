package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/Azure-DocumentDB/select-algorithm-go/internal/documentdb"
)

// Exact binary exit codes: 0=all nine passed, 1=runtime/partial/output failure,
// 2=arguments/configuration/input rejected before DB mutation. go run itself may
// report any nonzero child exit as shell code 1; use the built binary for scripts.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(command(ctx, os.Args[1:], os.Stdout, os.Stderr, documentdb.LoadConfig, documentdb.RunCompareAll))
}

type runner func(context.Context, documentdb.Config) (documentdb.ComparisonReport, error)

func command(ctx context.Context, args []string, out, stderr io.Writer, load func() (documentdb.Config, error), run runner) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "No arguments supported; configure through the process environment.")
		return 2
	}
	c, err := load()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	report, err := run(ctx, c)
	if len(report.Cases) > 0 {
		if encodeErr := json.NewEncoder(out).Encode(report); encodeErr != nil {
			fmt.Fprintln(stderr, "Unable to write comparison report.")
			return 1
		}
	}
	if err != nil {
		// Report no raw SDK/driver errors (which can embed secrets and documents).
		var input documentdb.InputError
		if errors.As(err, &input) {
			fmt.Fprintln(stderr, input)
			return 2
		}
		fmt.Fprintln(stderr, "Comparison failed; inspect sanitized per-case results. No cleanup was performed.")
		return 1
	}
	if report.Succeeded != 9 || report.Failed != 0 || len(report.Cases) != 9 {
		fmt.Fprintln(stderr, "Incomplete comparison report.")
		return 1
	}
	for _, row := range report.Cases {
		if row.Status != "passed" || row.Quality != "exact-metric-top-k-verified" {
			fmt.Fprintln(stderr, "Unverified comparison case.")
			return 1
		}
	}
	return 0
}
