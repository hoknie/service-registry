package main

import (
	"os"

	"svc-registry/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.IO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}))
}
