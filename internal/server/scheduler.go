package server

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/dockpilot/dockpilot/internal/version"
)

type Scheduler struct {
	store            *Store
	hub              *AgentHub
	notifier         *Notifier
	releases         *ReleaseService
	cfg              Config
	heartbeatTimeout time.Duration
	lastRun          map[string]time.Time
}

func NewScheduler(store *Store, hub *AgentHub, notifier *Notifier, releases *ReleaseService, cfg Config) *Scheduler {
	return &Scheduler{
		store:            store,
		hub:              hub,
		notifier:         notifier,
		releases:         releases,
		cfg:              cfg,
		heartbeatTimeout: cfg.HeartbeatTimeout,
		lastRun:          map[string]time.Time{},
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	s.tick()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.tick()
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) tick() {
	stale, err := s.store.MarkStaleNodesOffline(s.heartbeatTimeout)
	if err != nil {
		log.Printf("mark stale nodes failed: %v", err)
	}
	for _, node := range stale {
		s.notifier.Notify("warning", "节点离线", "节点 "+node.Name+" 超过心跳窗口未上报")
	}
	if err := s.enqueuePolicyTasks(); err != nil {
		log.Printf("enqueue policy tasks failed: %v", err)
	}
	if err := s.enqueueAgentUpdateTasks(); err != nil {
		log.Printf("enqueue agent update tasks failed: %v", err)
	}
	if err := s.store.PruneTaskHistory(); err != nil {
		log.Printf("prune task history failed: %v", err)
	}
}

func (s *Scheduler) enqueuePolicyTasks() error {
	nodes, err := s.store.ListNodes()
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.Status != "online" {
			continue
		}
		state, err := s.store.DockerState(node.ID)
		if err != nil {
			continue
		}
		for _, project := range state.ComposeProjects {
			if s.detectionDue(node.ID, project.ID) {
				task, err := s.createComposeTask(node, project.ID, project.Name, project.Path, "detect_updates", "scheduler", "")
				if err == nil {
					s.lastRun[detectionKey(node.ID, project.ID)] = time.Now()
					_ = s.hub.EnqueueTask(task)
				}
			}

			if !project.UpdateAvailable {
				continue
			}
			policy, err := s.store.EffectivePolicy("", project.ID, node.ID)
			if err != nil || !policy.Enabled || policy.Mode == PolicyManual || excluded(policy.ExcludePatterns, project.Name, project.Path) {
				continue
			}
			key := "compose-update:" + policy.ID + ":" + node.ID + ":" + project.ID
			if !due(policy, s.lastRun[key]) {
				continue
			}
			task, err := s.createComposeTask(node, project.ID, project.Name, project.Path, "compose_update", "scheduler", policy.ID)
			if err != nil {
				continue
			}
			s.lastRun[key] = time.Now()
			_ = s.hub.EnqueueTask(task)
		}
	}
	return nil
}

func (s *Scheduler) detectionDue(nodeID, projectID string) bool {
	return due(Policy{Schedule: DefaultDetectionSchedule}, s.lastRun[detectionKey(nodeID, projectID)])
}

func detectionKey(nodeID, projectID string) string {
	return "detect:" + nodeID + ":" + projectID
}

func (s *Scheduler) createComposeTask(node Node, projectID, projectName, projectPath, kind, requestedBy, policyID string) (Task, error) {
	args := map[string]string{"path": projectPath, "name": projectName}
	payload, _ := json.Marshal(args)
	return s.store.CreateTask(Task{
		NodeID:      node.ID,
		Kind:        kind,
		TargetType:  "compose",
		TargetID:    projectID,
		RequestedBy: requestedBy,
		PolicyID:    policyID,
		Payload:     string(payload),
	})
}

func (s *Scheduler) enqueueAgentUpdateTasks() error {
	settings := s.agentUpdateSettings()
	if !settings.enabled {
		return nil
	}
	key := "agent-update:scan"
	if !s.lastRun[key].IsZero() && time.Since(s.lastRun[key]) < s.cfg.AgentAutoUpdateInterval {
		return nil
	}
	s.lastRun[key] = time.Now()

	targetVersion := version.Clean(settings.targetVersion)
	if targetVersion == "" || targetVersion == "latest" {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		release := s.releases.Latest(ctx, "", false)
		cancel()
		if release.Error != "" && release.LatestVersion == "" {
			return nil
		}
		targetVersion = release.LatestVersion
	}
	if targetVersion == "" {
		return nil
	}
	nodes, err := s.store.ListNodes()
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.Status != "online" || !version.IsOutdated(node.Version, targetVersion) {
			continue
		}
		nodeKey := "agent-update:" + node.ID + ":" + targetVersion
		if !s.lastRun[nodeKey].IsZero() && time.Since(s.lastRun[nodeKey]) < s.cfg.AgentAutoUpdateInterval {
			continue
		}
		args := map[string]string{
			"version":    targetVersion,
			"repo":       s.cfg.ReleaseRepo,
			"server_url": s.cfg.PublicURL,
		}
		payload, _ := json.Marshal(args)
		task, err := s.store.CreateTask(Task{
			NodeID:      node.ID,
			Kind:        "agent_update",
			TargetType:  "node",
			TargetID:    node.ID,
			RequestedBy: "agent-auto-update",
			Payload:     string(payload),
		})
		if err != nil {
			continue
		}
		s.lastRun[nodeKey] = time.Now()
		_ = s.hub.EnqueueTask(task)
	}
	return nil
}

type agentUpdateSettings struct {
	enabled       bool
	targetVersion string
}

func (s *Scheduler) agentUpdateSettings() agentUpdateSettings {
	return agentUpdateSettings{
		enabled:       parseStoredBool(s.store.Setting("agent_auto_update", ""), s.cfg.AgentAutoUpdate),
		targetVersion: nonEmpty(s.store.Setting("agent_auto_update_version", s.cfg.AgentAutoUpdateVersion), "latest"),
	}
}

func due(policy Policy, last time.Time) bool {
	if last.IsZero() {
		return true
	}
	interval := 24 * time.Hour
	switch strings.TrimSpace(policy.Schedule) {
	case "@hourly":
		interval = time.Hour
	case "@daily", "":
		interval = 24 * time.Hour
	default:
		if strings.HasPrefix(policy.Schedule, "interval:") {
			if parsed, err := time.ParseDuration(strings.TrimPrefix(policy.Schedule, "interval:")); err == nil {
				interval = parsed
			}
		}
	}
	return time.Since(last) >= interval
}

func excluded(patterns string, values ...string) bool {
	for _, rawPattern := range strings.Split(patterns, ",") {
		pattern := strings.TrimSpace(rawPattern)
		if pattern == "" {
			continue
		}
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), strings.ToLower(pattern)) {
				return true
			}
		}
	}
	return false
}
