package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const secretRefMax = 64 << 10

func ResolveSecretRef(ref string) (string, error) {
	scheme, target, ok := strings.Cut(ref, ":")
	switch {
	case ok && scheme == "env":
		v := strings.TrimSpace(os.Getenv(target))
		if v == "" {
			return "", fmt.Errorf("environment variable %s is not set", target)
		}
		return v, nil
	case ok && scheme == "file":
		f, err := os.Open(target)
		if err != nil {
			return "", fmt.Errorf("secret file: %w", err)
		}
		defer f.Close()
		line, err := bufio.NewReader(io.LimitReader(f, secretRefMax)).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("secret file: %w", err)
		}
		if line = strings.TrimRight(line, "\r\n"); strings.TrimSpace(line) == "" {
			return "", errors.New("secret file is empty")
		}
		return line, nil
	}
	return "", errors.New("unknown secret reference")
}
