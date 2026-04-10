package comfyui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type QueuedPrompt struct {
	PromptID string `json:"prompt_id"`
	Number   int    `json:"number"`
	NodeErrors map[string]interface{} `json:"node_errors"`
}

func (c *Client) QueuePrompt(prompt map[string]interface{}, clientID string) (*QueuedPrompt, error) {
	apiURL := fmt.Sprintf("%s/prompt", c.BaseURL)
	payload := map[string]interface{}{
		"prompt":    prompt,
		"client_id": clientID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ComfyUI /prompt failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result QueuedPrompt
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) GetHistory(promptID string) (map[string]interface{}, error) {
	apiURL := fmt.Sprintf("%s/history/%s", c.BaseURL, promptID)
	resp, err := c.HTTPClient.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ComfyUI /history failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if historyEntry, ok := result[promptID].(map[string]interface{}); ok {
		return historyEntry, nil
	}
	return nil, nil
}

func (c *Client) UploadImage(data []byte, filename string) (map[string]interface{}, error) {
	apiURL := fmt.Sprintf("%s/upload/image", c.BaseURL)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	_ = writer.WriteField("type", "input")
	_ = writer.WriteField("overwrite", "true")

	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return nil, err
	}
	_, err = part.Write(data)
	if err != nil {
		return nil, err
	}
	
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ComfyUI /upload/image failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) DownloadImage(filename, subfolder, folderType string) ([]byte, error) {
	u, err := url.Parse(fmt.Sprintf("%s/view", c.BaseURL))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("filename", filename)
	if subfolder != "" {
		q.Set("subfolder", subfolder)
	}
	q.Set("type", folderType)
	u.RawQuery = q.Encode()

	resp, err := c.HTTPClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ComfyUI /view failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) GetWsURL(clientID string) string {
	u, _ := url.Parse(c.BaseURL)
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	return fmt.Sprintf("%s://%s/ws?clientId=%s", scheme, u.Host, clientID)
}

func (c *Client) WaitForCompletion(promptID, clientID string, timeout time.Duration) (map[string]interface{}, error) {
	wsURL := c.GetWsURL(clientID)
	
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		// Fallback to polling if WS fails
		return c.pollForCompletion(promptID, timeout)
	}
	defer conn.Close()

	timeoutChan := time.After(timeout)
	
	// Start polling in background just in case WS misses the event
	pollResultChan := make(chan map[string]interface{})
	pollErrChan := make(chan error)
	go func() {
		res, err := c.pollForCompletion(promptID, timeout)
		if err != nil {
			pollErrChan <- err
		} else {
			pollResultChan <- res
		}
	}()

	for {
		select {
		case <-timeoutChan:
			return nil, fmt.Errorf("timeout waiting for prompt completion")
		case res := <-pollResultChan:
			return res, nil
		case err := <-pollErrChan:
			// Ignore polling errors and rely on WS, or return if WS is stuck
			fmt.Printf("ComfyUI polling error: %v\n", err)
		default:
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				// WS error, rely on polling
				res, ok := <-pollResultChan
				if ok {
					return res, nil
				}
				// if channel closed or we read from error chan
				return nil, <-pollErrChan
			}
			
			if msgType == websocket.TextMessage {
				var event map[string]interface{}
				if errJson := json.Unmarshal(msg, &event); errJson == nil {
					if t, ok := event["type"].(string); ok && t == "executing" {
						data := event["data"].(map[string]interface{})
						if data["prompt_id"] == promptID && data["node"] == nil {
							// Execution completed
							return c.GetHistory(promptID)
						}
					}
				}
			}
		}
	}
}

func (c *Client) pollForCompletion(promptID string, timeout time.Duration) (map[string]interface{}, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for prompt completion")
		}

		history, err := c.GetHistory(promptID)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		if history != nil {
			if status, ok := history["status"].(map[string]interface{}); ok {
				if completed, ok := status["completed"].(bool); ok && completed {
					return history, nil
				}
			}
		}

		<-ticker.C
	}
}