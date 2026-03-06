package update

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:embed sources.json
var embeddedSources []byte

type ManifestSource struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Type              string `json:"type"` // manifest | github
	URL               string `json:"url"`
	Repo              string `json:"repo"` // github: owner/repo
	APIURL            string `json:"api_url"`
	AssetNameContains string `json:"asset_name_contains"`
	TimeoutSeconds    int    `json:"timeout_seconds"`
}

type DownloadSource struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	ArchURLs map[string]string `json:"arch_urls"`
}

type SourcesConfig struct {
	DefaultManifestSource string           `json:"default_manifest_source"`
	DefaultDownloadSource string           `json:"default_download_source"`
	ManifestSources       []ManifestSource `json:"manifest_sources"`
	DownloadSources       []DownloadSource `json:"download_sources"`
}

type ManifestPayload struct {
	Version      string `json:"version"`
	DownloadURL  string `json:"download_url"`
	Notes        string `json:"notes"`
	ReleaseNotes string `json:"release_notes"`
}

type CheckResult struct {
	LocalVersion     string `json:"local_version"`
	LocalComparable  string `json:"local_comparable"`
	RemoteVersion    string `json:"remote_version"`
	RemoteComparable string `json:"remote_comparable"`
	HasUpdate        bool   `json:"has_update"`

	ManifestSourceID string `json:"manifest_source_id"`
	ManifestSource   string `json:"manifest_source"`
	ManifestType     string `json:"manifest_type"`
	ManifestURL      string `json:"manifest_url"`

	DownloadSourceID string `json:"download_source_id"`
	DownloadSource   string `json:"download_source"`
	DownloadURL      string `json:"download_url"`

	Notes string `json:"notes"`
}

func Check(ctx context.Context, localVersion string, arch string) (CheckResult, error) {
	cfg, err := LoadSourcesConfig()
	if err != nil {
		return CheckResult{}, err
	}
	manifestSrc, err := cfg.DefaultManifest()
	if err != nil {
		return CheckResult{}, err
	}
	downloadSrc, _ := cfg.DefaultDownload(arch)

	manifest, err := fetchManifest(ctx, manifestSrc, arch)
	if err != nil {
		return CheckResult{}, err
	}

	remoteVersion := strings.TrimSpace(manifest.Version)
	if remoteVersion == "" {
		return CheckResult{}, fmt.Errorf("manifest missing version")
	}

	localComparable := normalizeVersion(localVersion)
	remoteComparable := normalizeVersion(remoteVersion)
	hasUpdate := compareNormalizedVersion(remoteComparable, localComparable) > 0

	downloadURL := strings.TrimSpace(manifest.DownloadURL)
	if downloadURL == "" && downloadSrc != nil {
		downloadURL = downloadSrc.URL
	}

	result := CheckResult{
		LocalVersion:     strings.TrimSpace(localVersion),
		LocalComparable:  localComparable,
		RemoteVersion:    remoteVersion,
		RemoteComparable: remoteComparable,
		HasUpdate:        hasUpdate,

		ManifestSourceID: manifestSrc.ID,
		ManifestSource:   manifestSrc.Name,
		ManifestType:     normalizeSourceType(manifestSrc.Type),
		ManifestURL:      sourceURLForResult(manifestSrc),

		DownloadURL: downloadURL,
		Notes:       firstNonEmpty(manifest.ReleaseNotes, manifest.Notes),
	}
	if downloadSrc != nil {
		result.DownloadSourceID = downloadSrc.ID
		result.DownloadSource = downloadSrc.Name
	}
	return result, nil
}

func LoadSourcesConfig() (SourcesConfig, error) {
	var cfg SourcesConfig
	if err := json.Unmarshal(embeddedSources, &cfg); err != nil {
		return SourcesConfig{}, fmt.Errorf("invalid update sources config: %w", err)
	}
	if len(cfg.ManifestSources) == 0 {
		return SourcesConfig{}, fmt.Errorf("manifest_sources is empty")
	}
	if strings.TrimSpace(cfg.DefaultManifestSource) == "" {
		cfg.DefaultManifestSource = cfg.ManifestSources[0].ID
	}
	return cfg, nil
}

