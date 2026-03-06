package telegram

import (
	"admin/internal/config"
	"admin/pkg/logger"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	baseURL string
	token   string
	chatID  string
	http    *http.Client
}

func NewClient(cfg *config.TelegramCfg) (*Client, error) {

	token := strings.TrimSpace(cfg.BotToken)
	chatID := strings.TrimSpace(cfg.ChatID)
	if token == "" {
		logger.SysWarn("telegram.init", "telegram enabled but bot token is empty")
		return nil, errors.New("telegram bot token is required when TELEGRAM_ENABLE=true")
	}
	if chatID == "" {
		logger.SysWarn("telegram.init", "telegram enabled but chat id is empty")
		return nil, errors.New("telegram chat id is required when TELEGRAM_ENABLE=true")
	}

	client := &Client{
		baseURL: cfg.BaseURL,
		token:   token,
		chatID:  chatID,
		http: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
	}

	logger.SysInfo(
		"telegram.init",
		"telegram notifier initialized base_url=%s chat_id=%s timeout=%s",
		client.baseURL,
		maskSensitive(client.chatID),
		client.http.Timeout,
	)

	return client, nil
}

func (c *Client) SendMessage(ctx context.Context, text string) error {
	if c == nil {
		logger.SysDebug("telegram.send", "skip send: telegram client is nil")
		return nil
	}

	text = strings.TrimSpace(text)
	if text == "" {
		logger.SysWarn("telegram.send", "skip send: empty telegram message")
		return errors.New("telegram message is empty")
	}
	logger.SysDebug(
		"telegram.send",
		"sending telegram message chat_id=%s text_len=%d",
		maskSensitive(c.chatID),
		len(text),
	)

	payload := telegramSendMessageReq{
		ChatID: c.chatID,
		Text:   text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, c.token),
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		logger.SysWarn("telegram.send", "telegram request failed: %v", err)
		return fmt.Errorf("send telegram request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read telegram response: %w", err)
	}

	var apiResp telegramSendMessageResp
	_ = json.Unmarshal(respBody, &apiResp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.SysWarn(
			"telegram.send",
			"telegram returned non-2xx status=%d desc=%q",
			resp.StatusCode,
			apiResp.Description,
		)
		if apiResp.Description != "" {
			return fmt.Errorf("telegram send message failed: status=%d desc=%s", resp.StatusCode, apiResp.Description)
		}
		return fmt.Errorf("telegram send message failed: status=%d", resp.StatusCode)
	}
	if !apiResp.OK {
		logger.SysWarn("telegram.send", "telegram response not ok desc=%q", apiResp.Description)
		if apiResp.Description != "" {
			return fmt.Errorf("telegram send message failed: %s", apiResp.Description)
		}
		return errors.New("telegram send message failed")
	}

	logger.SysDebug("telegram.send", "telegram message sent successfully status=%d", resp.StatusCode)
	return nil
}

type telegramSendMessageReq struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type telegramSendMessageResp struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func maskSensitive(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) <= 4 {
		return "****"
	}
	return raw[:2] + "****" + raw[len(raw)-2:]
}
