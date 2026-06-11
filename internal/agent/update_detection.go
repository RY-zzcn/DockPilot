package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dockpilot/dockpilot/internal/protocol"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var (
	digestPattern      = regexp.MustCompile(`(?i)sha256:[a-f0-9]{64}`)
	containerIDPattern = regexp.MustCompile(`^[a-f0-9]{12,64}$`)
)

type commandRunner func(context.Context, string, ...string) (string, error)
type registryLookup func(context.Context, string, platformSpec) (remoteImageInfo, error)

type UpdateDetector struct {
	mu       sync.Mutex
	ttl      time.Duration
	cache    map[string]cachedRemoteInfo
	command  commandRunner
	registry registryLookup
	now      func() time.Time
}

type cachedRemoteInfo struct {
	info      remoteImageInfo
	expiresAt time.Time
}

type platformSpec struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Variant      string `json:"variant,omitempty"`
}

type localImageInfo struct {
	ConfigDigest    string
	ConfigDigests   []string
	RepoDigests     []string
	ManifestDigests []string
	Platform        platformSpec
	Missing         bool
}

type remoteImageInfo struct {
	ConfigDigest   string
	ManifestDigest string
	Method         string
}

func NewUpdateDetector(ttl time.Duration) *UpdateDetector {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	detector := &UpdateDetector{
		ttl:     ttl,
		cache:   map[string]cachedRemoteInfo{},
		command: commandCombined,
		now:     time.Now,
	}
	detector.registry = detector.registryRemoteInfo
	return detector
}

func (d *UpdateDetector) Detect(ctx context.Context, image string) protocol.ImageUpdateDetection {
	if d == nil {
		d = NewUpdateDetector(15 * time.Minute)
	}
	local, _ := d.localImageInfo(ctx, image)
	return d.detectWithLocal(ctx, image, local)
}

func (d *UpdateDetector) DetectWithLocal(ctx context.Context, image string, local localImageInfo) protocol.ImageUpdateDetection {
	if d == nil {
		d = NewUpdateDetector(15 * time.Minute)
	}
	return d.detectWithLocal(ctx, image, local)
}

func (d *UpdateDetector) detectWithLocal(ctx context.Context, image string, local localImageInfo) protocol.ImageUpdateDetection {
	checkedAt := d.now().Format(time.RFC3339)
	result := protocol.ImageUpdateDetection{
		Image:     image,
		CheckedAt: checkedAt,
	}
	platform := local.Platform
	if platform.empty() {
		platform = defaultPlatform()
	}
	result.Platform = platform.String()
	result.LocalConfigDigest = firstDigest(localConfigDigests(local), local.ConfigDigest)
	result.LocalManifestDigest = firstDigest(local.ManifestDigests, "")
	result.LocalDigest = firstDigest(local.ManifestDigests, firstDigest(local.RepoDigests, result.LocalConfigDigest))

	if pinnedDigest := digestFromReference(image); pinnedDigest != "" {
		result.Method = "pinned"
		result.Pinned = true
		result.RemoteManifestDigest = pinnedDigest
		result.RemoteDigest = pinnedDigest
		result.UpdateAvailable = false
		return result
	}

	remoteInfo, remoteErr := d.remoteInfo(ctx, image, platform)
	result.Method = remoteInfo.Method
	result.RemoteConfigDigest = remoteInfo.ConfigDigest
	result.RemoteManifestDigest = remoteInfo.ManifestDigest
	result.RemoteDigest = firstDigest([]string{remoteInfo.ManifestDigest}, remoteInfo.ConfigDigest)
	if remoteErr != nil {
		result.Error = remoteErr.Error()
		return result
	}
	result.UpdateAvailable = updateAvailable(local, remoteInfo)
	return result
}

func (d *UpdateDetector) localImageInfo(ctx context.Context, image string) (localImageInfo, error) {
	out, err := d.command(ctx, "docker", "image", "inspect", image)
	if err != nil {
		return localImageInfo{Missing: true}, fmt.Errorf("local image is missing or unavailable")
	}
	return localImageInfoFromInspectOutput(out, d.localContainerManifestDigests(ctx, image))
}

