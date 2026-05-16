package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/deposist/s-ui-rus-inst/config"
	"github.com/deposist/s-ui-rus-inst/logger"
)

const (
	versionCheckURL     = "https://api.github.com/repos/deposist/s-ui-rus-inst/releases/latest"
	versionCheckCache   = time.Hour
	versionCheckTimeout = 3 * time.Second
)

type VersionService struct{}

type VersionInfo struct {
	Current         string `json:"current"`
	Version         string `json:"version"`
	Latest          string `json:"latest,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable,omitempty"`
	ReleaseURL      string `json:"releaseURL,omitempty"`
	CheckedAt       int64  `json:"checkedAt,omitempty"`
}

type latestRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

var versionCheckState = struct {
	sync.Mutex
	client     *http.Client
	url        string
	checkedAt  time.Time
	latest     string
	releaseURL string
}{
	client: &http.Client{Timeout: versionCheckTimeout},
	url:    versionCheckURL,
}

func (s *VersionService) GetVersionInfo() VersionInfo {
	current := config.GetVersion()
	latest, releaseURL, checkedAt := latestReleaseCached()
	info := VersionInfo{
		Current: current,
		Version: current,
	}
	if latest == "" {
		return info
	}
	info.Latest = latest
	info.ReleaseURL = releaseURL
	info.CheckedAt = checkedAt.Unix()
	info.UpdateAvailable = versionIsNewer(latest, current)
	return info
}

func latestReleaseCached() (string, string, time.Time) {
	versionCheckState.Lock()
	now := time.Now()
	if !versionCheckState.checkedAt.IsZero() && now.Sub(versionCheckState.checkedAt) < versionCheckCache {
		latest := versionCheckState.latest
		releaseURL := versionCheckState.releaseURL
		checkedAt := versionCheckState.checkedAt
		versionCheckState.Unlock()
		return latest, releaseURL, checkedAt
	}
	client := versionCheckState.client
	url := versionCheckState.url
	versionCheckState.Unlock()

	latest, releaseURL, err := fetchLatestRelease(client, url)
	if err != nil {
		logger.Warning("version check failed:", err)
	}

	versionCheckState.Lock()
	defer versionCheckState.Unlock()
	versionCheckState.checkedAt = now
	if err == nil {
		versionCheckState.latest = latest
		versionCheckState.releaseURL = releaseURL
	}
	return versionCheckState.latest, versionCheckState.releaseURL, versionCheckState.checkedAt
}

func fetchLatestRelease(client *http.Client, url string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), versionCheckTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "s-ui-version-check")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		return "", "", http.ErrNotSupported
	}
	var release latestRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return "", "", err
	}
	release.TagName = strings.TrimSpace(release.TagName)
	release.HTMLURL = strings.TrimSpace(release.HTMLURL)
	return release.TagName, release.HTMLURL, nil
}

func versionIsNewer(latest string, current string) bool {
	latestParts, okLatest := parseVersion(latest)
	currentParts, okCurrent := parseVersion(current)
	if !okLatest || !okCurrent {
		return normalizeVersion(latest) != "" && normalizeVersion(latest) != normalizeVersion(current)
	}
	for i := 0; i < len(latestParts); i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

func parseVersion(value string) ([3]int, bool) {
	var parts [3]int
	value = normalizeVersion(value)
	items := strings.Split(value, ".")
	if len(items) == 0 {
		return parts, false
	}
	for i := 0; i < len(parts) && i < len(items); i++ {
		item := takeLeadingDigits(items[i])
		if item == "" {
			return parts, false
		}
		n, err := strconv.Atoi(item)
		if err != nil {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	value = strings.TrimPrefix(value, "V")
	return value
}

func takeLeadingDigits(value string) string {
	for i, r := range value {
		if r < '0' || r > '9' {
			return value[:i]
		}
	}
	return value
}
