// Copyright 2026 The go-qrl Authors
// This file is part of the go-qrl library.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cyyber/qrl-tests/endtoend/runner"
)

func main() {
	if err := runner.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
