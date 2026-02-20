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

func main() {
	// Flags
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

	args := flag.Args()

	// Determine command and environment
	command := "deploy"
	envName := "local"

	if len(args) >= 1 {
		switch args[0] {
		case "list", "rollback", "status", "manifest":
			command = args[0]
			if len(args) >= 2 {
				envName = args[1]
			}
		default:
			envName = args[0]
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
		if len(args) >= 3 {
			targetRelease = args[2]
		}
		cmdErr = deploy.Rollback(*env, targetRelease)

	case "status":
		cmdErr = deploy.Status(*env)

	case "manifest":
		buildDir := "public"
		if len(args) >= 2 {
			buildDir = args[1]
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

func availableEnvs(config *deploy.Config) string {
	var names []string
	for _, env := range config.Environments {
		names = append(names, env.Name)
	}
	return strings.Join(names, ", ")
}
