package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
)

type listedPackage struct {
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

func main() {
	pkgs, err := listPackages()
	if err != nil {
		fmt.Fprintln(os.Stderr, "layout:", err)
		os.Exit(2)
	}
	var problems []string
	for _, p := range pkgs {
		for _, imp := range p.allImports() {
			problems = append(problems, check(p.ImportPath, imp)...)
		}
	}
	sort.Strings(problems)
	for _, problem := range problems {
		fmt.Println("layout:", problem)
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
}

func listPackages() ([]listedPackage, error) {
	goBin := os.Getenv("GO")
	if goBin == "" {
		goBin = "go"
	}
	out, err := exec.Command(goBin, "list", "-json", "./...").Output() // #nosec G204 -- the go binary comes from the recipe's GO variable
	if err != nil {
		return nil, fmt.Errorf("go list: %w", err)
	}
	var pkgs []listedPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p listedPackage
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			return pkgs, nil
		} else if err != nil {
			return nil, fmt.Errorf("go list output: %w", err)
		}
		pkgs = append(pkgs, p)
	}
}

func (p listedPackage) allImports() []string {
	all := append([]string{}, p.Imports...)
	all = append(all, p.TestImports...)
	return append(all, p.XTestImports...)
}
