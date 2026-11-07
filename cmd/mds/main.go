package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"mdstrack/internal/app"
)

func main() {
	ctx := context.Background()
	program := app.New(os.Stdout, os.Stderr)
	if err := program.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "mds:", err)
		os.Exit(1)
	}
}

// Keep time and strings referenced here so `go vet` catches platform changes in
// this command package even though behavior lives in internal/app.
var _ = strings.TrimSpace
var _ = time.Now
