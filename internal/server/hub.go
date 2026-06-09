package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/dockpilot/dockpilot/internal/protocol"
	"github.com/gorilla/websocket"
)

type AgentHub struct {
	cfg      Config
	store    *Store
	notifier *Notifier
	mu       sync.RWMutex
	sessions map[string]*AgentSession
}

type AgentSession struct {
	nodeID string
	conn   *websocket.Conn
	send   chan protocol.Message
	closed chan struct{}
}

func NewAgentHub(cfg Config, store *Store, notifier *Notifier) *AgentHub {
	return &AgentHub{
		cfg:      cfg,
		store:    store,
		notifier: notifier,
		sessions: map[string]*AgentSession{},
	}
}

func (h *AgentHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("agent ws upgrade failed: %v", err)
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	var msg protocol.Message
	if err := conn.ReadJSON(&msg); err != nil {
		_ = conn.Close()
		return
	}
	if msg.Type != protocol.TypeHello {
		_ = conn.Close()
		return
	}
	hello, err := protocol.DecodePayload[protocol.HelloPayload](msg)
	if err != nil {
		_ = conn.Close()
		return
	}
	node, err := h.authenticateHello(hello)
	if err != nil {
		_ = conn.WriteJSON(errorMessage("", "auth_failed", err.Error()))
		_ = conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	session := &AgentSession{
		nodeID: node.ID,
		conn:   conn,
		send:   make(chan protocol.Message, 64),
		closed: make(chan struct{}),
	}
	h.setSession(session)
	defer h.removeSession(node.ID, session)

	ack, _ := protocol.NewMessage(protocol.TypeHello, node.ID, protocol.HelloAckPayload{
		NodeID:    node.ID,
		NodeToken: node.Token,
		Message:   "registered",
	})
	session.send <- ack
	_ = h.store.MarkNodeSeen(node.ID, "online")
	_ = h.store.AddEvent("info", "node_online", "Node connected: "+node.Name, map[string]string{"node_id": node.ID})
	h.notifier.Notify("info", "节点上线", "节点 "+node.Name+" 已连接")

	go session.writeLoop()
	h.dispatchPending(node.ID)
	session.readLoop(h)
}

func (h *AgentHub) authenticateHello(hello protocol.HelloPayload) (Node, error) {
	if hello.NodeID != "" && hello.NodeToken != "" {
		if _, err := h.store.AuthenticateNode(hello.NodeID, hello.NodeToken); err != nil {
			return Node{}, err
		}
		node, _, err := h.store.UpsertNodeFromHello(hello, hello.NodeID)
		return node, err
	}
	if hello.RegistrationToken == "" || hello.RegistrationToken != h.cfg.AgentRegistrationToken {
		return Node{}, errors.New("invalid registration token")
	}
	node, _, err := h.store.UpsertNodeFromHello(hello, "")
	return node, err
}

func (h *AgentHub) setSession(session *AgentSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old := h.sessions[session.nodeID]; old != nil {
		_ = old.conn.Close()
		close(old.closed)
	}
	h.sessions[session.nodeID] = session
}

func (h *AgentHub) removeSession(nodeID string, session *AgentSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions[nodeID] == session {
		delete(h.sessions, nodeID)
		_ = h.store.MarkNodeSeen(nodeID, "offline")
	}
}

func (h *AgentHub) IsOnline(nodeID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessions[nodeID] != nil
}

func (h *AgentHub) EnqueueTask(task Task) error {
	h.mu.RLock()
	session := h.sessions[task.NodeID]
	h.mu.RUnlock()
	if session == nil {
		return h.store.AddTaskLog(task.ID, "Node is offline; task is queued until the agent reconnects.")
	}
	return h.sendTask(session, task)
}

func (h *AgentHub) dispatchPending(nodeID string) {
	tasks, err := h.store.PendingTasksForNode(nodeID)
	if err != nil {
		log.Printf("load pending tasks for %s failed: %v", nodeID, err)
		return
	}
	for _, task := range tasks {
		if err := h.EnqueueTask(task); err != nil {
			log.Printf("dispatch task %s failed: %v", task.ID, err)
		}
	}
}

func (h *AgentHub) sendTask(session *AgentSession, task Task) error {
	if err := h.store.MarkTaskRunning(task.ID); err != nil {
		return err
	}
	args := map[string]string{}
	if task.Payload != "" {
		_ = json.Unmarshal([]byte(task.Payload), &args)
	}
	payload := protocol.TaskPayload{
		ID:         task.ID,
		Kind:       task.Kind,
		TargetType: task.TargetType,
		TargetID:   task.TargetID,
		Args:       args,
	}
	msg, err := protocol.NewMessage(protocol.TypeTaskRequest, task.NodeID, payload)
	if err != nil {
		return err
	}
	msg.RequestID = task.ID
	select {
	case session.send <- msg:
		return h.store.AddTaskLog(task.ID, "Task dispatched to agent.")
	case <-session.closed:
		return h.store.AddTaskLog(task.ID, "Agent disconnected before task dispatch.")
	case <-time.After(3 * time.Second):
		return h.store.AddTaskLog(task.ID, "Agent send buffer is full; task dispatch timed out.")
	}
}

func (s *AgentSession) writeLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	defer s.conn.Close()
	for {
		select {
		case msg := <-s.send:
			if err := s.conn.WriteJSON(msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := s.conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
				return
			}
		case <-s.closed:
			return
		}
	}
}

