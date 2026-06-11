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

func TestDefaultPolicyDisablesAutomaticUpdateOnly(t *testing.T) {
	store := testStore(t)
	policy, err := store.ResolvePolicy("global", "")
	if err != nil {
		t.Fatalf("resolve default policy: %v", err)
	}
	if policy.Mode != PolicyManual || policy.Schedule != DefaultPolicySchedule || !policy.Enabled {
		t.Fatalf("unexpected default policy: %#v", policy)
	}

	saved, err := store.UpsertPolicy(Policy{Scope: "node", ScopeID: "node-1", Enabled: true})
	if err != nil {
		t.Fatalf("upsert defaulted policy: %v", err)
	}
	if saved.Mode != PolicyManual || saved.Schedule != DefaultPolicySchedule {
		t.Fatalf("policy defaults were not applied: %#v", saved)
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

func TestUpdateRecords(t *testing.T) {
	store := testStore(t)
	_, _, err := store.UpsertNodeFromHello(testHello("node-1"), "node-1")
	if err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	task, err := store.CreateTask(Task{NodeID: "node-1", Kind: "compose_update", TargetType: "compose", TargetID: "compose-1"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	err = store.InsertUpdateRecords(task, []protocol.ImageChange{{
		TargetType:      "compose",
		TargetID:        "compose-1",
		Name:            "web",
		PreviousVersion: "sha256:old",
		CurrentVersion:  "sha256:new",
		Changed:         true,
	}})
	if err != nil {
		t.Fatalf("insert update records: %v", err)
	}
	records, err := store.ListUpdateRecords(10)
	if err != nil {
		t.Fatalf("list update records: %v", err)
	}
	if len(records) != 1 || records[0].Name != "web" || !records[0].Changed {
		t.Fatalf("unexpected update records: %#v", records)
	}
}

func TestUpdateNodePreservesCustomName(t *testing.T) {
	store := testStore(t)
	_, _, err := store.UpsertNodeFromHello(testHello("node-1"), "node-1")
	if err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	node, err := store.UpdateNode("node-1", "edge-prod", "primary docker host")
	if err != nil {
		t.Fatalf("update node: %v", err)
	}
	if node.Name != "edge-prod" || node.Note != "primary docker host" {
		t.Fatalf("node metadata was not updated: %#v", node)
	}
	hello := testHello("node-1")
	hello.Name = "agent-hostname"
	node, _, err = store.UpsertNodeFromHello(hello, "node-1")
	if err != nil {
		t.Fatalf("upsert renamed node: %v", err)
	}
	if node.Name != "edge-prod" {
		t.Fatalf("custom name was overwritten: %#v", node)
	}
}

func TestUpsertNodeReusesOfflineDaemonID(t *testing.T) {
	store := testStore(t)
	hello := testHello("node-1")
	hello.Labels = map[string]string{"docker_daemon_id": "daemon-a"}
	node, _, err := store.UpsertNodeFromHello(hello, "node-1")
	if err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if err := store.MarkNodeSeen(node.ID, "offline"); err != nil {
		t.Fatalf("mark offline: %v", err)
	}
	rejoin := testHello("")
	rejoin.NodeToken = ""
	rejoin.Labels = map[string]string{"docker_daemon_id": "daemon-a"}
	reused, _, err := store.UpsertNodeFromHello(rejoin, "")
	if err != nil {
		t.Fatalf("upsert rejoin: %v", err)
	}
	if reused.ID != node.ID {
		t.Fatalf("expected node %s to be reused, got %s", node.ID, reused.ID)
	}
	nodes, err := store.ListNodes()
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected one node after rejoin, got %#v", nodes)
	}
}

func TestUpsertNodeFallsBackToOfflineHostIdentity(t *testing.T) {
	store := testStore(t)
	hello := testHello("node-1")
	node, _, err := store.UpsertNodeFromHello(hello, "node-1")
	if err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if err := store.MarkNodeSeen(node.ID, "offline"); err != nil {
		t.Fatalf("mark offline: %v", err)
	}
	rejoin := testHello("")
	rejoin.NodeToken = ""
	rejoin.Labels = map[string]string{"docker_daemon_id": "daemon-after-upgrade"}
	reused, _, err := store.UpsertNodeFromHello(rejoin, "")
	if err != nil {
		t.Fatalf("upsert rejoin: %v", err)
	}
	if reused.ID != node.ID {
		t.Fatalf("expected node %s to be reused, got %s", node.ID, reused.ID)
	}
}

func TestClearFinishedTasks(t *testing.T) {
	store := testStore(t)
	_, _, err := store.UpsertNodeFromHello(testHello("node-1"), "node-1")
	if err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	pending, _ := store.CreateTask(Task{NodeID: "node-1", Kind: "detect_updates"})
	success, _ := store.CreateTask(Task{NodeID: "node-1", Kind: "detect_updates"})
	failed, _ := store.CreateTask(Task{NodeID: "node-1", Kind: "detect_updates"})
	if err := store.FinishTask(success.ID, TaskSuccess, "{}"); err != nil {
		t.Fatalf("finish success: %v", err)
	}
	if err := store.FinishTask(failed.ID, TaskFailed, "{}"); err != nil {
		t.Fatalf("finish failed: %v", err)
	}
	deleted, err := store.ClearTasks("finished")
	if err != nil {
		t.Fatalf("clear tasks: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected two deleted tasks, got %d", deleted)
	}
	tasks, err := store.ListTasks(10)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != pending.ID {
		t.Fatalf("pending task should remain: %#v", tasks)
	}
}

func TestApplyAndClearUpdateDetections(t *testing.T) {
	store := testStore(t)
	_, _, err := store.UpsertNodeFromHello(testHello("node-1"), "node-1")
	if err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	snapshot := protocol.DockerSnapshotPayload{
		Containers: []protocol.ContainerSnapshot{{
			ID:             "container-1",
			Name:           "web",
			Image:          "nginx:stable",
			State:          "running",
			Status:         "Up",
			ComposeProject: "site",
		}},
		ComposeProjects: []protocol.ComposeProjectSnapshot{{
			ID:   "compose-1",
			Name: "site",
			Path: "/opt/site/compose.yml",
		}},
	}
	if err := store.ReplaceDockerSnapshot("node-1", snapshot); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	count, err := store.ApplyUpdateDetections("node-1", []protocol.UpdateDetection{{
		TargetType:  "compose",
		TargetID:    "compose-1",
		ProjectName: "site",
		Path:        "/opt/site/compose.yml",
		Images: []protocol.ImageUpdateDetection{{
			Image:           "nginx:stable",
			LocalDigest:     "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			RemoteDigest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			UpdateAvailable: true,
		}},
	}})
	if err != nil {
		t.Fatalf("apply detections: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one available update, got %d", count)
	}
	state, err := store.DockerState("node-1")
	if err != nil {
		t.Fatalf("docker state: %v", err)
	}
	if !state.Containers[0].UpdateAvailable {
		t.Fatalf("container update flag was not set")
	}
	if !state.ComposeProjects[0].UpdateAvailable || state.ComposeProjects[0].CheckedAt == "" {
		t.Fatalf("compose update metadata was not set: %#v", state.ComposeProjects[0])
	}
	if state.ComposeProjects[0].DetectionStatus != "update_available" {
		t.Fatalf("compose detection status was not set: %#v", state.ComposeProjects[0])
	}
	task := Task{
		NodeID:     "node-1",
		Kind:       "compose_update",
		TargetType: "compose",
		TargetID:   "compose-1",
		Payload:    `{"name":"site"}`,
	}
	if err := store.ClearUpdateAvailabilityForTask(task); err != nil {
		t.Fatalf("clear update flags: %v", err)
	}
	state, err = store.DockerState("node-1")
	if err != nil {
		t.Fatalf("docker state after clear: %v", err)
	}
	if state.Containers[0].UpdateAvailable || state.ComposeProjects[0].UpdateAvailable {
		t.Fatalf("update flags were not cleared: %#v %#v", state.Containers[0], state.ComposeProjects[0])
	}
	if state.ComposeProjects[0].DetectionStatus != "current" {
		t.Fatalf("compose detection status was not cleared: %#v", state.ComposeProjects[0])
	}
}

func TestFailedUpdateDetectionPreservesAvailability(t *testing.T) {
	store := testStore(t)
	_, _, err := store.UpsertNodeFromHello(testHello("node-1"), "node-1")
	if err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	snapshot := protocol.DockerSnapshotPayload{
		Containers: []protocol.ContainerSnapshot{{
			ID:             "container-1",
			Name:           "web",
			Image:          "nginx:stable",
			State:          "running",
			Status:         "Up",
			ComposeProject: "site",
		}},
		ComposeProjects: []protocol.ComposeProjectSnapshot{{
			ID:   "compose-1",
			Name: "site",
			Path: "/opt/site/compose.yml",
		}},
	}
	if err := store.ReplaceDockerSnapshot("node-1", snapshot); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	_, err = store.ApplyUpdateDetections("node-1", []protocol.UpdateDetection{{
		TargetType:  "compose",
		TargetID:    "compose-1",
		ProjectName: "site",
		Path:        "/opt/site/compose.yml",
		Images: []protocol.ImageUpdateDetection{{
			Image:           "nginx:stable",
			UpdateAvailable: true,
		}},
	}})
	if err != nil {
		t.Fatalf("apply available detection: %v", err)
	}
	_, err = store.ApplyUpdateDetections("node-1", []protocol.UpdateDetection{{
		TargetType:  "compose",
		TargetID:    "compose-1",
		ProjectName: "site",
		Path:        "/opt/site/compose.yml",
		Error:       "registry unavailable",
		Images:      []protocol.ImageUpdateDetection{},
	}})
	if err != nil {
		t.Fatalf("apply failed detection: %v", err)
	}
	state, err := store.DockerState("node-1")
	if err != nil {
		t.Fatalf("docker state: %v", err)
	}
	project := state.ComposeProjects[0]
	if !project.UpdateAvailable {
		t.Fatalf("failed detection should preserve compose availability: %#v", project)
	}
	if project.DetectionStatus != "failed" || project.DetectionError == "" {
		t.Fatalf("failed detection metadata was not saved: %#v", project)
	}
	if !state.Containers[0].UpdateAvailable {
		t.Fatalf("failed detection should preserve container availability: %#v", state.Containers[0])
	}
}

func TestFallbackUpdateDetectionUsesImageResultStatus(t *testing.T) {
	store := testStore(t)
	_, _, err := store.UpsertNodeFromHello(testHello("node-1"), "node-1")
	if err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	snapshot := protocol.DockerSnapshotPayload{
		Containers: []protocol.ContainerSnapshot{{
			ID:             "container-1",
			Name:           "web",
			Image:          "nginx:stable",
			State:          "running",
			Status:         "Up",
			ComposeProject: "site",
		}},
		ComposeProjects: []protocol.ComposeProjectSnapshot{{
			ID:   "compose-1",
			Name: "site",
			Path: "/opt/site/compose.yml",
		}},
	}
	if err := store.ReplaceDockerSnapshot("node-1", snapshot); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	_, err = store.ApplyUpdateDetections("node-1", []protocol.UpdateDetection{{
		TargetType:  "compose",
		TargetID:    "compose-1",
		ProjectName: "site",
		Path:        "/opt/site/compose.yml",
		Error:       "compose config failed; using runtime images",
		Images: []protocol.ImageUpdateDetection{{
			Image:           "nginx:stable",
			Method:          "registry",
			Platform:        "linux/amd64",
			UpdateAvailable: false,
		}},
	}})
	if err != nil {
		t.Fatalf("apply fallback detection: %v", err)
	}
	state, err := store.DockerState("node-1")
	if err != nil {
		t.Fatalf("docker state: %v", err)
	}
	project := state.ComposeProjects[0]
	if project.DetectionStatus != "current" || project.DetectionError == "" {
		t.Fatalf("fallback image result should be current with retained error detail: %#v", project)
	}
	if project.UpdateAvailable || state.Containers[0].UpdateAvailable {
		t.Fatalf("fallback current result should not mark updates: %#v %#v", project, state.Containers[0])
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
