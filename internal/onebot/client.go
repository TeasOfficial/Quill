package onebot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) apiCall(action string, params interface{}) (*APIResponse, error) {
	url := fmt.Sprintf("%s/%s", c.BaseURL, action)
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if apiResp.Status != "ok" {
		return &apiResp, fmt.Errorf("api returned status=%s retcode=%d", apiResp.Status, apiResp.RetCode)
	}
	return &apiResp, nil
}

func (c *Client) SendGroupMsg(groupID int64, message interface{}) (int64, error) {
	resp, err := c.apiCall("send_group_msg", map[string]interface{}{
		"group_id":    groupID,
		"message":     message,
		"auto_escape": false,
	})
	if err != nil {
		return 0, err
	}
	var data MessageData
	json.Unmarshal(resp.Data, &data)
	return data.MessageID, nil
}

func (c *Client) SendPrivateMsg(userID int64, message interface{}) (int64, error) {
	resp, err := c.apiCall("send_private_msg", map[string]interface{}{
		"user_id":     userID,
		"message":     message,
		"auto_escape": false,
	})
	if err != nil {
		return 0, err
	}
	var data MessageData
	json.Unmarshal(resp.Data, &data)
	return data.MessageID, nil
}

func (c *Client) DeleteMsg(messageID int64) error {
	_, err := c.apiCall("delete_msg", map[string]interface{}{
		"message_id": messageID,
	})
	return err
}

func (c *Client) SendMsg(messageType string, userID, groupID int64, message interface{}) (int64, error) {
	params := map[string]interface{}{
		"message_type": messageType,
		"message":      message,
		"auto_escape":  false,
	}
	if messageType == "private" {
		params["user_id"] = userID
	} else {
		params["group_id"] = groupID
	}
	resp, err := c.apiCall("send_msg", params)
	if err != nil {
		return 0, err
	}
	var data MessageData
	json.Unmarshal(resp.Data, &data)
	return data.MessageID, nil
}

func (c *Client) GetMsg(messageID int64) (json.RawMessage, error) {
	resp, err := c.apiCall("get_msg", map[string]interface{}{
		"message_id": messageID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetForwardMsg(messageID string) (json.RawMessage, error) {
	resp, err := c.apiCall("get_forward_msg", map[string]interface{}{
		"message_id": messageID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}
