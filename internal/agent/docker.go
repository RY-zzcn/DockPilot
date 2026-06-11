package agent

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dockpilot/dockpilot/internal/protocol"
	"gopkg.in/yaml.v3"
)

type DockerClient struct {
	ComposeDirs []string
}

func (d DockerClient) DockerVersion(ctx context.Context) string {
	return strings.TrimSpace(commandOutput(ctx, "docker", "version", "--format", "{{.Server.Version}}"))
}

func (d DockerClient) ComposeVersion(ctx context.Context) string {
	return strings.TrimSpace(commandOutput(ctx, "docker", "compose", "version", "--short"))
}

func (d DockerClient) DaemonID(ctx context.Context) string {
	return strings.TrimSpace(commandOutput(ctx, "docker", "info", "--format", "{{.ID}}"))
}

func (d DockerClient) Snapshot(ctx context.Context) protocol.DockerSnapshotPayload {
	containers := d.containers(ctx)
	images := d.images(ctx)
	projects := d.composeProjects(ctx)
	return protocol.DockerSnapshotPayload{
		Containers:      containers,
		Images:          images,
		ComposeProjects: projects,
	}
}

func (d DockerClient) ContainerCount(ctx context.Context) int {
	out := commandOutput(ctx, "docker", "ps", "-q")
	if strings.TrimSpace(out) == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(out), "\n"))
}

