package main

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/urfave/cli/v2"
)

var (
	// OsqueryDarwin default installation
	OsqueryDarwin = []string{
		"/private/var/osquery/io.osquery.agent.plist",
		"/opt/osquery/lib/osquery.app/Contents/MacOS/osqueryd",
	}
	// OsqueryLinux default installation
	OsqueryLinux = []string{
		"/usr/lib/systemd/system/osqueryd.service",
		"/opt/osquery/bin/osqueryd",
	}
	// OsqueryWindows default installation
	OsqueryWindows = []string{
		"C:\\Program Files\\osquery\\manage-osqueryd.ps1",
		"C:\\Program Files\\osquery\\osqueryd\\osqueryd.exe",
	}
	// FlagTLSServerCerts for TLS server certificates
	FlagTLSServerCerts = "--tls_server_certs"
	// FlagOsqueryVersion to get osquery version
	FlagOsqueryVersion = "-version"
)

// FlagsRequest to retrieve flags
type FlagsRequest struct {
	Secret     string `json:"secret"`
	SecretFile string `json:"secretFile"`
	CertFile   string `json:"certFile"`
}

// CertRequest to retrieve certificate
type CertRequest struct {
	Secret string `json:"secret"`
}

// ScriptRequest to retrieve script
type ScriptRequest CertRequest

// VerifyRequest to verify node
type VerifyRequest FlagsRequest

// VerifyResponse for verify requests from osctrld
type VerifyResponse struct {
	Flags          string `json:"flags"`
	Certificate    string `json:"certificate"`
	OsqueryVersion string `json:"osquery_version"`
}

// Function to action on enroll command
func enrollNode(c *cli.Context) error {
	slog.Debug("enrolling node", "url", osctrlURLs.Enroll)
	script, err := retrieveScript(jsonConfig.Secret, osctrlURLs.Enroll, jsonConfig.Insecure)
	if err != nil {
		return fmt.Errorf("error retrieving enroll - %v", err)
	}
	fmt.Printf("%s", script)
	return nil
}

// Function to action on flags command
func getFlags(c *cli.Context) error {
	slog.Debug("getting flags", "url", osctrlURLs.Flags)
	flags, err := retrieveFlags(jsonConfig.Secret, jsonConfig.SecretFile, jsonConfig.CertFile)
	if err != nil {
		return fmt.Errorf("error retrieving flags - %v", err)
	}
	slog.Debug("flags content", "flags", flags)
	if err := writeContentExists(jsonConfig.FlagFile, flags, "flags", jsonConfig.Force); err != nil {
		return err
	}
	slog.Info("flags ready", "path", jsonConfig.FlagFile)
	return nil
}

// Function to action on cert command
func getCert(c *cli.Context) error {
	slog.Debug("getting cert", "url", osctrlURLs.Cert)
	cert, err := retrieveCert(jsonConfig.Secret, osctrlURLs.Cert, jsonConfig.Insecure)
	if err != nil {
		return fmt.Errorf("error retrieving cert - %v", err)
	}
	slog.Debug("cert content", "cert", cert)
	if err := writeContentExists(jsonConfig.CertFile, cert, "cert", jsonConfig.Force); err != nil {
		return err
	}
	slog.Info("cert ready", "path", jsonConfig.CertFile)
	return nil
}

// Function to action on remove command. It retrieves the script to run the removal from osctrl
func removeNode(c *cli.Context) error {
	slog.Debug("removing node", "url", osctrlURLs.Remove)
	script, err := retrieveScript(jsonConfig.Secret, osctrlURLs.Remove, jsonConfig.Insecure)
	if err != nil {
		return fmt.Errorf("error retrieving remove - %v", err)
	}

	fmt.Printf("%s", script)
	return nil
}

// Function to action on verify command. It verifies flags, cert and secret for and enrolled node in osctrl
func verifyNode(c *cli.Context) error {
	// Compare secret with local
	slog.Debug("comparing secret", "path", jsonConfig.SecretFile)
	if checkFileContent(jsonConfig.SecretFile, jsonConfig.Secret) {
		slog.Info("osquery secret is valid")
	} else {
		slog.Warn("osquery secret mismatch")
	}
	// Retrieve verification
	slog.Debug("retrieving verification", "url", osctrlURLs.Verify)
	verification, err := retrieveVerify(jsonConfig.Secret, jsonConfig.SecretFile, jsonConfig.CertFile, osctrlURLs.Verify, jsonConfig.Insecure)
	if err != nil {
		return fmt.Errorf("error retrieving verification - %v", err)
	}
	// Compare flags with local
	slog.Debug("comparing flags", "path", jsonConfig.FlagFile)
	if checkFileContent(jsonConfig.FlagFile, strings.TrimSpace(verification.Flags)) {
		slog.Info("flags are valid")
	} else {
		slog.Warn("flags mismatch")
	}
	// Retrieve certificate if flag is present
	if strings.Contains(verification.Flags, FlagTLSServerCerts) {
		// Compare certificate with local
		slog.Debug("comparing certificate", "path", jsonConfig.CertFile)
		if checkFileContent(jsonConfig.CertFile, strings.TrimSpace(verification.Certificate)) {
			slog.Info("osquery certificate is valid")
		} else {
			slog.Warn("osquery certificate mismatch")
		}
	}
	// Check local files
	var localFiles []string
	switch runtime.GOOS {
	case DarwinOS:
		localFiles = OsqueryDarwin
	case LinuxOS:
		localFiles = OsqueryLinux
	case WindowsOS:
		localFiles = OsqueryWindows
	}
	validLocal := true
	for _, l := range localFiles {
		slog.Debug("checking local file", "path", l)
		if !checkFileExist(l) {
			slog.Warn("local file missing", "path", l)
			validLocal = false
		}
	}
	if validLocal {
		slog.Info("osquery local files are present")
		// osquery version check
		slog.Debug("expected osquery version", "version", verification.OsqueryVersion)
		existingVersion := getOsqueryVersion()
		slog.Debug("existing osquery version", "version", existingVersion)
		if osqueryVersionCompare(existingVersion, verification.OsqueryVersion) > 1 {
			slog.Warn("osquery version too low", "existing", existingVersion, "required", verification.OsqueryVersion)
		} else {
			slog.Info("osquery version is valid", "version", existingVersion)
		}
		// Check if osquery is running
		slog.Debug("checking running process")
		ps, err := process.Processes()
		if err != nil {
			return fmt.Errorf("error getting processes - %s", err)
		}
		osqueryRunning := false
		var osqueryPid int32
		for _, p := range ps {
			pCmd, _ := p.Cmdline()
			if strings.Contains(pCmd, "/osqueryd ") {
				osqueryRunning = true
				osqueryPid = p.Pid
				break
			}
		}
		if osqueryRunning {
			slog.Info("osqueryd is running", "pid", osqueryPid)
		} else {
			slog.Warn("osqueryd is not running")
		}
	} else {
		slog.Error("osquery is not installed")
	}
	return nil
}