func localImageInfoFromInspectOutput(out string, manifestDigests []string) (localImageInfo, error) {
	var raw []struct {
		ID           string   `json:"Id"`
		RepoDigests  []string `json:"RepoDigests"`
		OS           string   `json:"Os"`
		Architecture string   `json:"Architecture"`
		Variant      string   `json:"Variant"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil || len(raw) == 0 {
		return localImageInfo{Missing: true}, fmt.Errorf("local image inspect output is invalid")
	}
	imageInfo := raw[0]
	configDigest := digestOnly(imageInfo.ID)
	var repoDigests []string
	for _, repoDigest := range imageInfo.RepoDigests {
		if digest := digestFromReference(repoDigest); digest != "" {
			repoDigests = append(repoDigests, digest)
		}
	}
	return localImageInfo{
		ConfigDigest:    configDigest,
		ConfigDigests:   uniqueDigests([]string{configDigest}),
		RepoDigests:     repoDigests,
		ManifestDigests: uniqueDigests(manifestDigests),
		Platform: platformSpec{
			OS:           imageInfo.OS,
			Architecture: imageInfo.Architecture,
			Variant:      imageInfo.Variant,
		},
	}, nil
}

func (d *UpdateDetector) localContainerManifestDigests(ctx context.Context, image string) []string {
	out, err := d.command(ctx, "docker", "ps", "-aq", "--filter", "ancestor="+image)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var digests []string
	for _, line := range strings.Split(out, "\n") {
		containerID := strings.TrimSpace(line)
		if !containerIDPattern.MatchString(containerID) {
			continue
		}
		inspectOut, err := d.command(ctx, "docker", "container", "inspect", containerID)
		if err != nil {
			continue
		}
		digestsForContainer := parseContainerManifestDigests(inspectOut)
		if len(digestsForContainer) == 0 {
			if digest := containerManifestDigestFromDockerAPI(ctx, containerID); digest != "" {
				digestsForContainer = append(digestsForContainer, digest)
			}
		}
		for _, digest := range digestsForContainer {
			if !seen[digest] {
				seen[digest] = true
				digests = append(digests, digest)
			}
		}
	}
	return digests
}

type containerInspectDigest struct {
	ImageManifestDescriptor struct {
		Digest string `json:"digest"`
	} `json:"ImageManifestDescriptor"`
}

func parseContainerManifestDigests(output string) []string {
	var raw []containerInspectDigest
	if json.Unmarshal([]byte(output), &raw) == nil {
		return containerManifestDigests(raw)
	}
	var single containerInspectDigest
	if json.Unmarshal([]byte(output), &single) == nil {
		return containerManifestDigests([]containerInspectDigest{single})
	}
	return nil
}

func containerManifestDigests(raw []containerInspectDigest) []string {
	var digests []string
	for _, item := range raw {
		if digest := digestOnly(item.ImageManifestDescriptor.Digest); digest != "" {
			digests = append(digests, digest)
		}
	}
	return digests
}

func containerManifestDigestFromDockerAPI(ctx context.Context, containerID string) string {
	socketPath := dockerSocketPath()
	if socketPath == "" {
		return ""
	}
	dialer := net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/"+url.PathEscape(containerID)+"/json", nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	var inspect containerInspectDigest
	if err := json.NewDecoder(resp.Body).Decode(&inspect); err != nil {
		return ""
	}
	return digestOnly(inspect.ImageManifestDescriptor.Digest)
}

func dockerSocketPath() string {
	host := strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	if host == "" {
		return "/var/run/docker.sock"
	}
	if strings.HasPrefix(host, "unix://") {
		return strings.TrimPrefix(host, "unix://")
	}
	return ""
}

func (d *UpdateDetector) remoteInfo(ctx context.Context, image string, platform platformSpec) (remoteImageInfo, error) {
	key := image + "|" + platform.String()
	now := d.now()
	d.mu.Lock()
	if cached, ok := d.cache[key]; ok && now.Before(cached.expiresAt) {
		d.mu.Unlock()
		return cached.info, nil
	}
	d.mu.Unlock()

	info, err := d.registry(ctx, image, platform)
	if err != nil {
		info, err = d.cliRemoteInfo(ctx, image, platform)
	}
	if err != nil {
		return info, err
	}

	d.mu.Lock()
	d.cache[key] = cachedRemoteInfo{info: info, expiresAt: now.Add(d.ttl)}
	d.mu.Unlock()
	return info, nil
}

func (d *UpdateDetector) registryRemoteInfo(ctx context.Context, image string, platform platformSpec) (remoteImageInfo, error) {
	ref, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		return remoteImageInfo{}, err
	}
	options := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithPlatform(platform.v1()),
	}
	img, err := remote.Image(ref, options...)
	if err != nil {
		return remoteImageInfo{}, err
	}
	configDigest, err := img.ConfigName()
	if err != nil {
		return remoteImageInfo{}, err
	}
	manifestDigest, err := img.Digest()
	if err != nil {
		return remoteImageInfo{}, err
	}
	return remoteImageInfo{
		ConfigDigest:   digestOnly(configDigest.String()),
		ManifestDigest: digestOnly(manifestDigest.String()),
		Method:         "registry",
	}, nil
}

func (d *UpdateDetector) cliRemoteInfo(ctx context.Context, image string, platform platformSpec) (remoteImageInfo, error) {
	rawInfo := remoteImageInfo{}
	out, err := d.command(ctx, "docker", "buildx", "imagetools", "inspect", "--raw", image)
	if err == nil {
		rawInfo = parseRawManifestInfo(out, platform)
		if rawInfo.ConfigDigest != "" {
			rawInfo.Method = "cli"
			return rawInfo, nil
		}
		if !rawInfo.empty() {
			rawInfo.Method = "cli"
		}
	}
	firstErr := strings.TrimSpace(out)
	out, err = d.command(ctx, "docker", "manifest", "inspect", "--verbose", image)
	if err == nil {
		if info := parseVerboseManifestInfo(out, platform); !info.empty() {
			if info.ConfigDigest == "" {
				info.ConfigDigest = rawInfo.ConfigDigest
			}
			if info.ManifestDigest == "" {
				info.ManifestDigest = rawInfo.ManifestDigest
			}
			info.Method = "cli"
			return info, nil
		}
	}
	if !rawInfo.empty() {
		return rawInfo, nil
	}
	message := strings.TrimSpace(out)
	if message == "" {
		message = firstErr
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	return remoteImageInfo{Method: "cli"}, fmt.Errorf("remote digest unavailable: %s", message)
}

func parseRawManifestInfo(output string, platform platformSpec) remoteImageInfo {
	var raw struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
		Manifests []struct {
			Digest   string       `json:"digest"`
			Platform platformSpec `json:"platform"`
		} `json:"manifests"`
	}
	if json.Unmarshal([]byte(output), &raw) != nil {
		return remoteImageInfo{}
	}
	if digest := digestOnly(raw.Config.Digest); digest != "" {
		return remoteImageInfo{ConfigDigest: digest}
	}
	for _, manifest := range raw.Manifests {
		if manifest.Platform.matches(platform) {
			return remoteImageInfo{ManifestDigest: digestOnly(manifest.Digest)}
		}
	}
	if len(raw.Manifests) == 1 {
		return remoteImageInfo{ManifestDigest: digestOnly(raw.Manifests[0].Digest)}
	}
	return remoteImageInfo{}
}

func parseVerboseManifestInfo(output string, platform platformSpec) remoteImageInfo {
	var list []verboseManifestItem
	if json.Unmarshal([]byte(output), &list) == nil {
		for _, item := range list {
			if item.Descriptor.Platform.matches(platform) {
				return item.remoteInfo()
			}
		}
		if len(list) == 1 {
			return list[0].remoteInfo()
		}
		return remoteImageInfo{}
	}
	var item verboseManifestItem
	if json.Unmarshal([]byte(output), &item) == nil {
		return item.remoteInfo()
	}
	return remoteImageInfo{}
}

type verboseManifestItem struct {
	Descriptor struct {
		Digest   string       `json:"digest"`
		Platform platformSpec `json:"platform"`
	} `json:"Descriptor"`
	SchemaV2Manifest struct {
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
	} `json:"SchemaV2Manifest"`
}

func (v verboseManifestItem) remoteInfo() remoteImageInfo {
	return remoteImageInfo{
		ConfigDigest:   digestOnly(v.SchemaV2Manifest.Config.Digest),
		ManifestDigest: digestOnly(v.Descriptor.Digest),
	}
}

func (r remoteImageInfo) empty() bool {
	return r.ConfigDigest == "" && r.ManifestDigest == ""
}

func (p platformSpec) empty() bool {
	return p.OS == "" && p.Architecture == ""
}

func (p platformSpec) String() string {
	osName := nonEmpty(p.OS, runtime.GOOS)
	arch := nonEmpty(p.Architecture, runtime.GOARCH)
	if p.Variant != "" {
		return osName + "/" + arch + "/" + p.Variant
	}
	return osName + "/" + arch
}

func (p platformSpec) matches(target platformSpec) bool {
	if p.OS == "" || p.Architecture == "" {
		return false
	}
	if !strings.EqualFold(p.OS, nonEmpty(target.OS, runtime.GOOS)) {
		return false
	}
	if !strings.EqualFold(p.Architecture, nonEmpty(target.Architecture, runtime.GOARCH)) {
		return false
	}
	if target.Variant == "" {
		return true
	}
	return strings.EqualFold(p.Variant, target.Variant)
}

func (p platformSpec) v1() v1.Platform {
	return v1.Platform{
		OS:           nonEmpty(p.OS, runtime.GOOS),
		Architecture: nonEmpty(p.Architecture, runtime.GOARCH),
		Variant:      p.Variant,
	}
}

func defaultPlatform() platformSpec {
	return platformSpec{OS: runtime.GOOS, Architecture: runtime.GOARCH}
}

func digestFromInspectOutput(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "digest:") {
			if digest := digestPattern.FindString(trimmed); digest != "" {
				return strings.ToLower(digest)
			}
		}
	}
	if digest := digestPattern.FindString(output); digest != "" {
		return strings.ToLower(digest)
	}
	return ""
}

func digestFromReference(value string) string {
	if at := strings.LastIndex(value, "@"); at >= 0 {
		return digestOnly(value[at+1:])
	}
	return ""
}

func digestOnly(value string) string {
	if digest := digestPattern.FindString(value); digest != "" {
		return strings.ToLower(digest)
	}
	return ""
}

func updateAvailable(local localImageInfo, remote remoteImageInfo) bool {
	if remote.empty() {
		return false
	}
	if local.Missing {
		return true
	}
	manifestDigests := uniqueDigests(local.ManifestDigests)
	if remote.ManifestDigest != "" && len(manifestDigests) > 0 {
		return !allDigestsMatch(manifestDigests, remote.ManifestDigest)
	}
	if remote.ManifestDigest != "" && containsDigest(local.RepoDigests, remote.ManifestDigest) {
		return false
	}
	configDigests := localConfigDigests(local)
	if remote.ConfigDigest != "" && len(configDigests) > 0 {
		return !allDigestsMatch(configDigests, remote.ConfigDigest)
	}
	if len(manifestDigests) > 0 && remote.ManifestDigest != "" {
		return true
	}
	if len(configDigests) > 0 && remote.ConfigDigest != "" {
		return true
	}
	if remote.ManifestDigest != "" && len(local.RepoDigests) > 0 {
		return true
	}
	if local.ConfigDigest == "" && (remote.ConfigDigest != "" || remote.ManifestDigest != "") {
		return true
	}
	return false
}

func localConfigDigests(local localImageInfo) []string {
	values := append([]string{}, local.ConfigDigests...)
	if local.ConfigDigest != "" {
		values = append(values, local.ConfigDigest)
	}
	return uniqueDigests(values)
}

func uniqueDigests(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		digest := digestOnly(value)
		if digest == "" || seen[digest] {
			continue
		}
		seen[digest] = true
		out = append(out, digest)
	}
	return out
}

func containsDigest(values []string, target string) bool {
	target = digestOnly(target)
	if target == "" {
		return false
	}
	for _, value := range values {
		if digestOnly(value) == target {
			return true
		}
	}
	return false
}

func allDigestsMatch(values []string, target string) bool {
	digests := uniqueDigests(values)
	target = digestOnly(target)
	if len(digests) == 0 || target == "" {
		return false
	}
	for _, digest := range digests {
		if digest != target {
			return false
		}
	}
	return true
}

func firstDigest(values []string, fallback string) string {
	for _, value := range values {
		if digest := digestOnly(value); digest != "" {
			return digest
		}
	}
	return digestOnly(fallback)
}

func shortDigest(value string) string {
	value = digestOnly(value)
	if len(value) > 19 {
		return value[:19]
	}
	if value == "" {
		return "-"
	}
	return value
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
