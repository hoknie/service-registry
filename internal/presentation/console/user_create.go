package console

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"svc-registry/internal/app"
	"svc-registry/internal/feature/access"
	accessservice "svc-registry/internal/feature/access/service"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
)

func userCreateCmd() *cobra.Command {
	var email, name, password string
	var superadmin bool
	cmd := &cobra.Command{
		Use:   "user:create --email E --name N [--superadmin]",
		Short: "Create an active user (password from USER_PASSWORD or stdin)",
		Long: "Create an active user. The password is read from USER_PASSWORD or, if unset, from the\n" +
			"first line of stdin — never from arguments.",
		Example: `  printf '%s\n' "$PW" | svc-registry user:create --email admin@example.com --name Admin --superadmin`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("password") {
				return errors.New("passwords are never accepted as arguments; set USER_PASSWORD or pipe it to stdin")
			}
			return failed(userCreate(cmd, access.CreateUser{Email: email, DisplayName: name, IsSuperadmin: superadmin}))
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "email of the user")
	cmd.Flags().StringVar(&name, "name", "", "display name of the user")
	cmd.Flags().BoolVar(&superadmin, "superadmin", false, "make the user a superadmin")
	cmd.Flags().StringVarP(&password, "password", "p", "", "")
	_ = cmd.Flags().MarkHidden("password")
	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func userCreate(cmd *cobra.Command, in access.CreateUser) error {
	logCfg, err := config.Load[config.LogConfig](config.ProcessEnv())
	if err != nil {
		return invalidConfig(err)
	}
	app.InitLogging(logCfg)
	type slice struct {
		DB   config.DbConfig
		Hash config.PasswordHashConfig
		User config.UserPasswordConfig
	}
	cfg, err := config.Load[slice](config.ProcessEnv())
	if err != nil {
		return invalidConfig(err)
	}
	in.Password = cfg.User.Password
	if in.Password == "" {
		if in.Password, err = readPasswordLine(cmd.InOrStdin()); err != nil {
			return err
		}
	}
	pool, err := postgres.ConnectLazy(cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()
	accounts := accessservice.New(accessservice.Deps{DB: postgres.New(pool), Hasher: auth.NewPasswordHasher(cfg.Hash)})
	user, err := accounts.InsertUser(context.Background(), in)
	if err != nil {
		code, message := apperr.From(err).Public()
		return fmt.Errorf("cannot create user: %s (%s)", code, message)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "created user %s %s\n", user.ID, user.Email)
	return nil
}

func readPasswordLine(stdin io.Reader) (string, error) {
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("cannot read the password from stdin: %w", err)
	}
	line = strings.TrimSuffix(line, "\n")
	return strings.TrimSuffix(line, "\r"), nil
}
