package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dockpilot/dockpilot/internal/protocol"
	_ "modernc.org/sqlite"
)

const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"

	PolicyManual    = "manual"
	PolicyScheduled = "scheduled"
	PolicyAutomatic = "automatic"

	TaskPending = "pending"
	TaskRunning = "running"
	TaskSuccess = "success"
	TaskFailed  = "failed"
	TaskCanceled = "canceled"
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	CreatedAt    string `json:"created_at"`
}

type Node struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Token          string `json:"-"`
	Version        string `json:"version"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	DockerVersion  string `json:"docker_version"`
	ComposeVersion string `json:"compose_version"`
	Status         string `json:"status"`
	LastSeen       string `json:"last_seen"`
	Labels         string `json:"labels"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type Metric struct {
	ID             int64   `json:"id"`
	NodeID         string  `json:"node_id"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsed    uint64  `json:"memory_used"`
	MemoryTotal   uint64  `json:"memory_total"`
	DiskUsed       uint64  `json:"disk_used"`
	DiskTotal      uint64  `json:"disk_total"`
	NetworkRx      uint64  `json:"network_rx"`
	NetworkTx      uint64  `json:"network_tx"`
	ContainerCount int     `json:"container_count"`
	RecordedAt     string  `json:"recorded_at"`
}

type Container struct {
	ID             string `json:"id"`
	NodeID         string `json:"node_id"`
	Name           string `json:"name"`
	Image          string `json:"image"`
	State          string `json:"state"`
	Status         string `json:"status"`
	ComposeProject string `json:"compose_project"`
	UpdateAvailable bool   `json:"update_available"`
	UpdatedAt       string `json:"updated_at"`
}

type Image struct {
	ID         string `json:"id"`
	NodeID     string `json:"node_id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       string `json:"size"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type ComposeProject struct {
	ID              string `json:"id"`
	NodeID          string `json:"node_id"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	Managed         bool   `json:"managed"`
	Content         string `json:"content,omitempty"`
	Version         int    `json:"version"`
	UpdateAvailable bool   `json:"update_available"`
	CheckedAt       string `json:"checked_at"`
	LastSeen        string `json:"last_seen"`
	UpdatedAt       string `json:"updated_at"`
}

type Task struct {
	ID          string `json:"id"`
	NodeID      string `json:"node_id"`
	Kind        string `json:"kind"`
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	Status      string `json:"status"`
	RequestedBy string `json:"requested_by"`
	PolicyID    string `json:"policy_id,omitempty"`
	Payload     string `json:"payload,omitempty"`
	Result      string `json:"result,omitempty"`
	CreatedAt   string `json:"created_at"`
	StartedAt   string `json:"started_at,omitempty"`
	FinishedAt  string `json:"finished_at,omitempty"`
}

type TaskLog struct {
	ID        int64  `json:"id"`
	TaskID    string `json:"task_id"`
	Line      string `json:"line"`
	CreatedAt string `json:"created_at"`
}

type Policy struct {
	ID              string `json:"id"`
	Scope           string `json:"scope"`
	ScopeID         string `json:"scope_id"`
	Mode            string `json:"mode"`
	Schedule        string `json:"schedule"`
	ExcludePatterns string `json:"exclude_patterns"`
	Enabled         bool   `json:"enabled"`
	UpdatedAt       string `json:"updated_at"`
}

type Notification struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Channel   string `json:"channel"`
	Config    string `json:"config"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Event struct {
	ID        int64  `json:"id"`
	Severity  string `json:"severity"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Payload   string `json:"payload"`
	CreatedAt string `json:"created_at"`
}

type Overview struct {
	NodesTotal       int64  `json:"nodes_total"`
	NodesOnline      int64  `json:"nodes_online"`
	ContainersTotal  int64  `json:"containers_total"`
	UpdatesAvailable int64  `json:"updates_available"`
	FailedTasks      int64  `json:"failed_tasks"`
	LastMetric        Metric `json:"last_metric"`
}

type DockerState struct {
	Containers      []Container      `json:"containers"`
	Images          []Image          `json:"images"`
	ComposeProjects []ComposeProject `json:"compose_projects"`
}

func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL CHECK(role IN ('admin','viewer')),
  created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS nodes (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  token TEXT NOT NULL UNIQUE,
  version TEXT NOT NULL DEFAULT '',
  os TEXT NOT NULL DEFAULT '',
  arch TEXT NOT NULL DEFAULT '',
  docker_version TEXT NOT NULL DEFAULT '',
  compose_version TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'offline',
  last_seen TEXT NOT NULL DEFAULT '',
  labels TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS node_metrics (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  node_id TEXT NOT NULL,
  cpu_percent REAL NOT NULL,
  memory_used INTEGER NOT NULL,
  memory_total INTEGER NOT NULL,
  disk_used INTEGER NOT NULL,
  disk_total INTEGER NOT NULL,
  network_rx INTEGER NOT NULL,
  network_tx INTEGER NOT NULL,
  container_count INTEGER NOT NULL,
  recorded_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
  FOREIGN KEY(node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS containers (
  id TEXT NOT NULL,
  node_id TEXT NOT NULL,
  name TEXT NOT NULL,
  image TEXT NOT NULL,
  state TEXT NOT NULL,
  status TEXT NOT NULL,
  compose_project TEXT NOT NULL DEFAULT '',
  update_available INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
  PRIMARY KEY(node_id, id),
  FOREIGN KEY(node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS images (
  id TEXT NOT NULL,
  node_id TEXT NOT NULL,
  repository TEXT NOT NULL,
  tag TEXT NOT NULL,
  size TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
  PRIMARY KEY(node_id, id),
  FOREIGN KEY(node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compose_projects (
  id TEXT NOT NULL,
  node_id TEXT NOT NULL,
  name TEXT NOT NULL,
  path TEXT NOT NULL,
  managed INTEGER NOT NULL DEFAULT 0,
  content TEXT NOT NULL DEFAULT '',
  version INTEGER NOT NULL DEFAULT 1,
  update_available INTEGER NOT NULL DEFAULT 0,
  checked_at TEXT NOT NULL DEFAULT '',
  last_seen TEXT NOT NULL DEFAULT (datetime('now','localtime')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
  PRIMARY KEY(node_id, id),
  FOREIGN KEY(node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS compose_revisions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id TEXT NOT NULL,
  node_id TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
  created_by TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
  id TEXT PRIMARY KEY,
  node_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  target_type TEXT NOT NULL DEFAULT '',
  target_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  requested_by TEXT NOT NULL DEFAULT '',
  policy_id TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}',
  result TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
  started_at TEXT NOT NULL DEFAULT '',
  finished_at TEXT NOT NULL DEFAULT '',
  FOREIGN KEY(node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS task_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id TEXT NOT NULL,
  line TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
  FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS policies (
  id TEXT PRIMARY KEY,
  scope TEXT NOT NULL,
  scope_id TEXT NOT NULL DEFAULT '',
  mode TEXT NOT NULL CHECK(mode IN ('manual','scheduled','automatic')),
  schedule TEXT NOT NULL DEFAULT '',
  exclude_patterns TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 1,
  updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
  UNIQUE(scope, scope_id)
);

CREATE TABLE IF NOT EXISTS notifications (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  channel TEXT NOT NULL,
  config TEXT NOT NULL DEFAULT '{}',
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  severity TEXT NOT NULL,
  type TEXT NOT NULL,
  message TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
);

CREATE INDEX IF NOT EXISTS idx_metrics_node_recorded ON node_metrics(node_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_node_created ON tasks(node_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_created ON events(created_at DESC);
`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	if err := s.ensureColumn("compose_projects", "update_available", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	return s.ensureColumn("compose_projects", "checked_at", "TEXT NOT NULL DEFAULT ''")
}

func (s *Store) ensureColumn(table, column, definition string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func (s *Store) EnsureAdmin(username, password string) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO users(username, password_hash, role, created_at) VALUES(?,?,?,datetime('now','localtime'))`, username, hash, RoleAdmin)
	return err
}

func (s *Store) CreateUser(username, password, role string) (User, error) {
	if role == "" {
		role = RoleViewer
	}
	hash, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	res, err := s.db.Exec(`INSERT INTO users(username, password_hash, role, created_at) VALUES(?,?,?,datetime('now','localtime'))`, username, hash, role)
	if err != nil {
		return User{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetUserByID(id)
}

func (s *Store) GetUserByID(id int64) (User, error) {
	var user User
	err := s.db.QueryRow(`SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?`, id).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)
	return user, err
}

func (s *Store) GetUserByUsername(username string) (User, error) {
	var user User
	err := s.db.QueryRow(`SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)
	return user, err
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, password_hash, role, created_at FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) UpsertNodeFromHello(hello protocol.HelloPayload, fallbackID string) (Node, bool, error) {
	created := false
	nodeID := hello.NodeID
	if nodeID == "" {
		nodeID = fallbackID
	}
	if nodeID == "" {
		nodeID = RandomToken("node_")
	}
	token := hello.NodeToken
	if token == "" {
		token = RandomToken("nt_")
		created = true
	}
	labels, _ := json.Marshal(hello.Labels)
	res, err := s.db.Exec(`
INSERT INTO nodes(id, name, token, version, os, arch, docker_version, compose_version, status, last_seen, labels, created_at, updated_at)
VALUES(?,?,?,?,?,?,?,?, 'online', datetime('now','localtime'), ?, datetime('now','localtime'), datetime('now','localtime'))
ON CONFLICT(id) DO UPDATE SET
  name = excluded.name,
  version = excluded.version,
  os = excluded.os,
  arch = excluded.arch,
  docker_version = excluded.docker_version,
  compose_version = excluded.compose_version,
  status = 'online',
  last_seen = datetime('now','localtime'),
  labels = excluded.labels,
  updated_at = datetime('now','localtime')
`, nodeID, nonEmpty(hello.Name, nodeID), token, hello.Version, hello.OS, hello.Arch, hello.DockerVersion, hello.ComposeVersion, string(labels))
	if err != nil {
		return Node{}, false, err
	}
	if affected, _ := res.RowsAffected(); affected == 1 && created {
		created = true
	}
	node, err := s.GetNode(nodeID)
	return node, created, err
}

func (s *Store) AuthenticateNode(nodeID, token string) (Node, error) {
	var node Node
	err := s.db.QueryRow(`SELECT id, name, token, version, os, arch, docker_version, compose_version, status, last_seen, labels, created_at, updated_at FROM nodes WHERE id = ? AND token = ?`, nodeID, token).
		Scan(&node.ID, &node.Name, &node.Token, &node.Version, &node.OS, &node.Arch, &node.DockerVersion, &node.ComposeVersion, &node.Status, &node.LastSeen, &node.Labels, &node.CreatedAt, &node.UpdatedAt)
	return node, err
}

func (s *Store) GetNode(id string) (Node, error) {
	var node Node
	err := s.db.QueryRow(`SELECT id, name, token, version, os, arch, docker_version, compose_version, status, last_seen, labels, created_at, updated_at FROM nodes WHERE id = ?`, id).
		Scan(&node.ID, &node.Name, &node.Token, &node.Version, &node.OS, &node.Arch, &node.DockerVersion, &node.ComposeVersion, &node.Status, &node.LastSeen, &node.Labels, &node.CreatedAt, &node.UpdatedAt)
	return node, err
}

func (s *Store) ListNodes() ([]Node, error) {
	rows, err := s.db.Query(`SELECT id, name, token, version, os, arch, docker_version, compose_version, status, last_seen, labels, created_at, updated_at FROM nodes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := []Node{}
	for rows.Next() {
		var node Node
		if err := rows.Scan(&node.ID, &node.Name, &node.Token, &node.Version, &node.OS, &node.Arch, &node.DockerVersion, &node.ComposeVersion, &node.Status, &node.LastSeen, &node.Labels, &node.CreatedAt, &node.UpdatedAt); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

func (s *Store) MarkNodeSeen(nodeID, status string) error {
	_, err := s.db.Exec(`UPDATE nodes SET status = ?, last_seen = datetime('now','localtime'), updated_at = datetime('now','localtime') WHERE id = ?`, status, nodeID)
	return err
}

func (s *Store) MarkStaleNodesOffline(timeout time.Duration) ([]Node, error) {
	cutoff := time.Now().In(time.Local).Add(-timeout).Format("2006-01-02 15:04:05")
	rows, err := s.db.Query(`SELECT id, name, token, version, os, arch, docker_version, compose_version, status, last_seen, labels, created_at, updated_at FROM nodes WHERE status = 'online' AND last_seen < ?`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stale := []Node{}
	for rows.Next() {
		var node Node
		if err := rows.Scan(&node.ID, &node.Name, &node.Token, &node.Version, &node.OS, &node.Arch, &node.DockerVersion, &node.ComposeVersion, &node.Status, &node.LastSeen, &node.Labels, &node.CreatedAt, &node.UpdatedAt); err != nil {
			return nil, err
		}
		stale = append(stale, node)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	_, err = s.db.Exec(`UPDATE nodes SET status = 'offline', updated_at = datetime('now','localtime') WHERE status = 'online' AND last_seen < ?`, cutoff)
	return stale, err
}

func (s *Store) InsertMetrics(nodeID string, metric protocol.MetricsPayload) error {
	_, err := s.db.Exec(`
INSERT INTO node_metrics(node_id, cpu_percent, memory_used, memory_total, disk_used, disk_total, network_rx, network_tx, container_count, recorded_at)
VALUES(?,?,?,?,?,?,?,?,?,datetime('now','localtime'))`, nodeID, metric.CPUPercent, metric.MemoryUsed, metric.MemoryTotal, metric.DiskUsed, metric.DiskTotal, metric.NetworkRx, metric.NetworkTx, metric.ContainerCount)
	return err
}

func (s *Store) ReplaceDockerSnapshot(nodeID string, snapshot protocol.DockerSnapshotPayload) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	previousByID := map[string]bool{}
	previousByImage := map[string]bool{}
	rows, err := tx.Query(`SELECT id, image, compose_project, update_available FROM containers WHERE node_id = ?`, nodeID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, image, composeProject string
		var available bool
		if err := rows.Scan(&id, &image, &composeProject, boolScanner(&available)); err != nil {
			rows.Close()
			return err
		}
		if available {
			previousByID[id] = true
			previousByImage[composeProject+"\x00"+image] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if _, err := tx.Exec(`DELETE FROM containers WHERE node_id = ?`, nodeID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM images WHERE node_id = ?`, nodeID); err != nil {
		return err
	}
	for _, container := range snapshot.Containers {
		updateAvailable := container.UpdateAvailable || previousByID[container.ID] || previousByImage[container.ComposeProject+"\x00"+container.Image]
		_, err := tx.Exec(`
INSERT INTO containers(id, node_id, name, image, state, status, compose_project, update_available, updated_at)
VALUES(?,?,?,?,?,?,?,?,datetime('now','localtime'))`,
			container.ID, nodeID, container.Name, container.Image, container.State, container.Status, container.ComposeProject, boolInt(updateAvailable))
		if err != nil {
			return err
		}
	}
	for _, image := range snapshot.Images {
		_, err := tx.Exec(`
INSERT INTO images(id, node_id, repository, tag, size, created_at, updated_at)
VALUES(?,?,?,?,?,?,datetime('now','localtime'))`,
			image.ID, nodeID, image.Repository, image.Tag, image.Size, image.CreatedAt)
		if err != nil {
			return err
		}
	}
	for _, project := range snapshot.ComposeProjects {
		_, err := tx.Exec(`
INSERT INTO compose_projects(id, node_id, name, path, managed, content, last_seen, updated_at)
VALUES(?,?,?,?,?,?,datetime('now','localtime'),datetime('now','localtime'))
ON CONFLICT(node_id, id) DO UPDATE SET
  name = excluded.name,
  path = excluded.path,
  content = CASE WHEN compose_projects.managed = 1 THEN compose_projects.content ELSE excluded.content END,
  managed = CASE WHEN compose_projects.managed = 1 THEN 1 ELSE excluded.managed END,
  last_seen = datetime('now','localtime'),
  updated_at = datetime('now','localtime')`,
			project.ID, nodeID, project.Name, project.Path, boolInt(project.Managed), project.Content)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ApplyUpdateDetections(nodeID string, detections []protocol.UpdateDetection) (int, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	availableCount := 0
	for _, detection := range detections {
		projectAvailable := false
		for _, image := range detection.Images {
			if image.UpdateAvailable {
				projectAvailable = true
				availableCount++
			}
		}
		if detection.TargetID != "" {
			if _, err := tx.Exec(`
UPDATE compose_projects
SET update_available = ?, checked_at = datetime('now','localtime'), updated_at = datetime('now','localtime')
WHERE node_id = ? AND id = ?`, boolInt(projectAvailable), nodeID, detection.TargetID); err != nil {
				return 0, err
			}
		} else if detection.Path != "" {
			if _, err := tx.Exec(`
UPDATE compose_projects
SET update_available = ?, checked_at = datetime('now','localtime'), updated_at = datetime('now','localtime')
WHERE node_id = ? AND path = ?`, boolInt(projectAvailable), nodeID, detection.Path); err != nil {
				return 0, err
			}
		}
		if detection.ProjectName != "" {
			if _, err := tx.Exec(`UPDATE containers SET update_available = 0, updated_at = datetime('now','localtime') WHERE node_id = ? AND compose_project = ?`, nodeID, detection.ProjectName); err != nil {
				return 0, err
			}
		}
		for _, image := range detection.Images {
			if !image.UpdateAvailable {
				continue
			}
			if detection.ProjectName != "" {
				if _, err := tx.Exec(`UPDATE containers SET update_available = 1, updated_at = datetime('now','localtime') WHERE node_id = ? AND compose_project = ? AND image = ?`, nodeID, detection.ProjectName, image.Image); err != nil {
					return 0, err
				}
			} else {
				if _, err := tx.Exec(`UPDATE containers SET update_available = 1, updated_at = datetime('now','localtime') WHERE node_id = ? AND image = ?`, nodeID, image.Image); err != nil {
					return 0, err
				}
			}
		}
	}
	return availableCount, tx.Commit()
}

func (s *Store) ClearUpdateAvailabilityForTask(task Task) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	switch task.TargetType {
	case "compose":
		projectName := taskPayloadArg(task.Payload, "name")
		if task.TargetID != "" {
			_ = tx.QueryRow(`SELECT name FROM compose_projects WHERE node_id = ? AND id = ?`, task.NodeID, task.TargetID).Scan(&projectName)
			if _, err := tx.Exec(`
UPDATE compose_projects
SET update_available = 0, checked_at = datetime('now','localtime'), updated_at = datetime('now','localtime')
WHERE node_id = ? AND id = ?`, task.NodeID, task.TargetID); err != nil {
				return err
			}
		}
		if projectName != "" {
			if _, err := tx.Exec(`UPDATE containers SET update_available = 0, updated_at = datetime('now','localtime') WHERE node_id = ? AND compose_project = ?`, task.NodeID, projectName); err != nil {
				return err
			}
		}
	case "container":
		if task.TargetID != "" {
			if _, err := tx.Exec(`UPDATE containers SET update_available = 0, updated_at = datetime('now','localtime') WHERE node_id = ? AND id = ?`, task.NodeID, task.TargetID); err != nil {
				return err
			}
		}
	default:
		if _, err := tx.Exec(`UPDATE compose_projects SET update_available = 0, checked_at = datetime('now','localtime'), updated_at = datetime('now','localtime') WHERE node_id = ?`, task.NodeID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE containers SET update_available = 0, updated_at = datetime('now','localtime') WHERE node_id = ?`, task.NodeID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) DockerState(nodeID string) (DockerState, error) {
	state := DockerState{
		Containers:      []Container{},
		Images:          []Image{},
		ComposeProjects: []ComposeProject{},
	}
	containers, err := queryRows(s.db, `SELECT id, node_id, name, image, state, status, compose_project, update_available, updated_at FROM containers WHERE node_id = ? ORDER BY name`, scanContainer, nodeID)
	if err != nil {
		return state, err
	}
	images, err := queryRows(s.db, `SELECT id, node_id, repository, tag, size, created_at, updated_at FROM images WHERE node_id = ? ORDER BY repository, tag`, scanImage, nodeID)
	if err != nil {
		return state, err
	}
	projects, err := queryRows(s.db, `SELECT id, node_id, name, path, managed, content, version, update_available, checked_at, last_seen, updated_at FROM compose_projects WHERE node_id = ? ORDER BY name`, scanComposeProject, nodeID)
	if err != nil {
		return state, err
	}
	state.Containers = containers
	state.Images = images
	state.ComposeProjects = projects
	return state, nil
}

func (s *Store) SaveComposeProject(nodeID, projectID, name, path, content, username string) (ComposeProject, error) {
	if projectID == "" {
		projectID = RandomToken("compose_")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return ComposeProject{}, err
	}
	defer tx.Rollback()
	var oldContent string
	_ = tx.QueryRow(`SELECT content FROM compose_projects WHERE node_id = ? AND id = ?`, nodeID, projectID).Scan(&oldContent)
	if oldContent != "" {
		if _, err := tx.Exec(`INSERT INTO compose_revisions(project_id, node_id, content, created_at, created_by) VALUES(?,?,?,datetime('now','localtime'),?)`, projectID, nodeID, oldContent, username); err != nil {
			return ComposeProject{}, err
		}
	}
	_, err = tx.Exec(`
INSERT INTO compose_projects(id, node_id, name, path, managed, content, version, last_seen, updated_at)
VALUES(?,?,?,?,1,?,1,datetime('now','localtime'),datetime('now','localtime'))
ON CONFLICT(node_id, id) DO UPDATE SET
  name = excluded.name,
  path = excluded.path,
  managed = 1,
  content = excluded.content,
  version = compose_projects.version + 1,
  updated_at = datetime('now','localtime')`,
		projectID, nodeID, name, path, content)
	if err != nil {
		return ComposeProject{}, err
	}
	if err := tx.Commit(); err != nil {
		return ComposeProject{}, err
	}
	return s.GetComposeProject(nodeID, projectID)
}

func (s *Store) GetComposeProject(nodeID, projectID string) (ComposeProject, error) {
	var project ComposeProject
	err := s.db.QueryRow(`SELECT id, node_id, name, path, managed, content, version, update_available, checked_at, last_seen, updated_at FROM compose_projects WHERE node_id = ? AND id = ?`, nodeID, projectID).
		Scan(&project.ID, &project.NodeID, &project.Name, &project.Path, boolScanner(&project.Managed), &project.Content, &project.Version, boolScanner(&project.UpdateAvailable), &project.CheckedAt, &project.LastSeen, &project.UpdatedAt)
	return project, err
}

func (s *Store) CreateTask(task Task) (Task, error) {
	if task.ID == "" {
		task.ID = RandomToken("task_")
	}
	if task.Status == "" {
		task.Status = TaskPending
	}
	if task.Payload == "" {
		task.Payload = "{}"
	}
	_, err := s.db.Exec(`
INSERT INTO tasks(id, node_id, kind, target_type, target_id, status, requested_by, policy_id, payload, created_at)
VALUES(?,?,?,?,?,?,?,?,?,datetime('now','localtime'))`,
		task.ID, task.NodeID, task.Kind, task.TargetType, task.TargetID, task.Status, task.RequestedBy, task.PolicyID, task.Payload)
	if err != nil {
		return Task{}, err
	}
	return s.GetTask(task.ID)
}

func (s *Store) GetTask(id string) (Task, error) {
	var task Task
	err := s.db.QueryRow(`SELECT id, node_id, kind, target_type, target_id, status, requested_by, policy_id, payload, result, created_at, started_at, finished_at FROM tasks WHERE id = ?`, id).
		Scan(&task.ID, &task.NodeID, &task.Kind, &task.TargetType, &task.TargetID, &task.Status, &task.RequestedBy, &task.PolicyID, &task.Payload, &task.Result, &task.CreatedAt, &task.StartedAt, &task.FinishedAt)
	return task, err
}

func (s *Store) ListTasks(limit int) ([]Task, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, node_id, kind, target_type, target_id, status, requested_by, policy_id, payload, result, created_at, started_at, finished_at FROM tasks ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.NodeID, &task.Kind, &task.TargetType, &task.TargetID, &task.Status, &task.RequestedBy, &task.PolicyID, &task.Payload, &task.Result, &task.CreatedAt, &task.StartedAt, &task.FinishedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) PendingTasksForNode(nodeID string) ([]Task, error) {
	rows, err := s.db.Query(`SELECT id, node_id, kind, target_type, target_id, status, requested_by, policy_id, payload, result, created_at, started_at, finished_at FROM tasks WHERE node_id = ? AND status = ? ORDER BY created_at`, nodeID, TaskPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []Task{}
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.NodeID, &task.Kind, &task.TargetType, &task.TargetID, &task.Status, &task.RequestedBy, &task.PolicyID, &task.Payload, &task.Result, &task.CreatedAt, &task.StartedAt, &task.FinishedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) MarkTaskRunning(id string) error {
	_, err := s.db.Exec(`UPDATE tasks SET status = ?, started_at = CASE WHEN started_at = '' THEN datetime('now','localtime') ELSE started_at END WHERE id = ?`, TaskRunning, id)
	return err
}

func (s *Store) FinishTask(id, status, result string) error {
	_, err := s.db.Exec(`UPDATE tasks SET status = ?, result = ?, finished_at = datetime('now','localtime') WHERE id = ?`, status, result, id)
	return err
}

func (s *Store) AddTaskLog(taskID, line string) error {
	_, err := s.db.Exec(`INSERT INTO task_logs(task_id, line, created_at) VALUES(?,?,datetime('now','localtime'))`, taskID, line)
	return err
}

func (s *Store) TaskLogs(taskID string) ([]TaskLog, error) {
	rows, err := s.db.Query(`SELECT id, task_id, line, created_at FROM task_logs WHERE task_id = ? ORDER BY id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := []TaskLog{}
	for rows.Next() {
		var log TaskLog
		if err := rows.Scan(&log.ID, &log.TaskID, &log.Line, &log.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (s *Store) ListPolicies() ([]Policy, error) {
	rows, err := s.db.Query(`SELECT id, scope, scope_id, mode, schedule, exclude_patterns, enabled, updated_at FROM policies ORDER BY scope, scope_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	policies := []Policy{}
	for rows.Next() {
		var policy Policy
		if err := rows.Scan(&policy.ID, &policy.Scope, &policy.ScopeID, &policy.Mode, &policy.Schedule, &policy.ExcludePatterns, boolScanner(&policy.Enabled), &policy.UpdatedAt); err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	return policies, rows.Err()
}

func (s *Store) getPolicy(scope, scopeID string) (Policy, error) {
	var policy Policy
	err := s.db.QueryRow(`SELECT id, scope, scope_id, mode, schedule, exclude_patterns, enabled, updated_at FROM policies WHERE scope = ? AND scope_id = ?`, scope, scopeID).
		Scan(&policy.ID, &policy.Scope, &policy.ScopeID, &policy.Mode, &policy.Schedule, &policy.ExcludePatterns, boolScanner(&policy.Enabled), &policy.UpdatedAt)
	return policy, err
}

func (s *Store) UpsertPolicy(policy Policy) (Policy, error) {
	if policy.ID == "" {
		policy.ID = RandomToken("pol_")
	}
	if policy.Mode == "" {
		policy.Mode = PolicyManual
	}
	_, err := s.db.Exec(`
INSERT INTO policies(id, scope, scope_id, mode, schedule, exclude_patterns, enabled, updated_at)
VALUES(?,?,?,?,?,?,?,datetime('now','localtime'))
ON CONFLICT(scope, scope_id) DO UPDATE SET
  mode = excluded.mode,
  schedule = excluded.schedule,
  exclude_patterns = excluded.exclude_patterns,
  enabled = excluded.enabled,
  updated_at = datetime('now','localtime')`,
		policy.ID, policy.Scope, policy.ScopeID, policy.Mode, policy.Schedule, policy.ExcludePatterns, boolInt(policy.Enabled))
	if err != nil {
		return Policy{}, err
	}
	return s.ResolvePolicy(policy.Scope, policy.ScopeID)
}

func (s *Store) ResolvePolicy(scope, scopeID string) (Policy, error) {
	policy, err := s.getPolicy(scope, scopeID)
	if err == nil {
		return policy, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Policy{}, err
	}
	if scope != "global" {
		return s.ResolvePolicy("global", "")
	}
	return Policy{
		ID:      "default",
		Scope:   "global",
		Mode:    PolicyManual,
		Enabled: true,
	}, nil
}

func (s *Store) EffectivePolicy(containerID, composeID, nodeID string) (Policy, error) {
	for _, candidate := range []struct {
		scope string
		id    string
	}{
		{"container", containerID},
		{"compose", composeID},
		{"node", nodeID},
		{"global", ""},
	} {
		if candidate.id == "" && candidate.scope != "global" {
			continue
		}
		policy, err := s.getPolicy(candidate.scope, candidate.id)
		if err == nil && policy.Enabled {
			return policy, nil
		}
	}
	return s.ResolvePolicy("global", "")
}

func (s *Store) ListNotifications() ([]Notification, error) {
	rows, err := s.db.Query(`SELECT id, name, channel, config, enabled, created_at, updated_at FROM notifications ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notifications := []Notification{}
	for rows.Next() {
		var notification Notification
		if err := rows.Scan(&notification.ID, &notification.Name, &notification.Channel, &notification.Config, boolScanner(&notification.Enabled), &notification.CreatedAt, &notification.UpdatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func (s *Store) UpsertNotification(notification Notification) (Notification, error) {
	if notification.ID == "" {
		notification.ID = RandomToken("not_")
	}
	if notification.Config == "" {
		notification.Config = "{}"
	}
	_, err := s.db.Exec(`
INSERT INTO notifications(id, name, channel, config, enabled, created_at, updated_at)
VALUES(?,?,?,?,?,datetime('now','localtime'),datetime('now','localtime'))
ON CONFLICT(id) DO UPDATE SET
  name = excluded.name,
  channel = excluded.channel,
  config = excluded.config,
  enabled = excluded.enabled,
  updated_at = datetime('now','localtime')`,
		notification.ID, notification.Name, notification.Channel, notification.Config, boolInt(notification.Enabled))
	if err != nil {
		return Notification{}, err
	}
	var saved Notification
	err = s.db.QueryRow(`SELECT id, name, channel, config, enabled, created_at, updated_at FROM notifications WHERE id = ?`, notification.ID).
		Scan(&saved.ID, &saved.Name, &saved.Channel, &saved.Config, boolScanner(&saved.Enabled), &saved.CreatedAt, &saved.UpdatedAt)
	return saved, err
}

func (s *Store) EnabledNotifications() ([]Notification, error) {
	rows, err := s.db.Query(`SELECT id, name, channel, config, enabled, created_at, updated_at FROM notifications WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notifications := []Notification{}
	for rows.Next() {
		var notification Notification
		if err := rows.Scan(&notification.ID, &notification.Name, &notification.Channel, &notification.Config, boolScanner(&notification.Enabled), &notification.CreatedAt, &notification.UpdatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func (s *Store) AddEvent(severity, eventType, message string, payload any) error {
	raw, _ := json.Marshal(payload)
	_, err := s.db.Exec(`INSERT INTO events(severity, type, message, payload, created_at) VALUES(?,?,?,?,datetime('now','localtime'))`, severity, eventType, message, string(raw))
	return err
}

func (s *Store) Overview() (Overview, error) {
	var overview Overview
	queries := []struct {
		target *int64
		sql    string
	}{
		{&overview.NodesTotal, `SELECT COUNT(*) FROM nodes`},
		{&overview.NodesOnline, `SELECT COUNT(*) FROM nodes WHERE status = 'online'`},
		{&overview.ContainersTotal, `SELECT COUNT(*) FROM containers`},
		{&overview.UpdatesAvailable, `SELECT COUNT(*) FROM containers WHERE update_available = 1`},
		{&overview.FailedTasks, `SELECT COUNT(*) FROM tasks WHERE status = 'failed'`},
	}
	for _, query := range queries {
		if err := s.db.QueryRow(query.sql).Scan(query.target); err != nil {
			return overview, err
		}
	}
	_ = s.db.QueryRow(`SELECT id, node_id, cpu_percent, memory_used, memory_total, disk_used, disk_total, network_rx, network_tx, container_count, recorded_at FROM node_metrics ORDER BY recorded_at DESC LIMIT 1`).
		Scan(&overview.LastMetric.ID, &overview.LastMetric.NodeID, &overview.LastMetric.CPUPercent, &overview.LastMetric.MemoryUsed, &overview.LastMetric.MemoryTotal, &overview.LastMetric.DiskUsed, &overview.LastMetric.DiskTotal, &overview.LastMetric.NetworkRx, &overview.LastMetric.NetworkTx, &overview.LastMetric.ContainerCount, &overview.LastMetric.RecordedAt)
	return overview, nil
}

func queryRows[T any](db *sql.DB, query string, scan func(*sql.Rows) (T, error), args ...any) ([]T, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanContainer(rows *sql.Rows) (Container, error) {
	var c Container
	err := rows.Scan(&c.ID, &c.NodeID, &c.Name, &c.Image, &c.State, &c.Status, &c.ComposeProject, boolScanner(&c.UpdateAvailable), &c.UpdatedAt)
	return c, err
}

func scanImage(rows *sql.Rows) (Image, error) {
	var image Image
	err := rows.Scan(&image.ID, &image.NodeID, &image.Repository, &image.Tag, &image.Size, &image.CreatedAt, &image.UpdatedAt)
	return image, err
}

func scanComposeProject(rows *sql.Rows) (ComposeProject, error) {
	var project ComposeProject
	err := rows.Scan(&project.ID, &project.NodeID, &project.Name, &project.Path, boolScanner(&project.Managed), &project.Content, &project.Version, boolScanner(&project.UpdateAvailable), &project.CheckedAt, &project.LastSeen, &project.UpdatedAt)
	return project, err
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func taskPayloadArg(payload, key string) string {
	args := map[string]string{}
	_ = json.Unmarshal([]byte(payload), &args)
	return args[key]
}

type boolScanTarget struct {
	value *bool
}

func boolScanner(value *bool) sql.Scanner {
	return boolScanTarget{value: value}
}

func (b boolScanTarget) Scan(src any) error {
	switch v := src.(type) {
	case bool:
		*b.value = v
	case int64:
		*b.value = v != 0
	case int:
		*b.value = v != 0
	case []byte:
		*b.value = string(v) == "1" || strings.EqualFold(string(v), "true")
	case string:
		*b.value = v == "1" || strings.EqualFold(v, "true")
	default:
		return fmt.Errorf("cannot scan bool from %T", src)
	}
	return nil
}
