package main

import (
	"os"

	"syncmkv/internal/batch"
)

func main() {
	os.Exit(batch.Run(os.Args[1:], os.Stdout, os.Stderr))
}
