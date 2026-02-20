package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/deploy"
)

const usage = `juniper-deploy - Atomic deployment tool for Juniper Bible

Usage:
  juniper-deploy [flags] [environment]

Environments:
  local       Deploy to local releases directory (default)
  prod        Deploy to production VPS via SSH

Commands:
  juniper-deploy [env]           Deploy to environment
  juniper-deploy list [env]      List releases on target
  juniper-deploy rollback [env]  Rollback to previous release
  juniper-deploy status [env]    Show current deployment status

Flags:
`

// cliFlags holds parsed command line flags
type cliFlags struct {
	configPath string
	releaseID  string
	dryRun     bool
	full       bool
	noBuild    bool
}

// parseFlags parses and returns CLI flags
func parseFlags() cliFlags {
	configPath := flag.String("config", "deploy.toml", "Path to configuration file")
	releaseID := flag.String("release", "", "Release ID (default: auto-generated)")
	dryRun := flag.Bool("dry-run", false, "Show what would be deployed without deploying")
	full := flag.Bool("full", false, "Upload all files instead of delta")
	noBuild := flag.Bool("no-build", false, "Skip Hugo build (use existing public/ directory)")
	help := flag.Bool("help", false, "Show help")
	h := flag.Bool("h", false, "Show help")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *help || *h {
		flag.Usage()
		os.Exit(0)
	}

	return cliFlags{
		configPath: *configPath,
		releaseID:  *releaseID,
		dryRun:     *dryRun,
		full:       *full,
		noBuild:    *noBuild,
	}
}

// parseCommandAndEnv parses command and environment from args
func parseCommandAndEnv(args []string) (command, envName string) {
	command = "deploy"
	envName = "local"
	if len(args) < 1 {
		return
	}
	switch args[0] {
	case "list", "rollback", "status", "manifest":
		command = args[0]
		if len(args) >= 2 {
			envName = args[1]
		}
	default:
		envName = args[0]
	}
	return
}

// parseCommandLine parses CLI flags and returns the command, environment, and flags
func parseCommandLine() (command, envName string, args []string, flags cliFlags) {
	flags = parseFlags()
	args = flag.Args()
	command, envName = parseCommandAndEnv(args)
	return
}

// loadEnvironment loads config and finds the environment
func loadEnvironment(configPath, envName string) *deploy.Environment {
	config, err := deploy.LoadConfig(configPath)
	if err != nil {
		config = &deploy.Config{Environments: deploy.DefaultEnvironments()}
	}

	foundEnv, ok := config.GetEnvironment(envName)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: unknown environment '%s'\n", envName)
		fmt.Fprintf(os.Stderr, "Available environments: %s\n", availableEnvs(config))
		os.Exit(1)
	}
	return &foundEnv
}

// runDeploy executes the deploy command
func runDeploy(env *deploy.Environment, flags cliFlags) error {
	opts := deploy.Options{
		ReleaseID: flags.releaseID,
		DryRun:    flags.dryRun,
		Full:      flags.full,
		NoBuild:   flags.noBuild,
	}
	return deploy.Deploy(*env, opts)
}

// runRollback executes the rollback command
func runRollback(env *deploy.Environment, args []string) error {
	targetRelease := ""
	if len(args) >= 3 {
		targetRelease = args[2]
	}
	return deploy.Rollback(*env, targetRelease)
}

// runManifest executes the manifest command
func runManifest(args []string, releaseID string) error {
	buildDir := "public"
	if len(args) >= 2 {
		buildDir = args[1]
	}
	return deploy.GenerateManifestOnly(buildDir, releaseID)
}

// cmdHandler is a function type for command handlers
type cmdHandler func(*deploy.Environment, []string, cliFlags) error

// cmdDeployHandler handles the deploy command
func cmdDeployHandler(env *deploy.Environment, _ []string, flags cliFlags) error {
	return runDeploy(env, flags)
}

// cmdListHandler handles the list command
func cmdListHandler(env *deploy.Environment, _ []string, _ cliFlags) error {
	return deploy.ListReleases(*env)
}

// cmdRollbackHandler handles the rollback command
func cmdRollbackHandler(env *deploy.Environment, args []string, _ cliFlags) error {
	return runRollback(env, args)
}

// cmdStatusHandler handles the status command
func cmdStatusHandler(env *deploy.Environment, _ []string, _ cliFlags) error {
	return deploy.Status(*env)
}

// cmdManifestHandler handles the manifest command
func cmdManifestHandler(_ *deploy.Environment, args []string, flags cliFlags) error {
	return runManifest(args, flags.releaseID)
}

// cmdHandlers maps commands to handlers
var cmdHandlers = map[string]cmdHandler{
	"deploy":   cmdDeployHandler,
	"list":     cmdListHandler,
	"rollback": cmdRollbackHandler,
	"status":   cmdStatusHandler,
	"manifest": cmdManifestHandler,
}

// executeCommand runs the specified command
func executeCommand(command string, env *deploy.Environment, args []string, flags cliFlags) error {
	handler, ok := cmdHandlers[command]
	if !ok {
		return fmt.Errorf("unknown command '%s'", command)
	}
	return handler(env, args, flags)
}

func main() {
	command, envName, args, flags := parseCommandLine()
	env := loadEnvironment(flags.configPath, envName)

	if err := executeCommand(command, env, args, flags); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func availableEnvs(config *deploy.Config) string {
	var names []string
	for _, env := range config.Environments {
		names = append(names, env.Name)
	}
	return strings.Join(names, ", ")
}
