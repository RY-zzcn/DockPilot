package protocol

import (
	"encoding/json"
	"time"
)

const (
	TypeHello          = "hello"
	TypeHeartbeat      = "heartbeat"
	TypeMetrics        = "metrics"
	TypeDockerSnapshot = "docker_snapshot"
	TypeTaskRequest    = "task_request"
	TypeTaskLog        = "task_log"
	TypeTaskResult     = "task_result"
	TypeError          = "error"
)

type Message struct {
	Type      string          `json:"type"`
	NodeID    string          `json:"node_id,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func NewMessage(messageType string, nodeID string, payload any) (Message, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	return Message{
		Type:      messageType,
		NodeID:    nodeID,
		Timestamp: time.Now().UTC(),
		Payload:   raw,
	}, nil
}

func DecodePayload[T any](msg Message) (T, error) {
	var out T
	if len(msg.Payload) == 0 {
		return out, nil
	}
	err := json.Unmarshal(msg.Payload, &out)
	return out, err
}

type HelloPayload struct {
	NodeID            string            `json:"node_id,omitempty"`
	NodeToken         string            `json:"node_token,omitempty"`
	RegistrationToken string            `json:"registration_token,omitempty"`
	Name              string            `json:"name"`
	Version           string            `json:"version"`
	OS                string            `json:"os"`
	Arch              string            `json:"arch"`
	DockerVersion     string            `json:"docker_version,omitempty"`
	ComposeVersion    string            `json:"compose_version,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

type HelloAckPayload struct {
	NodeID    string `json:"node_id"`
	NodeToken string `json:"node_token"`
	Message   string `json:"message"`
}

type HeartbeatPayload struct {
	UptimeSeconds int64  `json:"uptime_seconds"`
	Status        string `json:"status"`
}

type MetricsPayload struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsed    uint64  `json:"memory_used"`
	MemoryTotal   uint64  `json:"memory_total"`
	DiskUsed      uint64  `json:"disk_used"`
	DiskTotal     uint64  `json:"disk_total"`
	NetworkRx     uint64  `json:"network_rx"`
	NetworkTx     uint64  `json:"network_tx"`
	ContainerCount int     `json:"container_count"`
}

type DockerSnapshotPayload struct {
	Containers      []ContainerSnapshot      `json:"containers"`
	Images          []ImageSnapshot          `json:"images"`
	ComposeProjects []ComposeProjectSnapshot `json:"compose_projects"`
}

type ContainerSnapshot struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Image          string            `json:"image"`
	State          string            `json:"state"`
	Status         string            `json:"status"`
	ComposeProject string            `json:"compose_project,omitempty"`
	UpdateAvailable bool             `json:"update_available,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

type ImageSnapshot struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Size       string `json:"size"`
	CreatedAt  string `json:"created_at"`
}

type ComposeProjectSnapshot struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Managed bool   `json:"managed"`
	Content string `json:"content,omitempty"`
}

type UpdateDetection struct {
	TargetType  string                 `json:"target_type,omitempty"`
	TargetID    string                 `json:"target_id,omitempty"`
	ProjectName string                 `json:"project_name,omitempty"`
	Path        string                 `json:"path,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Images      []ImageUpdateDetection `json:"images"`
}

type ImageUpdateDetection struct {
	Image                string `json:"image"`
	Platform             string `json:"platform,omitempty"`
	Method               string `json:"method,omitempty"`
	LocalDigest          string `json:"local_digest,omitempty"`
	RemoteDigest         string `json:"remote_digest,omitempty"`
	LocalConfigDigest    string `json:"local_config_digest,omitempty"`
	RemoteConfigDigest   string `json:"remote_config_digest,omitempty"`
	RemoteManifestDigest string `json:"remote_manifest_digest,omitempty"`
	Pinned               bool   `json:"pinned,omitempty"`
	CheckedAt            string `json:"checked_at,omitempty"`
	UpdateAvailable      bool   `json:"update_available"`
	Error                string `json:"error,omitempty"`
}

type TaskPayload struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	TargetType string            `json:"target_type,omitempty"`
	TargetID   string            `json:"target_id,omitempty"`
	Args       map[string]string `json:"args,omitempty"`
}

type TaskLogPayload struct {
	TaskID string `json:"task_id"`
	Line   string `json:"line"`
}

type TaskResultPayload struct {
	TaskID       string            `json:"task_id"`
	Status       string            `json:"status"`
	Message      string            `json:"message,omitempty"`
	ExitCode     int               `json:"exit_code"`
	Updates      []UpdateDetection `json:"updates,omitempty"`
	RestartAgent bool              `json:"restart_agent,omitempty"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
