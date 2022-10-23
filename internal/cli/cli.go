package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"text/tabwriter"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/andyfusniak/fbauthtool/internal/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/api/iterator"
)

// AppKey context key.
type AppKey string

// App cli tool.
type App struct {
	version      string
	gitCommit    string
	cfg          *config.Config
	fbAuthClient *auth.Client
	stdOut       io.Writer
	stdErr       io.Writer
}

// Option lets you configure the HTTP Client.
type Option func(*App)

// NewApp creates a new CLI application.
func NewApp(options ...Option) *App {
	a := &App{
		stdOut: os.Stdout,
		stdErr: os.Stderr,
	}
	for _, o := range options {
		o(a)
	}

	return a
}

// WithVersion option to set the cli version.
func WithVersion(s string) Option {
	return func(a *App) {
		a.version = s
	}
}

// WithGitCommit option to set the git commit hash.
func WithGitCommit(s string) Option {
	return func(a *App) {
		a.gitCommit = s
	}
}

// WithConfig option to set the config.
func WithConfig(cfg *config.Config) Option {
	return func(a *App) {
		a.cfg = cfg
	}
}

// WithStdOut option to set default output stream.
func WithStdOut(w io.Writer) Option {
	return func(a *App) {
		a.stdOut = w
	}
}

// WithStdErr option to set default error stream.
func WithStdErr(w io.Writer) Option {
	return func(a *App) {
		a.stdErr = w
	}
}

// SetFirebaseAuthClient sets the Firebase Auth client.
func (a *App) SetFirebaseAuthClient(c *auth.Client) {
	a.fbAuthClient = c
}

// FirebaseAuthClient returns the Firebase Auth client or nil if not set.
func (a *App) FirebaseAuthClient() *auth.Client {
	return a.fbAuthClient
}

// Config returns the application configuration.
func (a *App) Config() *config.Config {
	return a.cfg
}

// NewCmdUsers users sub command.
func NewCmdUsers() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage users",
	}
	cmd.AddCommand(NewCmdUsersGetOne())
	cmd.AddCommand(NewCmdUsersList())
	cmd.AddCommand(NewCmdUsersSetClaims())
	return cmd
}

// NewCmdUsersSetClaims set the user claims sub command.
func NewCmdUsersSetClaims() *cobra.Command {
	return &cobra.Command{
		Use:     "set-claims UID JSON",
		Short:   "set a user claims",
		Aliases: []string{"setclaims"},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("accepts UID JSON arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			app := ctx.Value(AppKey("app")).(*App)

			uid := args[0]
			fbAuthClient := app.FirebaseAuthClient()
			if !isValidFirebaseUID(uid) {
				fmt.Fprintf(app.stdErr, fmt.Sprintf("User with uid %s not found.\n", uid))
				os.Exit(1)
			}

			var customClaims map[string]interface{}
			jsonArg := args[1]
			if err := json.Unmarshal([]byte(jsonArg), &customClaims); err != nil {
				fmt.Fprintf(app.stdErr, "Failed to parse the JSON string")
				os.Exit(1)
			}

			if err := fbAuthClient.SetCustomUserClaims(ctx, uid, customClaims); err != nil {
				fmt.Fprintf(os.Stderr,
					fmt.Sprintf("Failed to set custom claims for user %s", uid))
				os.Exit(1)
			}

			userRecord, err := fbAuthClient.GetUser(ctx, uid)
			if err != nil {
				return err
			}
			displayUser(app.stdOut, userRecord)

			return nil
		},
	}
}

// NewCmdUsersGetOne get user sub command.
func NewCmdUsersGetOne() *cobra.Command {
	return &cobra.Command{
		Use:     "get UID|EMAIL",
		Short:   "get a user by uid or email",
		Long:    "get user does a best guess to determine the argument",
		Aliases: []string{"user"},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("missing UID|EMAIL argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			app := ctx.Value(AppKey("app")).(*App)

			uidOrEmail := args[0]
			fbAuthClient := app.FirebaseAuthClient()
			if isValidFirebaseUID(uidOrEmail) {
				userRecord, err := fbAuthClient.GetUser(ctx, uidOrEmail)
				if err != nil {
					return err
				}
				if err := displayUser(app.stdOut, userRecord); err != nil {
					return err
				}
			}

			// assume it's an email
			userRecord, err := fbAuthClient.GetUserByEmail(ctx, uidOrEmail)
			if err != nil {
				return err
			}
			if err := displayUser(app.stdOut, userRecord); err != nil {
				return err
			}

			return nil
		},
	}
}

var uidRegExp = regexp.MustCompile(`^[A-Za-z0123456789]{28}$`)

func isValidFirebaseUID(s string) bool {
	return uidRegExp.MatchString(s)
}

func displayUser(out io.Writer, u *auth.UserRecord) error {
	fmt.Fprintf(out, "UID: %s\n", u.UID)
	fmt.Fprintf(out, "Email: %s\n", u.Email)
	fmt.Fprintf(out, "Display name: %s\n", u.DisplayName)
	if u.EmailVerified {
		fmt.Println("Email verified: Yes")
	} else {
		fmt.Println("Email verified: No")
	}

	b, err := json.Marshal(u.CustomClaims)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "CustomClaims: %s\n", b)
	t := time.Unix(u.UserMetadata.CreationTimestamp/1000, 0)
	fmt.Printf("Created: %s\n", t.Format(time.RFC1123))

	return nil
}

// NewCmdUsersList list users sub command.
func NewCmdUsersList() *cobra.Command {
	return &cobra.Command{
		Use:   "list users",
		Short: "list users for the current auth config",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			app := ctx.Value(AppKey("app")).(*App)

			fbAuthClient := app.FirebaseAuthClient()

			pager := iterator.NewPager(fbAuthClient.Users(ctx, ""), 100, "")
			format := "%v\t\t%v\t\t%v\t\t\n"
			tw := new(tabwriter.Writer).Init(app.stdOut, 0, 8, 2, ' ', 0)
			fmt.Fprintf(tw, format, "UID", "Email", "DISPLAY NAME")

			for {
				var users []*auth.ExportedUserRecord
				nextPageToken, err := pager.NextPage(&users)
				if err != nil {
					return errors.Wrapf(err, "[run] paging error %v\n")
				}

				for _, u := range users {
					if u.Email != "" {
						fmt.Fprintf(tw, format, u.UID, u.Email, u.DisplayName)
					}
				}
				tw.Flush()

				if nextPageToken == "" {
					break
				}
			}
			return nil
		},
	}
}
