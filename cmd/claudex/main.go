// Command claudex exposes GPT-5.6 Codex models through the Anthropic Messages API
// expected by Claude Code.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/claudex"
	internalcmd "github.com/router-for-me/CLIProxyAPI/v7/internal/cmd"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/logging"
	_ "github.com/router-for-me/CLIProxyAPI/v7/internal/translator"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/util"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v7/sdk/auth"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func init() {
	logging.SetupBaseLogger()
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "claudex: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return runServe(nil)
	}

	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "login":
		return runLogin(args[1:])
	case "version", "--version", "-version":
		fmt.Printf("claudex %s (commit %s, built %s)\n", Version, Commit, BuildDate)
		return nil
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		if strings.HasPrefix(args[0], "-") {
			return runServe(args)
		}
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", defaultConfigPath(), "path to Claudex configuration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	cfg, resolvedPath, err := claudex.LoadServeConfig(*configPath)
	if err != nil {
		return err
	}
	if claudex.WaitForServer(cfg, 1) {
		fmt.Printf("Claudex server already running at %s; reusing it\n", claudex.ServerURL(cfg))
		return nil
	}
	if err = logging.ConfigureLogOutput(cfg); err != nil {
		return fmt.Errorf("configure logging: %w", err)
	}
	util.SetLogLevel(cfg)
	sdkAuth.RegisterTokenStore(sdkAuth.NewFileTokenStore())

	service, err := claudex.NewService(cfg, resolvedPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	fmt.Printf("Claudex %s listening on http://%s:%d (models: Luna, Sol, Terra; default effort: %s)\n", Version, cfg.Host, cfg.Port, claudex.DefaultEffort)
	if err = service.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("run service: %w", err)
	}
	return nil
}

func runLogin(args []string) error {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", defaultConfigPath(), "path to Claudex configuration")
	device := flags.Bool("device", false, "use the Codex device-code login flow")
	noBrowser := flags.Bool("no-browser", false, "do not open a browser automatically")
	callbackPort := flags.Int("oauth-callback-port", 0, "override the OAuth callback port")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	cfg, _, err := claudex.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	if err = claudex.Validate(cfg); err != nil {
		return err
	}
	sdkAuth.RegisterTokenStore(sdkAuth.NewFileTokenStore())
	options := &internalcmd.LoginOptions{
		NoBrowser:    *noBrowser,
		CallbackPort: *callbackPort,
	}
	if *device {
		internalcmd.DoCodexDeviceLogin(cfg, options)
	} else {
		internalcmd.DoCodexLogin(cfg, options)
	}
	return nil
}

func defaultConfigPath() string {
	if path := strings.TrimSpace(os.Getenv("CLAUDEX_CONFIG")); path != "" {
		return path
	}
	return "claudex.yaml"
}

func printUsage() {
	fmt.Print(`Usage:
  claudex login [options]   Authenticate a ChatGPT/Codex account
  claudex serve [options]   Start the Claude Code compatibility proxy
  claudex version           Print build information

Running claudex without a command is equivalent to claudex serve.
`)
}
