package deploycmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/JuniperBible/Public.Tool.Server.JuniperBible/internal/deploy"
)

// deployFlags holds parsed flags for deploy command
type deployFlags struct {
	configPath string
	releaseID  string
	dryRun     bool
	full       bool
	noBuild    bool
}

// parseDeployFlags parses flags and returns command, environment, remaining args, and flags
func parseDeployFlags(args []string) (command, envName string, remaining []string, flags deployFlags) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)

	configPath := fs.String("config", "deploy.toml", "Path to configuration file")
	releaseID := fs.String("release", "", "Release ID (default: auto-generated)")
	dryRun := fs.Bool("dry-run", false, "Show what would be deployed without deploying")
	full := fs.Bool("full", false, "Upload all files instead of delta")
	noBuild := fs.Bool("no-build", false, "Skip Hugo build (use existing public/ directory)")
	help := fs.Bool("help", false, "Show help")

	fs.Usage = func() {
		printUsage()
		fs.PrintDefaults()
	}
	fs.Parse(args)

	if *help {
		fs.Usage()
		os.Exit(0)
	}

	flags = deployFlags{
		configPath: *configPath,
		releaseID:  *releaseID,
		dryRun:     *dryRun,
		full:       *full,
		noBuild:    *noBuild,
	}

	remaining = fs.Args()
	command = "deploy"
	envName = "local"

	if len(remaining) >= 1 {
		switch remaining[0] {
		case "list", "rollback", "status", "manifest":
			command = remaining[0]
			if len(remaining) >= 2 {
				envName = remaining[1]
			}
		default:
			envName = remaining[0]
		}
	}
	return
}

// loadDeployEnv loads config and returns the environment
func loadDeployEnv(configPath, envName string) *deploy.Environment {
	config, err := deploy.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nCreate a deploy.toml in your project root:\n\n%s", deploy.ExampleConfig())
		os.Exit(1)
	}

	foundEnv, ok := config.GetEnvironment(envName)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: unknown environment '%s'\n", envName)
		fmt.Fprintf(os.Stderr, "Available environments: %s\n", availableEnvs(config))
		os.Exit(1)
	}
	return &foundEnv
}

// cmdDeploy executes the deploy command
func cmdDeploy(env *deploy.Environment, flags deployFlags) error {
	opts := deploy.Options{
		ReleaseID: flags.releaseID,
		DryRun:    flags.dryRun,
		Full:      flags.full,
		NoBuild:   flags.noBuild,
	}
	return deploy.Deploy(*env, opts)
}

// cmdRollback executes the rollback command
func cmdRollback(env *deploy.Environment, remaining []string) error {
	targetRelease := ""
	if len(remaining) >= 3 {
		targetRelease = remaining[2]
	}
	return deploy.Rollback(*env, targetRelease)
}

// cmdManifest executes the manifest command
func cmdManifest(remaining []string, releaseID string) error {
	buildDir := "public"
	if len(remaining) >= 2 {
		buildDir = remaining[1]
	}
	return deploy.GenerateManifestOnly(buildDir, releaseID)
}

// commandHandler is a function that handles a deploy subcommand
type commandHandler func(*deploy.Environment, []string, deployFlags) error

// handleDeploy handles the deploy command
func handleDeploy(env *deploy.Environment, _ []string, flags deployFlags) error {
	return cmdDeploy(env, flags)
}

// handleList handles the list command
func handleList(env *deploy.Environment, _ []string, _ deployFlags) error {
	return deploy.ListReleases(*env)
}

// handleRollback handles the rollback command
func handleRollback(env *deploy.Environment, remaining []string, _ deployFlags) error {
	return cmdRollback(env, remaining)
}

// handleStatus handles the status command
func handleStatus(env *deploy.Environment, _ []string, _ deployFlags) error {
	return deploy.Status(*env)
}

// handleManifest handles the manifest command
func handleManifest(_ *deploy.Environment, remaining []string, flags deployFlags) error {
	return cmdManifest(remaining, flags.releaseID)
}

// commandHandlers maps commands to their handlers
var commandHandlers = map[string]commandHandler{
	"deploy":   handleDeploy,
	"list":     handleList,
	"rollback": handleRollback,
	"status":   handleStatus,
	"manifest": handleManifest,
}

// runDeployCommand executes the deploy subcommand
func runDeployCommand(command string, env *deploy.Environment, remaining []string, flags deployFlags) error {
	handler, ok := commandHandlers[command]
	if !ok {
		return fmt.Errorf("unknown command '%s'", command)
	}
	return handler(env, remaining, flags)
}

// Run executes the deploy subcommand with the given arguments.
func Run(args []string) {
	command, envName, remaining, flags := parseDeployFlags(args)
	env := loadDeployEnv(flags.configPath, envName)

	if err := runDeployCommand(command, env, remaining, flags); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`juniper-host deploy - Atomic deployment with delta sync

Usage:
  juniper-host deploy [flags] [environment]
  juniper-host deploy <command> [environment]

Environments:
  local       Deploy to local releases directory (default)
  prod        Deploy to production VPS via SSH

Commands:
  [env]              Deploy to environment (default)
  list [env]         List releases on target
  rollback [env]     Rollback to previous release
  status [env]       Show current deployment status
  manifest [dir]     Generate build manifest only

Flags:
`)
}

func availableEnvs(config *deploy.Config) string {
	var names []string
	for _, env := range config.Environments {
		names = append(names, env.Name)
	}
	return strings.Join(names, ", ")
}
