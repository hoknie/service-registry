package main

import (
	"os"

	"svc-registry/internal/presentation/console"
)

func main() {
	os.Exit(console.Run(os.Args[1:], console.IO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}))
}
