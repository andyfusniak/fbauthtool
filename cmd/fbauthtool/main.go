package main

import (
	"context"
	"fmt"
	"os"

	firebase "firebase.google.com/go/v4"
	"github.com/andyfusniak/fbauthtool/internal/cli"
	"github.com/andyfusniak/fbauthtool/internal/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/api/option"
)

var version string
var gitCommit string

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "[run] error: %+v", err)
		os.Exit(1)
	}
}

func run() error {
	// read the config file
	cfg, err := config.NewConfigFromFile()
	if err != nil {
		return err
	}

	app := cli.NewApp(
		cli.WithVersion(version),
		cli.WithGitCommit(gitCommit),
		cli.WithConfig(cfg),
	)

	root := cobra.Command{
		Use:     "fbauthtool",
		Short:   "fbauthtool is a command line tool to manage Firebase Auth",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			v := ctx.Value(cli.AppKey("app"))
			app := v.(*cli.App)

			cfg := app.Config()
			opt := option.WithCredentialsJSON(cfg.Current.FBCreds)
			fbApp, err := firebase.NewApp(ctx, nil, opt)
			if err != nil {
				return errors.Wrapf(err, "[run] failed to initialize Firebase app")
			}
			fbAuthClient, err := fbApp.Auth(ctx)
			if err != nil {
				return errors.Wrapf(err, "[run] failed to initialize Firebase Auth client")
			}

			// inject the Firebase Auth client into the application context
			// so all sub commands have access to it.
			app.SetFirebaseAuthClient(fbAuthClient)

			return nil
		},
	}
	root.AddCommand(cli.NewCmdUsers())

	ctx := context.WithValue(context.Background(), cli.AppKey("app"), app)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	return nil
}
