package server

import (
	"path/filepath"
	"testing"

	"github.com/dockpilot/dockpilot/internal/protocol"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "dockpilot.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestEffectivePolicyPriority(t *testing.T) {
	store := testStore(t)
	_, _ = store.UpsertPolicy(Policy{Scope: "global", Mode: PolicyManual, Enabled: true})
	_, _ = store.UpsertPolicy(Policy{Scope: "node", ScopeID: "node-1", Mode: PolicyScheduled, Schedule: "@daily", Enabled: true})
	_, _ = store.UpsertPolicy(Policy{Scope: "compose", ScopeID: "compose-1", Mode: PolicyAutomatic, Schedule: "@hourly", Enabled: true})

	policy, err := store.EffectivePolicy("", "compose-1", "node-1")
	if err != nil {
		t.Fatalf("resolve compose policy: %v", err)
	}
	if policy.Mode != PolicyAutomatic {
		t.Fatalf("expected compose policy, got %s", policy.Mode)
	}

	policy, err = store.EffectivePolicy("", "", "node-1")
	if err != nil {
		t.Fatalf("resolve node policy: %v", err)
	}
	if policy.Mode != PolicyScheduled {
		t.Fatalf("expected node policy, got %s", policy.Mode)
	}

	policy, err = store.EffectivePolicy("", "", "missing")
	if err != nil {
		t.Fatalf("resolve global policy: %v", err)
	}
	if policy.Mode != PolicyManual {
		t.Fatalf("expected global manual policy, got %s", policy.Mode)
	}
}

func TestTaskLifecycle(t *testing.T) {
	store := testStore(t)
	_, _, err := store.UpsertNodeFromHello(testHello("node-1"), "node-1")
	if err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	task, err := store.CreateTask(Task{NodeID: "node-1", Kind: "detect_updates", RequestedBy: "admin"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.Status != TaskPending {
		t.Fatalf("expected pending, got %s", task.Status)
	}
	if err := store.MarkTaskRunning(task.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	if err := store.AddTaskLog(task.ID, "hello"); err != nil {
		t.Fatalf("add log: %v", err)
	}
	if err := store.FinishTask(task.ID, TaskSuccess, "{}"); err != nil {
		t.Fatalf("finish task: %v", err)
	}
	logs, err := store.TaskLogs(task.ID)
	if err != nil {
		t.Fatalf("task logs: %v", err)
	}
	if len(logs) != 1 || logs[0].Line != "hello" {
		t.Fatalf("unexpected logs: %#v", logs)
	}
}

func testHello(nodeID string) protocol.HelloPayload {
	return protocol.HelloPayload{
		NodeID:            nodeID,
		NodeToken:         "token",
		RegistrationToken: "registration",
		Name:              "test-node",
		Version:           "test",
		OS:                "linux",
		Arch:              "amd64",
	}
}
