package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/deposist/s-ui-rus-inst/config"
	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/importxui"
	xfile "github.com/deposist/s-ui-rus-inst/database/importxui/source/file"
	xssh "github.com/deposist/s-ui-rus-inst/database/importxui/source/ssh"
	"github.com/deposist/s-ui-rus-inst/database/importxui/source/xuihttp"
)

func runImportXui(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("import-xui", flag.ContinueOnError)
	fs.SetOutput(out)
	var src string
	var remote string
	var dryRun bool
	var strategy string
	var reportPath string
	var yes bool
	var sshKey string
	var sshFingerprint string
	var acceptHostKey bool
	var xuiUser string
	var xuiPass string
	var schedule string
	var profile string
	var includeHistory bool
	var includeRouting bool
	fs.StringVar(&src, "src", "", "path to x-ui.db")
	fs.StringVar(&remote, "remote", "", "remote source URL: ssh://user@host:/path/x-ui.db or http(s)://host:port")
	fs.BoolVar(&dryRun, "dry-run", false, "preview import without committing changes")
	fs.StringVar(&strategy, "strategy", string(importxui.StrategyMerge), "conflict strategy: merge, replace or skip")
	fs.StringVar(&reportPath, "report", "", "write JSON report to path")
	fs.BoolVar(&yes, "yes", false, "confirm non-dry-run import")
	fs.StringVar(&sshKey, "ssh-key", "", "SSH private key path for --remote ssh://")
	fs.StringVar(&sshFingerprint, "ssh-fingerprint", "", "expected SSH host key SHA256 fingerprint")
	fs.BoolVar(&acceptHostKey, "accept-host-key", false, "accept and store the first SSH host key")
	fs.StringVar(&xuiUser, "xui-user", "", "3x-ui HTTP username")
	fs.StringVar(&xuiPass, "xui-pass", "", "3x-ui HTTP password")
	fs.StringVar(&schedule, "schedule", "", "save a sync profile with this cron expression instead of importing now")
	fs.StringVar(&profile, "profile", "", "sync profile name for --schedule")
	fs.BoolVar(&includeHistory, "include-history", false, "import aggregated historical traffic")
	fs.BoolVar(&includeRouting, "include-routing", false, "import Xray routing rules best-effort")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(src) == "" && strings.TrimSpace(remote) == "" {
		fmt.Fprintln(out, "import-xui: --src or --remote is required")
		return 2
	}
	if strings.TrimSpace(src) != "" && strings.TrimSpace(remote) != "" {
		fmt.Fprintln(out, "import-xui: use only one of --src or --remote")
		return 2
	}
	importStrategy := importxui.Strategy(strategy)
	if err := importStrategy.Validate(); err != nil {
		fmt.Fprintln(out, "import-xui:", err)
		return 2
	}
	if !dryRun && !yes {
		fmt.Fprint(out, "This will import into the active s-ui database. Type 'yes' to continue: ")
		var answer string
		if _, err := fmt.Fscan(os.Stdin, &answer); err != nil || answer != "yes" {
			fmt.Fprintln(out, "import-xui: cancelled")
			return 1
		}
	}
	if err := database.InitDB(config.GetDBPath()); err != nil {
		fmt.Fprintln(out, "import-xui:", err)
		return 1
	}
	source, profileSource, err := importXUISourceFromFlags(src, remote, sshKey, sshFingerprint, acceptHostKey, xuiUser, xuiPass)
	if err != nil {
		fmt.Fprintln(out, "import-xui:", err)
		return 2
	}
	if strings.TrimSpace(schedule) != "" {
		if strings.TrimSpace(profile) == "" {
			profile = defaultXUISyncProfileName(profileSource)
		}
		saved, err := importxui.SaveSyncProfile(importxui.SyncProfileInput{
			Name:       profile,
			SourceType: profileSource.Type,
			Source:     profileSource,
			Strategy:   importStrategy,
			OnlyNew:    true,
			Enabled:    true,
			Schedule:   schedule,
		})
		if err != nil {
			fmt.Fprintln(out, "import-xui:", err)
			return 1
		}
		fmt.Fprintf(out, "Saved x-ui sync profile %q (id %d)\n", saved.Name, saved.Id)
		return 0
	}
	if !dryRun {
		backupPath, err := importxui.WritePreImportBackup(time.Now().Unix())
		if err != nil {
			fmt.Fprintln(out, "import-xui:", err)
			return 1
		}
		fmt.Fprintln(out, "Pre-import backup:", backupPath)
	}
	opts := importxui.Options{
		DryRun:         dryRun,
		Strategy:       importStrategy,
		IncludeHistory: includeHistory,
		IncludeRouting: includeRouting,
	}
	var report *importxui.Report
	if source != nil {
		report, err = importxui.ImportFromSource(source, opts)
	} else {
		report, err = importxui.Import(src, opts)
	}
	if err != nil {
		fmt.Fprintln(out, "import-xui:", err)
		return 1
	}
	printImportXuiSummary(out, report)
	if reportPath != "" {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(out, "import-xui:", err)
			return 1
		}
		if err := os.WriteFile(reportPath, raw, 0o600); err != nil {
			fmt.Fprintln(out, "import-xui:", err)
			return 1
		}
	}
	return 0
}