func (c SourcesConfig) DefaultManifest() (*ManifestSource, error) {
	target := strings.TrimSpace(c.DefaultManifestSource)
	for i := range c.ManifestSources {
		item := c.ManifestSources[i]
		switch normalizeSourceType(item.Type) {
		case "github":
			if item.ID == target {
				if strings.TrimSpace(item.Repo) == "" && strings.TrimSpace(item.APIURL) == "" {
					return nil, fmt.Errorf("default github source requires repo or api_url")
				}
				return &item, nil
			}
		default:
			if item.ID == target {
				if strings.TrimSpace(item.URL) == "" {
					return nil, fmt.Errorf("default manifest source url is empty")
				}
				return &item, nil
			}
		}
	}
	return nil, fmt.Errorf("default manifest source not found: %s", target)
}

func (c SourcesConfig) DefaultDownload(arch string) (*DownloadSource, error) {
	target := strings.TrimSpace(c.DefaultDownloadSource)
	if target == "" || len(c.DownloadSources) == 0 {
		return nil, nil
	}
	for i := range c.DownloadSources {
		item := c.DownloadSources[i]
		if strings.TrimSpace(item.ID) == target {
			if archURL := strings.TrimSpace(item.archURL(arch)); archURL != "" {
				item.URL = archURL
			}
			return &item, nil
		}
	}
	return nil, fmt.Errorf("default download source not found: %s", target)
}

func fetchManifest(ctx context.Context, src *ManifestSource, arch string) (ManifestPayload, error) {
	if normalizeSourceType(src.Type) == "github" {
		return fetchGitHubManifest(ctx, src, arch)
	}
	return fetchGenericManifest(ctx, src, arch)
}

func fetchGenericManifest(ctx context.Context, src *ManifestSource, arch string) (ManifestPayload, error) {
	timeout := sourceTimeout(src.TimeoutSeconds)
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, src.URL, nil)
	if err != nil {
		return ManifestPayload{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "LDM-UpdateChecker/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ManifestPayload{}, fmt.Errorf("fetch manifest failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ManifestPayload{}, fmt.Errorf("manifest http status: %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return ManifestPayload{}, err
	}

	var payload ManifestPayload
	if err := json.Unmarshal(raw, &payload); err == nil && strings.TrimSpace(payload.Version) != "" {
		return payload, nil
	}

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return ManifestPayload{}, fmt.Errorf("invalid manifest json: %w", err)
	}

	if latestMap, ok := generic["latest"].(map[string]any); ok {
		payload.Version = pickString(latestMap, "version", "latest", "tag_name", "tag")
		if filesMap, ok := latestMap["files"].(map[string]any); ok {
			payload.DownloadURL = pickString(filesMap, archAliases(arch)...)
		}
		if strings.TrimSpace(payload.DownloadURL) == "" {
			payload.DownloadURL = pickString(latestMap, "download_url", "binary_url", "asset_url", "url")
		}
		payload.Notes = pickString(latestMap, "notes", "changelog", "message")
		payload.ReleaseNotes = pickString(latestMap, "release_notes", "releaseNote", "release")
		if strings.TrimSpace(payload.Version) != "" {
			return payload, nil
		}
	}

	payload.Version = pickString(generic, "version", "latest", "tag_name", "tag")
	payload.DownloadURL = pickString(generic, "download_url", "binary_url", "asset_url", "url")
	payload.Notes = pickString(generic, "notes", "changelog", "message")
	payload.ReleaseNotes = pickString(generic, "release_notes", "releaseNote", "release")
	if strings.TrimSpace(payload.Version) == "" {
		return ManifestPayload{}, fmt.Errorf("manifest version not found")
	}
	return payload, nil
}

