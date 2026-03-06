package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"strings"
)

type ServiceManifestSource struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
}

type SealdiceRelease struct {
	SourceID    string `json:"source_id"`
	SourceName  string `json:"source_name"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
}

type LagrangeVersion struct {
	Key         string `json:"key"`
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	IsLatest    bool   `json:"is_latest"`
}

func ServiceManifestSourceInfo() (ServiceManifestSource, error) {
	cfg, err := LoadSourcesConfig()
	if err != nil {
		return ServiceManifestSource{}, err
	}
	src, err := cfg.defaultServiceManifestSource()
	if err != nil {
		return ServiceManifestSource{}, err
	}
	base, err := manifestBaseURL(src.URL)
	if err != nil {
		return ServiceManifestSource{}, err
	}
	return ServiceManifestSource{
		ID:      src.ID,
		Name:    src.Name,
		BaseURL: base,
	}, nil
}

func FetchSealdiceLatest(ctx context.Context, arch string) (SealdiceRelease, error) {
	srcInfo, err := ServiceManifestSourceInfo()
	if err != nil {
		return SealdiceRelease{}, err
	}

	body, err := fetchJSON(ctx, strings.TrimRight(srcInfo.BaseURL, "/")+"/Sealdice/latest.json", 8)
	if err != nil {
		return SealdiceRelease{}, err
	}

	var root struct {
		Latest struct {
			Version string            `json:"version"`
			Files   map[string]string `json:"files"`
		} `json:"latest"`
	}
	if err := json.Unmarshal(body, &root); err != nil {
		return SealdiceRelease{}, fmt.Errorf("invalid sealdice latest manifest: %w", err)
	}
	if strings.TrimSpace(root.Latest.Version) == "" {
		return SealdiceRelease{}, errors.New("sealdice latest manifest missing version")
	}

	url := pickArchURL(root.Latest.Files, normalizeArch(arch), false)
	if strings.TrimSpace(url) == "" {
		return SealdiceRelease{}, fmt.Errorf("sealdice manifest missing file for arch %s", normalizeArch(arch))
	}

	return SealdiceRelease{
		SourceID:    srcInfo.ID,
		SourceName:  srcInfo.Name,
		Version:     strings.TrimSpace(root.Latest.Version),
		DownloadURL: strings.TrimSpace(url),
	}, nil
}

func FetchLagrangeVersions(ctx context.Context, arch string) ([]LagrangeVersion, error) {
	srcInfo, err := ServiceManifestSourceInfo()
	if err != nil {
		return nil, err
	}

	body, err := fetchJSON(ctx, strings.TrimRight(srcInfo.BaseURL, "/")+"/Lagrange/versions.json", 8)
	if err != nil {
		return nil, err
	}

	type lagrangeNode struct {
		Version string            `json:"version"`
		Files   map[string]string `json:"files"`
	}
	var root struct {
		Original map[string]lagrangeNode `json:"original"`
		Forks    map[string]any          `json:"forks"`
	}
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("invalid lagrange versions manifest: %w", err)
	}

	keyArch := lagrangeArchKey(normalizeArch(arch))
	versions := make([]LagrangeVersion, 0, len(root.Original))
	for key, node := range root.Original {
		downloadURL := strings.TrimSpace(node.Files[keyArch])
		if downloadURL == "" {
			continue
		}
		versions = append(versions, LagrangeVersion{
			Key:         strings.TrimSpace(key),
			Version:     strings.TrimSpace(node.Version),
			DownloadURL: downloadURL,
			IsLatest:    strings.EqualFold(strings.TrimSpace(key), "latest"),
		})
	}
	if len(versions) == 0 {
		return nil, errors.New("no compatible lagrange versions for current architecture")
	}

	sort.SliceStable(versions, func(i, j int) bool {
		if versions[i].IsLatest != versions[j].IsLatest {
			return versions[i].IsLatest
		}
		return versions[i].Key > versions[j].Key
	})
	return versions, nil
}

func ResolveLagrangeDownloadURL(ctx context.Context, arch, key string) (LagrangeVersion, error) {
	items, err := FetchLagrangeVersions(ctx, arch)
	if err != nil {
		return LagrangeVersion{}, err
	}
	target := strings.TrimSpace(key)
	if target == "" || strings.EqualFold(target, "old") {
		for _, item := range items {
			if !item.IsLatest {
				return item, nil
			}
		}
		return items[0], nil
	}
	if strings.EqualFold(target, "latest") {
		for _, item := range items {
			if item.IsLatest {
				return item, nil
			}
		}
		return items[0], nil
	}
	for _, item := range items {
		if strings.EqualFold(item.Key, target) || strings.EqualFold(item.Version, target) {
			return item, nil
		}
	}
	return LagrangeVersion{}, fmt.Errorf("lagrange version not found: %s", target)
}

func (c SourcesConfig) defaultServiceManifestSource() (*ManifestSource, error) {
	for i := range c.ManifestSources {
		item := c.ManifestSources[i]
		if normalizeSourceType(item.Type) != "manifest" {
			continue
		}
		if !item.SupportsService && !item.SupportsRemotePkg {
			continue
		}
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		return &item, nil
	}
	return nil, errors.New("no service manifest source configured")
}

func manifestBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(u.Scheme) == "" || strings.TrimSpace(u.Host) == "" {
		return "", errors.New("invalid manifest source url")
	}
	return strings.TrimRight((&url.URL{Scheme: u.Scheme, Host: u.Host}).String(), "/"), nil
}

func fetchJSON(ctx context.Context, rawURL string, timeoutSeconds int) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, sourceTimeout(timeoutSeconds))
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "LDM-ServiceManifest/1.0 ("+runtime.GOOS+"/"+runtime.GOARCH+")")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
}

func lagrangeArchKey(arch string) string {
	switch normalizeArch(arch) {
	case "arm64":
		return "linux_arm64"
	case "arm":
		return "linux_arm"
	default:
		return "linux_x64"
	}
}

func pickArchURL(files map[string]string, arch string, lagrangeStyle bool) string {
	if len(files) == 0 {
		return ""
	}
	if lagrangeStyle {
		return strings.TrimSpace(files[lagrangeArchKey(arch)])
	}
	if normalizeArch(arch) == "arm64" {
		return firstNonEmpty(files["arm64"], files["aarch64"])
	}
	return firstNonEmpty(files["amd64"], files["x64"], files["x86_64"])
}