func importXUISourceFromFlags(src, remote, sshKey, sshFingerprint string, acceptHostKey bool, xuiUser, xuiPass string) (importxui.Source, importxui.SyncProfileSource, error) {
	src = strings.TrimSpace(src)
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return nil, importxui.SyncProfileSource{Type: "file", URL: src}, nil
	}
	if strings.HasPrefix(remote, "ssh://") {
		parsed, err := xssh.New(remote)
		if err != nil {
			return nil, importxui.SyncProfileSource{}, err
		}
		parsed.KeyPath = strings.TrimSpace(sshKey)
		parsed.HostKeyFingerprint = strings.TrimSpace(sshFingerprint)
		parsed.ConfirmHostKey = acceptHostKey
		if xuiUser != "" {
			parsed.User = xuiUser
		}
		if xuiPass != "" {
			parsed.Password = xuiPass
		}
		cfg := importxui.SyncProfileSource{
			Type:               "ssh",
			URL:                remote,
			Username:           parsed.User,
			Password:           parsed.Password,
			KeyPath:            parsed.KeyPath,
			RemotePath:         parsed.RemotePath,
			ConfirmHostKey:     parsed.ConfirmHostKey,
			HostKeyFingerprint: parsed.HostKeyFingerprint,
		}
		return parsed, cfg, nil
	}
	if strings.HasPrefix(remote, "http://") || strings.HasPrefix(remote, "https://") {
		cfg := importxui.SyncProfileSource{
			Type:     "xuihttp",
			URL:      remote,
			BaseURL:  remote,
			Username: xuiUser,
			Password: xuiPass,
		}
		return xuihttp.New(remote, xuiUser, xuiPass), cfg, nil
	}
	return xfile.New(remote), importxui.SyncProfileSource{Type: "file", URL: remote}, nil
}

func defaultXUISyncProfileName(source importxui.SyncProfileSource) string {
	if source.URL != "" {
		return source.Type + "-" + strconv.FormatInt(time.Now().Unix(), 10)
	}
	if source.Host != "" {
		return source.Type + "-" + strings.ReplaceAll(source.Host, ".", "-")
	}
	return "xui-sync-" + strconv.FormatInt(time.Now().Unix(), 10)
}

func printImportXuiSummary(out io.Writer, report *importxui.Report) {
	fmt.Fprintf(out, "Import summary:\n")
	fmt.Fprintf(out, "  Inbounds: %d/%d imported, %d skipped, %d conflicts\n",
		report.Summary.Inbounds.Imported,
		report.Summary.Inbounds.Total,
		report.Summary.Inbounds.Skipped,
		report.Summary.Inbounds.Conflicts,
	)
	fmt.Fprintf(out, "  Endpoints: %d imported\n", report.Summary.Endpoints.Imported)
	fmt.Fprintf(out, "  TLS: %d created, %d reused\n", report.Summary.TLS.Created, report.Summary.TLS.Reused)
	fmt.Fprintf(out, "  Clients: %d unique, %d created, %d merged\n",
		report.Summary.Clients.UniqueEmails,
		report.Summary.Clients.Created,
		report.Summary.Clients.Merged,
	)
	if len(report.Warnings) > 0 {
		fmt.Fprintf(out, "  Warnings: %d\n", len(report.Warnings))
	}
}
