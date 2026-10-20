package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	url   string
	key   string
	model string
	http  *http.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func New(url, key, model string) *Client {
	return &Client{
		url:   strings.TrimSuffix(url, "/") + "/chat/completions",
		key:   key,
		model: model,
		http:  &http.Client{Timeout: 5 * time.Minute},
	}
}

func (c *Client) Model() string { return c.model }

func (c *Client) Complete(ctx context.Context, msgs []Message) (string, error) {
	var b strings.Builder
	err := c.Stream(ctx, msgs, &b)
	return b.String(), err
}

func (c *Client) Stream(ctx context.Context, msgs []Message, w io.Writer) error {
	body, _ := json.Marshal(map[string]any{
		"model":       c.model,
		"messages":    msgs,
		"temperature": 0.2,
		"stream":      true,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "PR Ladder")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("llm: %s %s", res.Status, strings.TrimSpace(string(msg)))
	}

	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data: ")
		if !ok {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return fmt.Errorf("llm: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) > 0 {
			if _, err := io.WriteString(w, chunk.Choices[0].Delta.Content); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}
