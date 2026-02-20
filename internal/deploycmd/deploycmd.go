package deploycmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/deploy"
)

// Run executes the deploy subcommand with the given arguments.
func Run(args []string) {
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

	remaining := fs.Args()

	// Determine command and environment
	command := "deploy"
	envName := "local"

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

	// Load configuration
	config, err := deploy.LoadConfig(*configPath)
	if err != nil {
		// If no config file, use defaults
		config = &deploy.Config{
			Environments: deploy.DefaultEnvironments(),
		}
	}

	// Find environment
	foundEnv, ok := config.GetEnvironment(envName)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: unknown environment '%s'\n", envName)
		fmt.Fprintf(os.Stderr, "Available environments: %s\n", availableEnvs(config))
		os.Exit(1)
	}
	env := &foundEnv

	// Execute command
	var cmdErr error
	switch command {
	case "deploy":
		opts := deploy.Options{
			ReleaseID: *releaseID,
			DryRun:    *dryRun,
			Full:      *full,
			NoBuild:   *noBuild,
		}
		cmdErr = deploy.Deploy(*env, opts)

	case "list":
		cmdErr = deploy.ListReleases(*env)

	case "rollback":
		targetRelease := ""
		if len(remaining) >= 3 {
			targetRelease = remaining[2]
		}
		cmdErr = deploy.Rollback(*env, targetRelease)

	case "status":
		cmdErr = deploy.Status(*env)

	case "manifest":
		buildDir := "public"
		if len(remaining) >= 2 {
			buildDir = remaining[1]
		}
		cmdErr = deploy.GenerateManifestOnly(buildDir, *releaseID)

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n", command)
		os.Exit(1)
	}

	if cmdErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", cmdErr)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`juniper-host deploy - Atomic deployment with delta sync

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