func fetchGitHubManifest(ctx context.Context, src *ManifestSource, arch string) (ManifestPayload, error) {
	timeout := sourceTimeout(src.TimeoutSeconds)
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	apiURL := strings.TrimSpace(src.APIURL)
	if apiURL == "" {
		repo := strings.Trim(strings.TrimSpace(src.Repo), "/")
		if repo == "" {
			return ManifestPayload{}, fmt.Errorf("github source missing repo")
		}
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL, nil)
	if err != nil {
		return ManifestPayload{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "LDM-UpdateChecker/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ManifestPayload{}, fmt.Errorf("fetch github release failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return ManifestPayload{}, fmt.Errorf("github release http status: %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return ManifestPayload{}, err
	}

	var release struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Body    string `json:"body"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(raw, &release); err != nil {
		return ManifestPayload{}, fmt.Errorf("invalid github release json: %w", err)
	}
	version := strings.TrimSpace(release.TagName)
	if version == "" {
		version = strings.TrimSpace(release.Name)
	}
	if version == "" {
		return ManifestPayload{}, fmt.Errorf("github release missing tag_name")
	}

	match := strings.ToLower(strings.TrimSpace(src.AssetNameContains))
	if match == "" {
		match = "ldm-linux-{arch}"
	}
	match = strings.ReplaceAll(match, "{arch}", normalizeArch(arch))
	match = strings.ToLower(match)

	downloadURL := ""
	for _, a := range release.Assets {
		if strings.TrimSpace(a.BrowserDownloadURL) == "" {
			continue
		}
		if strings.Contains(strings.ToLower(a.Name), match) {
			downloadURL = strings.TrimSpace(a.BrowserDownloadURL)
			break
		}
	}
	if downloadURL == "" && len(release.Assets) > 0 {
		downloadURL = strings.TrimSpace(release.Assets[0].BrowserDownloadURL)
	}

	return ManifestPayload{
		Version:      version,
		DownloadURL:  downloadURL,
		ReleaseNotes: strings.TrimSpace(release.Body),
	}, nil
}

func (d DownloadSource) archURL(arch string) string {
	if len(d.ArchURLs) == 0 {
		return ""
	}
	return strings.TrimSpace(d.ArchURLs[normalizeArch(arch)])
}

func normalizeSourceType(v string) string {
	t := strings.ToLower(strings.TrimSpace(v))
	if t == "github" {
		return "github"
	}
	return "manifest"
}

func sourceURLForResult(src *ManifestSource) string {
	if src == nil {
		return ""
	}
	if normalizeSourceType(src.Type) == "github" {
		if strings.TrimSpace(src.APIURL) != "" {
			return strings.TrimSpace(src.APIURL)
		}
		repo := strings.Trim(strings.TrimSpace(src.Repo), "/")
		if repo != "" {
			return "https://api.github.com/repos/" + repo + "/releases/latest"
		}
	}
	return strings.TrimSpace(src.URL)
}

func sourceTimeout(v int) time.Duration {
	if v > 0 && v < 120 {
		return time.Duration(v) * time.Second
	}
	return 8 * time.Second
}

func normalizeArch(arch string) string {
	v := strings.ToLower(strings.TrimSpace(arch))
	switch v {
	case "x86_64", "x64":
		return "amd64"
	case "aarch64":
		return "arm64"
	case "":
		return "amd64"
	default:
		return v
	}
}

func archAliases(arch string) []string {
	if normalizeArch(arch) == "arm64" {
		return []string{"arm64", "aarch64", "linux_arm64", "linux-aarch64", "linux_arm"}
	}
	return []string{"amd64", "x86_64", "linux_amd64", "linux-x86_64"}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func pickString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

var versionPattern = regexp.MustCompile(`\d+(?:\.\d+)+`)

func normalizeVersion(raw string) string {
	raw = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(raw, "v"), "V"))
	match := versionPattern.FindString(raw)
	return strings.TrimSpace(match)
}

func compareNormalizedVersion(a, b string) int {
	ap := parseParts(a)
	bp := parseParts(b)
	maxLen := len(ap)
	if len(bp) > maxLen {
		maxLen = len(bp)
	}
	for i := 0; i < maxLen; i++ {
		av := 0
		if i < len(ap) {
			av = ap[i]
		}
		bv := 0
		if i < len(bp) {
			bv = bp[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func parseParts(v string) []int {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	items := strings.Split(v, ".")
	out := make([]int, 0, len(items))
	for _, item := range items {
		num, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil {
			return nil
		}
		out = append(out, num)
	}
	return out
}
