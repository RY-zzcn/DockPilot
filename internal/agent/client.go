package agent

import (
	"context"
	"log"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dockpilot/dockpilot/internal/protocol"
	"github.com/dockpilot/dockpilot/internal/version"
	"github.com/gorilla/websocket"
)

type Client struct {
	cfg      Config
	docker   DockerClient
	detector *UpdateDetector
	metrics  MetricsCollector
	stateMu  sync.Mutex
}

func NewClient(cfg Config) *Client {
	return &Client{
		cfg:      cfg,
		docker:   DockerClient{ComposeDirs: cfg.ComposeDirs},
		detector: NewUpdateDetector(cfg.UpdateCacheTTL),
	}
}

func (c *Client) Run(ctx context.Context) error {
	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := c.connectOnce(ctx); err != nil {
			log.Printf("agent connection failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) connectOnce(ctx context.Context) error {
	wsURL, err := agentURL(c.cfg.ServerURL)
	if err != nil {
		return err
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	writeMu := sync.Mutex{}
	send := func(msg protocol.Message) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(msg)
	}

	labels := map[string]string{"runtime": "docker-cli"}
	if daemonID := c.docker.DaemonID(ctx); daemonID != "" {
		labels["docker_daemon_id"] = daemonID
	}
	hello := protocol.HelloPayload{
		NodeID:            c.cfg.NodeID,
		NodeToken:         c.cfg.NodeToken,
		RegistrationToken: c.cfg.RegistrationToken,
		Name:              c.cfg.Name,
		Version:           version.Version,
		OS:                runtime.GOOS,
		Arch:              runtime.GOARCH,
		DockerVersion:     c.docker.DockerVersion(ctx),
		ComposeVersion:    c.docker.ComposeVersion(ctx),
		Labels:            labels,
	}
	msg, _ := protocol.NewMessage(protocol.TypeHello, c.cfg.NodeID, hello)
	if err := send(msg); err != nil {
		return err
	}

	var incoming protocol.Message
	if err := conn.ReadJSON(&incoming); err != nil {
		return err
	}
	if incoming.Type == protocol.TypeHello {
		ack, err := protocol.DecodePayload[protocol.HelloAckPayload](incoming)
		if err == nil {
			c.updateState(ack.NodeID, ack.NodeToken)
			log.Printf("registered as node %s", ack.NodeID)
		}
	} else if incoming.Type == protocol.TypeError {
		payload, _ := protocol.DecodePayload[protocol.ErrorPayload](incoming)
		return logServerError(payload)
	} else {
		return logServerError(protocol.ErrorPayload{Code: "unexpected_message", Message: incoming.Type})
	}

	stop := make(chan struct{})
	defer close(stop)
	go c.reportLoop(ctx, stop, send)
	go c.selfUpdateLoop(ctx, stop)
	for {
		if err := conn.ReadJSON(&incoming); err != nil {
			return err
		}
		switch incoming.Type {
		case protocol.TypeHello:
			ack, err := protocol.DecodePayload[protocol.HelloAckPayload](incoming)
			if err == nil {
				c.updateState(ack.NodeID, ack.NodeToken)
				log.Printf("registered as node %s", ack.NodeID)
			}
		case protocol.TypeTaskRequest:
			task, err := protocol.DecodePayload[protocol.TaskPayload](incoming)
			if err == nil {
				go c.handleTask(ctx, task, send)
			}
		case protocol.TypeError:
			payload, _ := protocol.DecodePayload[protocol.ErrorPayload](incoming)
			log.Printf("server error: %s %s", payload.Code, payload.Message)
		}
	}
}

func (c *Client) reportLoop(ctx context.Context, stop <-chan struct{}, send func(protocol.Message) error) {
	heartbeat := time.NewTicker(15 * time.Second)
	metrics := time.NewTicker(c.cfg.MetricsInterval)
	snapshot := time.NewTicker(c.cfg.SnapshotInterval)
	defer heartbeat.Stop()
	defer metrics.Stop()
	defer snapshot.Stop()

	c.sendMetrics(ctx, send)
	c.sendSnapshot(ctx, send)
	for {
		select {
		case <-heartbeat.C:
			c.sendHeartbeat(send)
		case <-metrics.C:
			c.sendMetrics(ctx, send)
		case <-snapshot.C:
			c.sendSnapshot(ctx, send)
		case <-stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) sendHeartbeat(send func(protocol.Message) error) {
	msg, _ := protocol.NewMessage(protocol.TypeHeartbeat, c.cfg.NodeID, protocol.HeartbeatPayload{
		UptimeSeconds: int64(time.Since(startedAt).Seconds()),
		Status:        "online",
	})
	_ = send(msg)
}

func (c *Client) sendMetrics(ctx context.Context, send func(protocol.Message) error) {
	count := c.docker.ContainerCount(ctx)
	payload := c.metrics.Collect(count)
	msg, _ := protocol.NewMessage(protocol.TypeMetrics, c.cfg.NodeID, payload)
	_ = send(msg)
}

func (c *Client) sendSnapshot(ctx context.Context, send func(protocol.Message) error) {
	payload := c.docker.Snapshot(ctx)
	msg, _ := protocol.NewMessage(protocol.TypeDockerSnapshot, c.cfg.NodeID, payload)
	_ = send(msg)
}

func (c *Client) handleTask(ctx context.Context, task protocol.TaskPayload, send func(protocol.Message) error) {
	executor := TaskExecutor{
		Docker:            c.docker,
		Detector:          c.detector,
		ServerURL:         c.cfg.ServerURL,
		RegistrationToken: c.cfg.RegistrationToken,
		NodeName:          c.cfg.Name,
		ComposeDirs:       c.cfg.ComposeDirs,
		MetricsInterval:   c.cfg.MetricsInterval,
		SnapshotInterval:  c.cfg.SnapshotInterval,
		UpdateCacheTTL:    c.cfg.UpdateCacheTTL,
		InstallMode:       c.cfg.InstallMode,
		ReleaseRepo:       c.cfg.ReleaseRepo,
		AgentImage:        c.cfg.AgentImage,
		AllowDeploy:       c.cfg.AllowDeploy,
	}
	logLine := func(line string) {
		msg, _ := protocol.NewMessage(protocol.TypeTaskLog, c.cfg.NodeID, protocol.TaskLogPayload{
			TaskID: task.ID,
			Line:   line,
		})
		_ = send(msg)
	}
	result := executor.Execute(ctx, task, logLine)
	msg, _ := protocol.NewMessage(protocol.TypeTaskResult, c.cfg.NodeID, result)
	_ = send(msg)
	c.sendSnapshot(ctx, send)
	if result.RestartAgent {
		go func() {
			log.Printf("agent restart requested after task %s", task.ID)
			time.Sleep(2 * time.Second)
			os.Exit(0)
		}()
	}
}

func (c *Client) selfUpdateLoop(ctx context.Context, stop <-chan struct{}) {
	if !c.cfg.SelfUpdate {
		return
	}
	interval := c.cfg.SelfUpdateInterval
	if interval <= 0 {
		interval = time.Hour
	}
	timer := time.NewTimer(45 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			c.runSelfUpdateCheck(ctx)
			timer.Reset(interval)
		case <-stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) runSelfUpdateCheck(ctx context.Context) {
	executor := TaskExecutor{
		Docker:            c.docker,
		Detector:          c.detector,
		ServerURL:         c.cfg.ServerURL,
		RegistrationToken: c.cfg.RegistrationToken,
		NodeName:          c.cfg.Name,
		ComposeDirs:       c.cfg.ComposeDirs,
		MetricsInterval:   c.cfg.MetricsInterval,
		SnapshotInterval:  c.cfg.SnapshotInterval,
		UpdateCacheTTL:    c.cfg.UpdateCacheTTL,
		InstallMode:       c.cfg.InstallMode,
		ReleaseRepo:       c.cfg.ReleaseRepo,
		AgentImage:        c.cfg.AgentImage,
		AllowDeploy:       c.cfg.AllowDeploy,
	}
	logLine := func(line string) {
		log.Printf("self-update: %s", line)
	}
	restart, err := executor.agentUpdate(ctx, protocol.TaskPayload{
		ID:   "agent-self-update",
		Kind: "agent_update",
		Args: map[string]string{"version": "latest"},
	}, logLine)
	if err != nil {
		log.Printf("agent self-update check failed: %v", err)
		return
	}
	if restart {
		log.Printf("agent self-update installed a new binary; restarting")
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}
}

func (c *Client) updateState(nodeID, nodeToken string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.cfg.NodeID = nodeID
	c.cfg.NodeToken = nodeToken
	_ = SaveState(c.cfg.StatePath, State{NodeID: nodeID, NodeToken: nodeToken})
}

func agentURL(serverURL string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http", "":
		u.Scheme = "ws"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/agent/ws"
	return u.String(), nil
}

var startedAt = time.Now()

func logServerError(payload protocol.ErrorPayload) error {
	log.Printf("server error: %s %s", payload.Code, payload.Message)
	return &serverError{payload: payload}
}

type serverError struct {
	payload protocol.ErrorPayload
}

func (e *serverError) Error() string {
	return e.payload.Code + ": " + e.payload.Message
}
