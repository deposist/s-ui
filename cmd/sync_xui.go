package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/deposist/s-ui-rus-inst/config"
	"github.com/deposist/s-ui-rus-inst/cronjob"
	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/database/model"
)

func runSyncXui(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("sync-xui", flag.ContinueOnError)
	fs.SetOutput(out)
	var once bool
	var list bool
	var profile string
	fs.BoolVar(&once, "once", false, "run one sync now")
	fs.BoolVar(&list, "list-profiles", false, "list saved x-ui sync profiles")
	fs.StringVar(&profile, "profile", "", "profile name or id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := database.InitDB(config.GetDBPath()); err != nil {
		fmt.Fprintln(out, "sync-xui:", err)
		return 1
	}
	switch {
	case list:
		return listXUISyncProfiles(out)
	case once:
		return runXUISyncProfileOnce(profile, out)
	default:
		fmt.Fprintln(out, "sync-xui: use --once or --list-profiles")
		return 2
	}
}

func listXUISyncProfiles(out io.Writer) int {
	var profiles []model.XUISyncProfile
	if err := database.GetDB().Order("id asc").Find(&profiles).Error; err != nil {
		fmt.Fprintln(out, "sync-xui:", err)
		return 1
	}
	for _, profile := range profiles {
		fmt.Fprintf(out, "%d\t%s\t%s\tenabled=%t\tlast=%s\n", profile.Id, profile.Name, profile.SourceType, profile.Enabled, profile.LastRunStatus)
	}
	return 0
}

func runXUISyncProfileOnce(selector string, out io.Writer) int {
	profile, err := findXUISyncProfile(selector)
	if err != nil {
		fmt.Fprintln(out, "sync-xui:", err)
		return 1
	}
	if err := cronjob.NewXUISyncJob().RunProfile(context.Background(), profile); err != nil {
		fmt.Fprintln(out, "sync-xui:", err)
		return 1
	}
	fmt.Fprintf(out, "Synced x-ui profile %q\n", profile.Name)
	return 0
}

func findXUISyncProfile(selector string) (*model.XUISyncProfile, error) {
	selector = strings.TrimSpace(selector)
	var profile model.XUISyncProfile
	db := database.GetDB()
	if selector == "" {
		err := db.Where("enabled = ?", true).Order("id asc").First(&profile).Error
		if err != nil {
			return nil, err
		}
		return &profile, nil
	}
	if id, err := strconv.ParseUint(selector, 10, 64); err == nil && id > 0 {
		if err := db.First(&profile, id).Error; err != nil {
			return nil, err
		}
		return &profile, nil
	}
	if err := db.Where("name = ?", selector).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func runSyncXuiFromMain() {
	os.Exit(runSyncXui(os.Args[2:], os.Stdout))
}
