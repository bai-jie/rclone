package s3

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rclone/rclone/fs"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/hash"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
)

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	Auth:           libhttp.DefaultAuthCfg(),
	HTTP:           libhttp.DefaultCfg(),

	pathBucketMode: true,
	hashName:       "MD5",
	hashType:       hash.MD5,

	noCleanup: false,
}

// Opt is options set by command line flags
var Opt = DefaultOpt

func init() {
	flagSet := Command.Flags()
	libhttp.AddAuthFlagsPrefix(flagSet, "", &Opt.Auth)
	libhttp.AddHTTPFlagsPrefix(flagSet, "", &Opt.HTTP)
	vfsflags.AddFlags(flagSet)
	flags.BoolVarP(flagSet, &Opt.pathBucketMode, "force-path-style", "", Opt.pathBucketMode, "If true use path style access if false use virtual hosted style (default true)")
	flags.StringVarP(flagSet, &Opt.hashName, "etag-hash", "", Opt.hashName, "Which hash to use for the ETag, or auto or blank for off")
	flags.StringArrayVarP(flagSet, &Opt.authPair, "s3-authkey", "", Opt.authPair, "Set key pair for v4 authorization, split by comma")
	flags.BoolVarP(flagSet, &Opt.noCleanup, "no-cleanup", "", Opt.noCleanup, "Not to cleanup empty folder after object is deleted")
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "s3 remote:path",
	Short: `Serve remote:path over s3.`,
	Long:  strings.ReplaceAll(longHelp, "|", "`") + libhttp.Help + libhttp.AuthHelp + vfs.Help,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)

		if Opt.hashName == "auto" {
			Opt.hashType = f.Hashes().GetOne()
		} else if Opt.hashName != "" {
			err := Opt.hashType.Set(Opt.hashName)
			if err != nil {
				return err
			}
		}
		cmd.Run(false, false, command, func() error {
			ctx := context.Background()
			s := newServer(ctx, f, &Opt)

			sc, err := run(ctx, Opt)
			if err != nil {
				return err
			}

			router := sc.server.Router()
			s.Bind(router)
			sc.server.Wait()
			return nil
		})
		return nil
	},
}

// server contains everything to run the server
type serveCmd struct {
	server *libhttp.Server
}

func run(ctx context.Context, opt Options) (*serveCmd, error) {
	var err error

	s := &serveCmd{
	}

	s.server, err = libhttp.NewServer(ctx,
		libhttp.WithConfig(opt.HTTP),
		libhttp.WithAuth(opt.Auth),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init server: %w", err)
	}

	router := s.server.Router()
	router.Use(
		middleware.SetHeader("Server", "rclone/"+fs.Version),
	)

	s.server.Serve()

	return s, nil
}
