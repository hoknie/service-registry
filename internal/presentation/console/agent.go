package console

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"svc-registry/internal/app"
	"svc-registry/internal/platform/config"
)

func agentCmd() *cobra.Command {
	var address, cluster, tokenEnv, healthAddr string
	var once bool
	cmd := &cobra.Command{
		Use:   "agent --url <address> --cluster <id> --token-env <NAME> [--once | --health-addr <host:port>]",
		Short: "Observe the Kubernetes cluster of this pod and report it to the registry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return failed(runAgent(cmd.Context(), agentFlags{address, cluster, tokenEnv, healthAddr, once}))
		},
	}
	cmd.Flags().StringVar(&address, "url", "", "address of the registry, e.g. https://registry.example")
	cmd.Flags().StringVar(&cluster, "cluster", "", "id of the cluster in the registry")
	cmd.Flags().StringVar(&tokenEnv, "token-env", "", "name of the environment variable with the agent token (svca_…)")
	cmd.Flags().BoolVar(&once, "once", false, "send one report and exit")
	cmd.Flags().StringVar(&healthAddr, "health-addr", "", "address for the /healthz and /readyz checks, e.g. :8081 (off when empty)")
	cmd.MarkFlagsMutuallyExclusive("once", "health-addr")
	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("cluster")
	_ = cmd.MarkFlagRequired("token-env")
	return cmd
}

type agentFlags struct {
	address, cluster, tokenEnv, healthAddr string
	once                                   bool
}

func runAgent(parent context.Context, f agentFlags) error {
	env := config.ProcessEnv()
	logCfg, err := config.Load[config.LogConfig](env)
	if err != nil {
		return invalidConfig(err)
	}
	app.InitLogging(logCfg)
	var errs []error
	out, err := config.Load[config.OutboundConfig](env)
	errs = append(errs, err)
	k8s, err := config.Load[config.K8sClientConfig](env)
	if err = errors.Join(append(errs, err)...); err != nil {
		return invalidConfig(err)
	}
	token := strings.TrimSpace(env[f.tokenEnv])
	if token == "" {
		return fmt.Errorf("%s is not set: put the agent token of the cluster there", f.tokenEnv)
	}
	u, err := url.Parse(strings.TrimSuffix(f.address, "/"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("--url must be an http(s) address of the registry")
	}
	id, err := uuid.Parse(f.cluster)
	if err != nil {
		return fmt.Errorf("--cluster must be the id of the cluster in the registry")
	}
	runner, err := app.OpenAgent(app.AgentSettings{URL: u.String(), Cluster: id, Token: token, Outbound: out, K8s: k8s})
	if err != nil {
		return err
	}
	if f.healthAddr != "" {
		stopHealth, err := serveAgentHealth(f.healthAddr, runner.Health())
		if err != nil {
			return err
		}
		defer stopHealth()
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runner.Run(ctx, f.once)
}