func (s *AgentSession) readLoop(h *AgentHub) {
	defer s.conn.Close()
	for {
		var msg protocol.Message
		if err := s.conn.ReadJSON(&msg); err != nil {
			return
		}
		if msg.NodeID == "" {
			msg.NodeID = s.nodeID
		}
		h.handleAgentMessage(msg)
	}
}

func (h *AgentHub) handleAgentMessage(msg protocol.Message) {
	switch msg.Type {
	case protocol.TypeHeartbeat:
		_ = h.store.MarkNodeSeen(msg.NodeID, "online")
	case protocol.TypeMetrics:
		payload, err := protocol.DecodePayload[protocol.MetricsPayload](msg)
		if err == nil {
			_ = h.store.InsertMetrics(msg.NodeID, payload)
		}
	case protocol.TypeDockerSnapshot:
		payload, err := protocol.DecodePayload[protocol.DockerSnapshotPayload](msg)
		if err == nil {
			_ = h.store.ReplaceDockerSnapshot(msg.NodeID, payload)
		}
	case protocol.TypeTaskLog:
		payload, err := protocol.DecodePayload[protocol.TaskLogPayload](msg)
		if err == nil {
			_ = h.store.AddTaskLog(payload.TaskID, payload.Line)
		}
	case protocol.TypeTaskResult:
		payload, err := protocol.DecodePayload[protocol.TaskResultPayload](msg)
		if err == nil {
			status := TaskSuccess
			if payload.Status != TaskSuccess {
				status = TaskFailed
			}
			task, _ := h.store.GetTask(payload.TaskID)
			if status == TaskSuccess && len(payload.Updates) > 0 {
				count, err := h.store.ApplyUpdateDetections(msg.NodeID, payload.Updates)
				if err == nil && count > 0 {
					h.notifier.Notify("warning", "更新可用", payload.TaskID+" 检测到 "+fmt.Sprint(count)+" 个镜像可更新")
				}
			}
			if status == TaskSuccess && (task.Kind == "compose_update" || task.Kind == "compose_deploy") {
				_ = h.store.ClearUpdateAvailabilityForTask(task)
			}
			raw, _ := json.Marshal(payload)
			_ = h.store.FinishTask(payload.TaskID, status, string(raw))
			if status == TaskFailed {
				h.notifier.Notify("error", "任务失败", payload.TaskID+": "+payload.Message)
			}
			if status == TaskSuccess {
				h.notifier.Notify("info", "任务完成", payload.TaskID+" 已完成")
			}
		}
	case protocol.TypeError:
		payload, err := protocol.DecodePayload[protocol.ErrorPayload](msg)
		if err == nil {
			_ = h.store.AddEvent("error", payload.Code, payload.Message, payload)
		}
	}
}

func errorMessage(nodeID, code, message string) protocol.Message {
	msg, _ := protocol.NewMessage(protocol.TypeError, nodeID, protocol.ErrorPayload{
		Code:    code,
		Message: message,
	})
	return msg
}
