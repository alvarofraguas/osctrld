package main

import (
	"log"
	"log/slog"
	"os"
	"runtime"

	"github.com/urfave/cli/v2"
	_ "github.com/rs/zerolog"
)

const (
	// Application name
	appName = "osctrld"
	// Application version
	appVersion = OsctrldVersion
	// Application usage
	appUsage = "Daemon for osctrl, the fast and efficient osquery management"
	// Application description
	appDescription = appUsage + ", to manage secret, flags and osquery deployment"
)

const (
	// Default secret file
	defSecretFile = "osquery.secret"
	// Default flag file
	defFlagFile = "osquery.flags"
	// Default certificate
	defCertificate = "osctrl.crt"
	// Default enroll script
	defEnrollScript = appName + "-enroll"
	// Default remove script
	defRemoveScript = appName + "-remove"
	// Script extension for linux/darwin
	shExtension = ".sh"
	// Script extension for windows
	ps1Extension = ".ps1"
	// Default empty value
	defEmptyValue = ""
	// Default osquery path for darwin
	defDarwinPath = "/private/var/osquery/"
	// Default osquery path for linux
	defLinuxPath = "/etc/osquery/"
	// Default osquery path for windows
	defWindowsPath = "C:\\Program Files\\osquery\\"
)

const (
	// DarwinOS value for GOOS
	DarwinOS = "darwin"
	// LinuxOS value for GOOS
	LinuxOS = "linux"
	// WindowsOS value for GOOS
	WindowsOS = "windows"
)

// Global variables
var (
	err      error
	app      *cli.App
	flags    []cli.Flag
	commands []*cli.Command
)

// Variables for flags
var (
	configFile string
	jsonConfig JSONConfiguration
	osctrlURLs OsctrlURLs
)

// Initialization code
func init() {
	// Initialize CLI flags
	flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "configuration",
			Aliases:     []string{"c", "conf", "config"},
			Value:       defEmptyValue,
			Usage:       "Configuration file for osctrld to load all necessary values",
			EnvVars:     []string{"OSCTRL_CONFIG"},
			Destination: &configFile,
		},
		&cli.StringFlag{
			Name:        "secret",
			Aliases:     []string{"s"},
			Value:       defEmptyValue,
			Usage:       "Enroll secret to authenticate against osctrl server",
			EnvVars:     []string{"OSCTRL_SECRET"},
			Destination: &jsonConfig.Secret,
		},
		&cli.StringFlag{
			Name:        "environment",
			Aliases:     []string{"e", "env"},
			Value:       defEmptyValue,
			Usage:       "Environment in osctrl to enrolled nodes to",
			EnvVars:     []string{"OSCTRL_ENV"},
			Destination: &jsonConfig.Environment,
		},
		&cli.StringFlag{
			Name:        "secret-file",
			Aliases:     []string{"S"},
			Value:       defEmptyValue,
			Usage:       "Use `FILE` as secret file for osquery. Default depends on OS",
			EnvVars:     []string{"OSQUERY_SECRET"},
			Destination: &jsonConfig.SecretFile,
		},
		&cli.StringFlag{
			Name:        "flagfile",
			Aliases:     []string{"F"},
			Value:       defEmptyValue,
			Usage:       "Use `FILE` as flagfile for osquery. Default depends on OS",
			EnvVars:     []string{"OSQUERY_FLAGFILE"},
			Destination: &jsonConfig.FlagFile,
		},
		&cli.StringFlag{
			Name:        "certificate",
			Aliases:     []string{"C"},
			Value:       defEmptyValue,
			Usage:       "Use `FILE` as certificate for osquery, if needed. Default depends on OS",
			EnvVars:     []string{"OSQUERY_CERTIFICATE"},
			Destination: &jsonConfig.CertFile,
		},
		&cli.StringFlag{
			Name:        "osctrl-url",
			Aliases:     []string{"U"},
			Value:       defEmptyValue,
			Usage:       "Base URL for the osctrl server",
			EnvVars:     []string{"OSCTRL_URL"},
			Destination: &jsonConfig.BaseURL,
		},
		&cli.StringFlag{
			Name:        "osquery-path",
			Aliases:     []string{"osquery", "o"},
			Value:       defEmptyValue,
			Usage:       "Use `FILE` as path for osquery installation, if needed. Default depends on OS",
			EnvVars:     []string{"OSQUERY_PATH"},
			Destination: &jsonConfig.OsqueryPath,
		},
		&cli.BoolFlag{
			Name:        "insecure",
			Aliases:     []string{"i"},
			Value:       false,
			Usage:       "Ignore TLS warnings, often used with self-signed certificates",
			EnvVars:     []string{"OSCTRL_INSECURE"},
			Destination: &jsonConfig.Insecure,
		},
		&cli.BoolFlag{
			Name:        "verbose",
			Aliases:     []string{"V"},
			Value:       false,
			Usage:       "Enable verbose informational messages",
			EnvVars:     []string{"OSCTRL_VERBOSE"},
			Destination: &jsonConfig.Verbose,
		},
		&cli.BoolFlag{
			Name:        "force",
			Aliases:     []string{"f"},
			Value:       false,
			Usage:       "Overwrite existing files for flags, certificate and secret",
			EnvVars:     []string{"OSCTRL_FORCE"},
			Destination: &jsonConfig.Force,
		},
	}
	// Initialize CLI flags commands
	commands = []*cli.Command{
		{
			Name:   "enroll",
			Usage:  "Enroll a new node in osctrl, using new secret and flag files",
			Action: cliWrapper(enrollNode),
		},
		{
			Name:   "remove",
			Usage:  "Remove enrolled node from osctrl, clearing secret and flag files",
			Action: cliWrapper(removeNode),
		},
		{
			Name:   "verify",
			Usage:  "Verify flags, cert and secret for an enrolled node in osctrl",
			Action: cliWrapper(verifyNode),
		},
		{
			Name:   "flags",
			Usage:  "Retrieve flags for osquery from osctrl and write them locally",
			Action: cliWrapper(getFlags),
		},
		{
			Name:   "cert",
			Usage:  "Retrieve server certificate for osquery from osctrl and write it locally",
			Action: cliWrapper(getCert),
		},
	}
}

