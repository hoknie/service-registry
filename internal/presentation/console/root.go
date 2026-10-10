package console

import (
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/spf13/cobra"

	"svc-registry/internal/app"
	"svc-registry/internal/presentation/http/handlers"
)

const Version = "0.1.0"

func init() { handlers.MCPVersion = Version }

type IO struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

type runError struct{ err error }

func (e *runError) Error() string { return e.err.Error() }
func (e *runError) Unwrap() error { return e.err }

func failed(err error) error {
	if err == nil {
		return nil
	}
	return &runError{err}
}

func invalidConfig(err error) error {
	return failed(errors.New("invalid configuration: " + err.Error()))
}

func newRoot(stdio IO) *cobra.Command {
	root := &cobra.Command{
		Use:   "svc-registry",
		Short: "Service registry: JSON API under /api + the web UI served from files",
		Long: "svc-registry — service registry: JSON API under /api + the web UI served from files.\n\n" +
			"Configuration comes from environment variables; see .env.example.",
		Version:       Version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetVersionTemplate("svc-registry {{.Version}}\n")
	root.Flags().BoolP("version", "V", false, "print the version")
	root.SetIn(stdio.Stdin)
	root.SetOut(stdio.Stdout)
	root.SetErr(stdio.Stderr)
	root.AddCommand(serveCmd(), dbMigrateCmd(), dbRollbackCmd(), userCreateCmd(), forgeSyncCmd(), mcpStdioCmd(), secretsRotateCmd(), versionCmd())
	return root
}

func Run(args []string, stdio IO) int {
	app.DisableLogging()
	root := newRoot(stdio)
	root.SetArgs(args)
	cmd, err := root.ExecuteC()
	if err == nil {
		return 0
	}
	fmt.Fprintf(stdio.Stderr, "error: %v\n", err)
	var run *runError
	if errors.As(err, &run) {
		slog.Error(err.Error())
		return 1
	}
	fmt.Fprintln(stdio.Stderr)
	fmt.Fprint(stdio.Stderr, cmd.UsageString())
	return 2
}
