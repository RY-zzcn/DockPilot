package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dockpilot/dockpilot/internal/version"
)

type ReleaseService struct {
	repo     string
	cacheTTL time.Duration
	client   *http.Client
	mu       sync.Mutex
	cached   ReleaseInfo
	cachedAt time.Time
}

type ReleaseInfo struct {
	Repository      string         `json:"repository"`
	LatestVersion   string         `json:"latest_version"`
	LatestTag       string         `json:"latest_tag"`
	Name            string         `json:"name,omitempty"`
	URL             string         `json:"url,omitempty"`
	PublishedAt     string         `json:"published_at,omitempty"`
	CheckedAt       string         `json:"checked_at"`
	UpdateAvailable bool           `json:"update_available"`
	Assets          []ReleaseAsset `json:"assets,omitempty"`
	Error           string         `json:"error,omitempty"`
}

type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Size        int64  `json:"size"`
}

func NewReleaseService(repo string, cacheTTL time.Duration) *ReleaseService {
	if cacheTTL <= 0 {
		cacheTTL = 15 * time.Minute
	}
	return &ReleaseService{
		repo:     strings.TrimSpace(repo),
		cacheTTL: cacheTTL,
		client:   &http.Client{Timeout: 12 * time.Second},
	}
}

func (s *ReleaseService) Latest(ctx context.Context, currentVersion string, force bool) ReleaseInfo {
	now := time.Now()
	s.mu.Lock()
	if !force && !s.cachedAt.IsZero() && now.Sub(s.cachedAt) < s.cacheTTL {
		info := s.withCurrentLocked(currentVersion)
		s.mu.Unlock()
		return info
	}
	s.mu.Unlock()

	info, err := s.fetchLatest(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		hadCache := !s.cachedAt.IsZero()
		s.cachedAt = now
		if hadCache {
			cached := s.withCurrentLocked(currentVersion)
			cached.Error = err.Error()
			s.cached.Error = cached.Error
			return cached
		}
		s.cached = ReleaseInfo{
			Repository: s.repo,
			CheckedAt:  now.Format(time.RFC3339),
			Error:      err.Error(),
		}
		return s.withCurrentLocked(currentVersion)
	}
	s.cached = info
	s.cachedAt = now
	return s.withCurrentLocked(currentVersion)
}

func (s *ReleaseService) Repository() string {
	return s.repo
}

func (s *ReleaseService) withCurrentLocked(currentVersion string) ReleaseInfo {
	info := s.cached
	info.Repository = s.repo
	info.UpdateAvailable = version.IsOutdated(currentVersion, info.LatestVersion)
	return info
}

func (s *ReleaseService) fetchLatest(ctx context.Context) (ReleaseInfo, error) {
	if s.repo == "" {
		return ReleaseInfo{}, fmt.Errorf("release repository is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+s.repo+"/releases/latest", nil)
	if err != nil {
		return ReleaseInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DockPilot-Server")
	resp, err := s.client.Do(req)
	if err != nil {
		return ReleaseInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return ReleaseInfo{}, fmt.Errorf("release lookup failed: %s", resp.Status)
	}
	var body struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ReleaseInfo{}, err
	}
	clean := version.Clean(body.TagName)
	if clean == "" {
		return ReleaseInfo{}, fmt.Errorf("latest release has no tag")
	}
	assets := make([]ReleaseAsset, 0, len(body.Assets))
	for _, asset := range body.Assets {
		assets = append(assets, ReleaseAsset{
			Name:        asset.Name,
			DownloadURL: asset.BrowserDownloadURL,
			Size:        asset.Size,
		})
	}
	return ReleaseInfo{
		Repository:    s.repo,
		LatestVersion: clean,
		LatestTag:     version.EnsureVPrefix(clean),
		Name:          body.Name,
		URL:           body.HTMLURL,
		PublishedAt:   body.PublishedAt,
		CheckedAt:     time.Now().Format(time.RFC3339),
		Assets:        assets,
	}, nil
}
