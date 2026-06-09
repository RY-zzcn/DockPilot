package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

type Notifier struct {
	store *Store
	client *http.Client
}

func NewNotifier(store *Store) *Notifier {
	return &Notifier{
		store:  store,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *Notifier) Notify(severity, title, body string) {
	if n == nil || n.store == nil {
		return
	}
	_ = n.store.AddEvent(severity, "notification", title+": "+body, map[string]string{"title": title, "body": body})
	notifications, err := n.store.EnabledNotifications()
	if err != nil {
		log.Printf("load notifications failed: %v", err)
		return
	}
	for _, notification := range notifications {
		go func(item Notification) {
			if err := n.send(item, title, body); err != nil {
				log.Printf("notification %s failed: %v", item.Name, err)
			}
		}(notification)
	}
}

func (n *Notifier) send(notification Notification, title, body string) error {
	switch strings.ToLower(notification.Channel) {
	case "telegram":
		return n.sendTelegram(notification.Config, title, body)
	case "webhook":
		return n.sendWebhook(notification.Config, title, body)
	case "email":
		return n.sendEmail(notification.Config, title, body)
	default:
		return fmt.Errorf("unsupported notification channel %q", notification.Channel)
	}
}

func (n *Notifier) sendTelegram(config, title, body string) error {
	var cfg struct {
		BotToken string `json:"bot_token"`
		ChatID   string `json:"chat_id"`
	}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return err
	}
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return fmt.Errorf("telegram bot_token and chat_id are required")
	}
	payload, _ := json.Marshal(map[string]string{
		"chat_id": cfg.ChatID,
		"text":    title + "\n" + body,
	})
	resp, err := n.client.Post("https://api.telegram.org/bot"+cfg.BotToken+"/sendMessage", "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram status %s", resp.Status)
	}
	return nil
}

func (n *Notifier) sendWebhook(config, title, body string) error {
	var cfg struct {
		URL     string            `json:"url"`
		Method  string            `json:"method"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return err
	}
	if cfg.URL == "" {
		return fmt.Errorf("webhook url is required")
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodPost
	}
	payload, _ := json.Marshal(map[string]string{"title": title, "body": body})
	req, err := http.NewRequest(cfg.Method, cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range cfg.Headers {
		req.Header.Set(key, value)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %s", resp.Status)
	}
	return nil
}

func (n *Notifier) sendEmail(config, title, body string) error {
	var cfg struct {
		SMTPHost string `json:"smtp_host"`
		SMTPPort string `json:"smtp_port"`
		Username string `json:"username"`
		Password string `json:"password"`
		From     string `json:"from"`
		To       string `json:"to"`
	}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return err
	}
	if cfg.SMTPPort == "" {
		cfg.SMTPPort = "587"
	}
	if cfg.SMTPHost == "" || cfg.From == "" || cfg.To == "" {
		return fmt.Errorf("smtp_host, from and to are required")
	}
	addr := cfg.SMTPHost + ":" + cfg.SMTPPort
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	msg := []byte("To: " + cfg.To + "\r\n" +
		"Subject: " + title + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		body)
	return smtp.SendMail(addr, auth, cfg.From, []string{cfg.To}, msg)
}