// Function to wrap actions
func cliWrapper(action func(*cli.Context) error) func(*cli.Context) error {
	return func(c *cli.Context) error {
		if configFile != defEmptyValue {
			jsonConfig, err = loadConfiguration(configFile, c.Bool("verbose"))
			if err != nil {
				slog.Error("error reading configuration file", "path", configFile, "error", err)
				return cli.Exit("", 2)
			}
		}
		logLevel := slog.LevelInfo
		if jsonConfig.Verbose {
			logLevel = slog.LevelDebug
		}
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		}))
		slog.SetDefault(logger)
		slog.Debug("initializing", "app", appName)
		// Based on OS, assign values for flag and secret file, if they have not been assigned already
		switch runtime.GOOS {
		case DarwinOS:
			if jsonConfig.OsqueryPath == defEmptyValue {
				jsonConfig.OsqueryPath = defDarwinPath
			}
			if jsonConfig.FlagFile == defEmptyValue {
				jsonConfig.FlagFile = genFullPath(jsonConfig.OsqueryPath, defFlagFile)
			}
			if jsonConfig.SecretFile == defEmptyValue {
				jsonConfig.SecretFile = genFullPath(jsonConfig.OsqueryPath, defSecretFile)
			}
			if jsonConfig.CertFile == defEmptyValue {
				jsonConfig.CertFile = genFullPath(jsonConfig.OsqueryPath, defCertificate)
			}
			if jsonConfig.EnrollScript == "" {
				jsonConfig.EnrollScript = genFullPath(jsonConfig.OsqueryPath, defEnrollScript+shExtension)
			}
			if jsonConfig.RemoveScript == "" {
				jsonConfig.RemoveScript = genFullPath(jsonConfig.OsqueryPath, defRemoveScript+shExtension)
			}
		case LinuxOS:
			if jsonConfig.OsqueryPath == defEmptyValue {
				jsonConfig.OsqueryPath = defLinuxPath
			}
			if jsonConfig.FlagFile == defEmptyValue {
				jsonConfig.FlagFile = genFullPath(jsonConfig.OsqueryPath, defFlagFile)
			}
			if jsonConfig.SecretFile == defEmptyValue {
				jsonConfig.SecretFile = genFullPath(jsonConfig.OsqueryPath, defSecretFile)
			}
			if jsonConfig.CertFile == defEmptyValue {
				jsonConfig.CertFile = genFullPath(jsonConfig.OsqueryPath, defCertificate)
			}
			if jsonConfig.EnrollScript == "" {
				jsonConfig.EnrollScript = genFullPath(jsonConfig.OsqueryPath, defEnrollScript+shExtension)
			}
			if jsonConfig.RemoveScript == "" {
				jsonConfig.RemoveScript = genFullPath(jsonConfig.OsqueryPath, defRemoveScript+shExtension)
			}
		case WindowsOS:
			if jsonConfig.OsqueryPath == defEmptyValue {
				jsonConfig.OsqueryPath = defWindowsPath
			}
			if jsonConfig.FlagFile == defEmptyValue {
				jsonConfig.FlagFile = genFullPath(jsonConfig.OsqueryPath, defFlagFile)
			}
			if jsonConfig.SecretFile == defEmptyValue {
				jsonConfig.SecretFile = genFullPath(jsonConfig.OsqueryPath, defSecretFile)
			}
			if jsonConfig.CertFile == defEmptyValue {
				jsonConfig.CertFile = genFullPath(jsonConfig.OsqueryPath, defCertificate)
			}
			if jsonConfig.EnrollScript == "" {
				jsonConfig.EnrollScript = genFullPath(jsonConfig.OsqueryPath, defEnrollScript+ps1Extension)
			}
			if jsonConfig.RemoveScript == "" {
				jsonConfig.RemoveScript = genFullPath(jsonConfig.OsqueryPath, defRemoveScript+ps1Extension)
			}
		}
		// Check for required parameters
		if jsonConfig.Environment == defEmptyValue {
			slog.Error("environment for osctrl is required")
			return cli.Exit("", 2)
		}
		if jsonConfig.BaseURL == defEmptyValue {
			slog.Error("base URL for osctrl is required")
			return cli.Exit("", 2)
		}
		// Initialize URLs
		osctrlURLs = genURLs(jsonConfig.BaseURL, jsonConfig.Environment, jsonConfig.Insecure)
		slog.Debug("configuration loaded",
			"osquery_path", jsonConfig.OsqueryPath,
			"flag_file", jsonConfig.FlagFile,
			"secret_file", jsonConfig.SecretFile,
			"cert_file", jsonConfig.CertFile,
			"enroll_script", jsonConfig.EnrollScript,
			"remove_script", jsonConfig.RemoveScript,
			"base_url", jsonConfig.BaseURL,
			"environment", jsonConfig.Environment,
			"insecure", jsonConfig.Insecure,
			"verbose", jsonConfig.Verbose,
			"force", jsonConfig.Force,
			"command", c.Command.Name,
		)
		return action(c)
	}
}

// Action to run when no flags are provided
func cliAction(c *cli.Context) error {
	if c.NumFlags() == 0 {
		if err := cli.ShowAppHelp(c); err != nil {
			log.Fatalf("Error with help - %s", err)
		}
		slog.Error("no command provided")
		return cli.Exit("", 2)
	}
	if c.Command.Name == "" {
		if err := cli.ShowAppHelp(c); err != nil {
			log.Fatalf("Error with help - %s", err)
		}
		slog.Error("invalid command")
		return cli.Exit("", 2)
	}
	return nil
}

// buildApp creates and configures the CLI application
func buildApp() *cli.App {
	a := cli.NewApp()
	a.Name = appName
	a.Usage = appUsage
	a.Version = appVersion
	a.Description = appDescription
	a.Flags = flags
	a.Commands = commands
	a.Action = cliAction
	return a
}

// Go go!
func main() {
	// Let's go!
	app = buildApp()
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("Failed to execute %v", err)
	}
}