func (d DockerClient) containers(ctx context.Context) []protocol.ContainerSnapshot {
	out := commandOutput(ctx, "docker", "ps", "-a", "--format", "{{json .}}")
	var containers []protocol.ContainerSnapshot
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw struct {
			ID     string `json:"ID"`
			Names  string `json:"Names"`
			Image  string `json:"Image"`
			State  string `json:"State"`
			Status string `json:"Status"`
			Labels string `json:"Labels"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		labels := parseLabels(raw.Labels)
		containers = append(containers, protocol.ContainerSnapshot{
			ID:             raw.ID,
			Name:           raw.Names,
			Image:          raw.Image,
			State:          raw.State,
			Status:         raw.Status,
			ComposeProject: labels["com.docker.compose.project"],
			Labels:         labels,
		})
	}
	return containers
}

func (d DockerClient) images(ctx context.Context) []protocol.ImageSnapshot {
	out := commandOutput(ctx, "docker", "images", "--format", "{{json .}}")
	var images []protocol.ImageSnapshot
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw struct {
			ID         string `json:"ID"`
			Repository string `json:"Repository"`
			Tag        string `json:"Tag"`
			Size       string `json:"Size"`
			CreatedAt  string `json:"CreatedAt"`
		}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		images = append(images, protocol.ImageSnapshot{
			ID:         raw.ID,
			Repository: raw.Repository,
			Tag:        raw.Tag,
			Size:       raw.Size,
			CreatedAt:  raw.CreatedAt,
		})
	}
	return images
}

func (d DockerClient) composeProjects(ctx context.Context) []protocol.ComposeProjectSnapshot {
	seen := map[string]protocol.ComposeProjectSnapshot{}
	for _, project := range d.composeProjectsFromCLI(ctx) {
		mergeComposeProject(seen, project)
	}
	for _, project := range d.composeProjectsFromDirs(ctx) {
		mergeComposeProject(seen, project)
	}
	var projects []protocol.ComposeProjectSnapshot
	for _, project := range seen {
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects
}

func mergeComposeProject(seen map[string]protocol.ComposeProjectSnapshot, project protocol.ComposeProjectSnapshot) {
	if project.ID == "" {
		return
	}
	if existing, ok := seen[project.ID]; ok {
		if existing.Content == "" && project.Content != "" {
			existing.Content = project.Content
			seen[project.ID] = existing
		}
		return
	}
	seen[project.ID] = project
}

func (d DockerClient) composeProjectsFromCLI(ctx context.Context) []protocol.ComposeProjectSnapshot {
	out := commandOutput(ctx, "docker", "compose", "ls", "--format", "json")
	var rawProjects []struct {
		Name        string `json:"Name"`
		Status      string `json:"Status"`
		ConfigFiles string `json:"ConfigFiles"`
	}
	if err := json.Unmarshal([]byte(out), &rawProjects); err != nil {
		return nil
	}
	var projects []protocol.ComposeProjectSnapshot
	for _, raw := range rawProjects {
		path := strings.Split(raw.ConfigFiles, ",")[0]
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		projects = append(projects, composeProject(raw.Name, path, false))
	}
	return projects
}

func (d DockerClient) composeProjectsFromDirs(ctx context.Context) []protocol.ComposeProjectSnapshot {
	var projects []protocol.ComposeProjectSnapshot
	for _, root := range d.ComposeDirs {
		if ctx.Err() != nil {
			break
		}
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "." || root == "" {
			continue
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if shouldSkipComposeScanDir(root, path, entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if !isComposeFileName(entry.Name()) {
				return nil
			}
			projects = append(projects, composeProject(composeProjectNameFromPath(path), path, false))
			return nil
		})
	}
	return projects
}

func shouldSkipComposeScanDir(root, path, name string) bool {
	if filepath.Clean(path) == filepath.Clean(root) {
		return false
	}
	if isKnownComposeTemplateDir(path) {
		return true
	}
	switch name {
	case ".git", ".hg", ".svn", "node_modules", "vendor", "dist", "build", "target", "__pycache__", ".cache", ".next", ".nuxt":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func isKnownComposeTemplateDir(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	templateDirs := []string{
		"/1panel/resource/apps/remote",
		"/1panel/resource/apps/local",
	}
	for _, dir := range templateDirs {
		if strings.Contains(clean, dir) {
			return true
		}
	}
	return false
}

func isComposeFileName(name string) bool {
	switch name {
	case "compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml":
		return true
	default:
		return false
	}
}

func composeProjectNameFromPath(path string) string {
	name := filepath.Base(filepath.Dir(path))
	if strings.TrimSpace(name) == "" || name == "." || name == string(filepath.Separator) {
		return "compose"
	}
	return name
}

func composeProject(name, path string, managed bool) protocol.ComposeProjectSnapshot {
	content := ""
	contentHash := composeFileHash(path)
	contentPreview := composeContentPreview(path)
	if !managed {
		// Scanned host projects often contain secrets in environment blocks.
		return protocol.ComposeProjectSnapshot{ID: stableID(path), Name: name, Path: path, Managed: managed, ContentHash: contentHash, ContentPreview: contentPreview}
	}
	if info, err := os.Stat(path); err == nil && info.Size() < 256*1024 {
		raw, _ := os.ReadFile(path)
		content = string(raw)
	}
	return protocol.ComposeProjectSnapshot{
		ID:             stableID(path),
		Name:           name,
		Path:           path,
		Managed:        managed,
		Content:        content,
		ContentHash:    contentHash,
		ContentPreview: contentPreview,
	}
}

func composeFileHash(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return ""
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func composeContentPreview(path string) string {
	info, err := os.Stat(path)
	if err != nil || info.Size() > 256*1024 {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return ""
	}
	doc := &root
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		doc = root.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return ""
	}
	services := mappingValue(doc, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return ""
	}
	var b strings.Builder
	b.WriteString("services:\n")
	wroteService := false
	for i := 0; i+1 < len(services.Content); i += 2 {
		name := services.Content[i].Value
		service := services.Content[i+1]
		if service.Kind != yaml.MappingNode {
			continue
		}
		fields := safeComposeServiceFields(service)
		if len(fields) == 0 {
			continue
		}
		wroteService = true
		b.WriteString("  ")
		b.WriteString(formatPreviewScalar(name))
		b.WriteString(":\n")
		for _, field := range fields {
			b.WriteString("    ")
			b.WriteString(field.name)
			b.WriteString(": ")
			b.WriteString(formatPreviewScalar(field.value))
			b.WriteByte('\n')
		}
	}
	if !wroteService {
		return ""
	}
	return b.String()
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func safeComposeServiceFields(service *yaml.Node) []struct{ name, value string } {
	allowed := []string{"image", "container_name", "restart"}
	fields := []struct{ name, value string }{}
	for _, key := range allowed {
		value := mappingValue(service, key)
		if value == nil || value.Kind != yaml.ScalarNode || strings.TrimSpace(value.Value) == "" {
			continue
		}
		fields = append(fields, struct{ name, value string }{name: key, value: value.Value})
	}
	return fields
}

func formatPreviewScalar(value string) string {
	if value == "" {
		return `""`
	}
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		switch ch {
		case '_', '-', '.', '/', ':', '@', '+':
			continue
		default:
			return strconv.Quote(value)
		}
	}
	return value
}

func commandOutput(ctx context.Context, name string, args ...string) string {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func parseLabels(value string) map[string]string {
	labels := map[string]string{}
	for _, item := range strings.Split(value, ",") {
		parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		}
	}
	return labels
}

func stableID(value string) string {
	hash := sha1.Sum([]byte(filepath.Clean(value)))
	return "compose_" + hex.EncodeToString(hash[:])[:16]
}

func composeFileArgs(path string) []string {
	if path == "" {
		return nil
	}
	file := composeFilePath(path)
	return []string{"--project-directory", composeProjectDir(file), "-f", file}
}

func composeProjectDir(path string) string {
	if path == "" {
		return "."
	}
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return path
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		if abs, err := filepath.Abs(dir); err == nil {
			return abs
		}
	}
	return dir
}

func composeFilePath(path string) string {
	if path == "" {
		return path
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return path
	}
	for _, name := range []string{"compose.yml", "compose.yaml", "docker-compose.yml", "docker-compose.yaml"} {
		candidate := filepath.Join(path, name)
		if stat, statErr := os.Stat(candidate); statErr == nil && !stat.IsDir() {
			return candidate
		}
	}
	return path
}

func commandTimeout() time.Duration {
	return 30 * time.Minute
}
