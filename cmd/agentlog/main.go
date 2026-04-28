package main

import (
	"context"
	"fmt"
	"os"

	_ "github.com/drmaas/agentlog/internal/backends/markdown"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
