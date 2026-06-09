package server

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"
)

type Scheduler struct {
	store            *Store
	hub              *AgentHub
	notifier         *Notifier
	heartbeatTimeout time.Duration
	lastRun          map[string]time.Time
}

func NewScheduler(store *Store, hub *AgentHub, notifier *Notifier, heartbeatTimeout time.Duration) *Scheduler {
	return &Scheduler{
		store:            store,
		hub:              hub,
		notifier:         notifier,
		heartbeatTimeout: heartbeatTimeout,
		lastRun:          map[string]time.Time{},
	}
}

func (s *Scheduler) Run(ctx context.Context) {
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
			policy, err := s.store.EffectivePolicy("", project.ID, node.ID)
			if err != nil || !policy.Enabled || policy.Mode == PolicyManual {
				continue
			}
			if excluded(policy.ExcludePatterns, project.Name, project.Path) {
				continue
			}
			key := policy.ID + ":" + node.ID + ":" + project.ID
			if !due(policy, s.lastRun[key]) {
				continue
			}
			kind := "detect_updates"
			if policy.Mode == PolicyAutomatic {
				kind = "compose_update"
			}
			args := map[string]string{"path": project.Path, "name": project.Name}
			payload, _ := json.Marshal(args)
			task, err := s.store.CreateTask(Task{
				NodeID:      node.ID,
				Kind:        kind,
				TargetType:  "compose",
				TargetID:    project.ID,
				RequestedBy: "scheduler",
				PolicyID:    policy.ID,
				Payload:     string(payload),
			})
			if err != nil {
				continue
			}
			s.lastRun[key] = time.Now()
			_ = s.hub.EnqueueTask(task)
		}
	}
	return nil
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
