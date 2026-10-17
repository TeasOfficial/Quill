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

func (c *Client) SendLike(userID int64, times int) error {
	_, err := c.apiCall("send_like", map[string]interface{}{
		"user_id": userID,
		"times":   times,
	})
	return err
}

func (c *Client) SetGroupKick(groupID, userID int64, rejectAddRequest bool) error {
	_, err := c.apiCall("set_group_kick", map[string]interface{}{
		"group_id":           groupID,
		"user_id":            userID,
		"reject_add_request": rejectAddRequest,
	})
	return err
}

func (c *Client) SetGroupBan(groupID, userID int64, duration int64) error {
	_, err := c.apiCall("set_group_ban", map[string]interface{}{
		"group_id": groupID,
		"user_id":  userID,
		"duration": duration,
	})
	return err
}

func (c *Client) SetGroupWholeBan(groupID int64, enable bool) error {
	_, err := c.apiCall("set_group_whole_ban", map[string]interface{}{
		"group_id": groupID,
		"enable":   enable,
	})
	return err
}

func (c *Client) SetGroupAdmin(groupID, userID int64, enable bool) error {
	_, err := c.apiCall("set_group_admin", map[string]interface{}{
		"group_id": groupID,
		"user_id":  userID,
		"enable":   enable,
	})
	return err
}

func (c *Client) SetGroupCard(groupID, userID int64, card string) error {
	_, err := c.apiCall("set_group_card", map[string]interface{}{
		"group_id": groupID,
		"user_id":  userID,
		"card":     card,
	})
	return err
}

func (c *Client) SetGroupName(groupID int64, groupName string) error {
	_, err := c.apiCall("set_group_name", map[string]interface{}{
		"group_id":   groupID,
		"group_name": groupName,
	})
	return err
}

func (c *Client) SetGroupSpecialTitle(groupID, userID int64, specialTitle string, duration int64) error {
	_, err := c.apiCall("set_group_special_title", map[string]interface{}{
		"group_id":      groupID,
		"user_id":       userID,
		"special_title": specialTitle,
		"duration":      duration,
	})
	return err
}

func (c *Client) SetGroupLeave(groupID int64, isDismiss bool) error {
	_, err := c.apiCall("set_group_leave", map[string]interface{}{
		"group_id":   groupID,
		"is_dismiss": isDismiss,
	})
	return err
}

func (c *Client) GetStatus() (json.RawMessage, error) {
	resp, err := c.apiCall("get_status", nil)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetVersionInfo() (json.RawMessage, error) {
	resp, err := c.apiCall("get_version_info", nil)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) SetFriendAddRequest(flag string, approve bool, remark string) error {
	_, err := c.apiCall("set_friend_add_request", map[string]interface{}{
		"flag":    flag,
		"approve": approve,
		"remark":  remark,
	})
	return err
}

func (c *Client) SetGroupAddRequest(flag, subType string, approve bool, reason string) error {
	_, err := c.apiCall("set_group_add_request", map[string]interface{}{
		"flag":     flag,
		"sub_type": subType,
		"approve":  approve,
		"reason":   reason,
	})
	return err
}

func (c *Client) GetLoginInfo() (json.RawMessage, error) {
	resp, err := c.apiCall("get_login_info", nil)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetStrangerInfo(userID int64, noCache bool) (json.RawMessage, error) {
	resp, err := c.apiCall("get_stranger_info", map[string]interface{}{
		"user_id":  userID,
		"no_cache": noCache,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetFriendList() (json.RawMessage, error) {
	resp, err := c.apiCall("get_friend_list", nil)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetGroupInfo(groupID int64, noCache bool) (json.RawMessage, error) {
	resp, err := c.apiCall("get_group_info", map[string]interface{}{
		"group_id": groupID,
		"no_cache": noCache,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetGroupList() (json.RawMessage, error) {
	resp, err := c.apiCall("get_group_list", nil)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetGroupMemberInfo(groupID, userID int64, noCache bool) (json.RawMessage, error) {
	resp, err := c.apiCall("get_group_member_info", map[string]interface{}{
		"group_id": groupID,
		"user_id":  userID,
		"no_cache": noCache,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetGroupMemberList(groupID int64) (json.RawMessage, error) {
	resp, err := c.apiCall("get_group_member_list", map[string]interface{}{
		"group_id": groupID,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetGroupHonorInfo(groupID int64, honorType string) (json.RawMessage, error) {
	resp, err := c.apiCall("get_group_honor_info", map[string]interface{}{
		"group_id": groupID,
		"type":     honorType,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetCookies(domain string) (json.RawMessage, error) {
	resp, err := c.apiCall("get_cookies", map[string]interface{}{
		"domain": domain,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetCSRFToken() (json.RawMessage, error) {
	resp, err := c.apiCall("get_csrf_token", nil)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetCredentials(domain string) (json.RawMessage, error) {
	resp, err := c.apiCall("get_credentials", map[string]interface{}{
		"domain": domain,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetRecord(file, outFormat string) (json.RawMessage, error) {
	resp, err := c.apiCall("get_record", map[string]interface{}{
		"file":       file,
		"out_format": outFormat,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetImage(file string) (json.RawMessage, error) {
	resp, err := c.apiCall("get_image", map[string]interface{}{
		"file": file,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) CanSendImage() (json.RawMessage, error) {
	resp, err := c.apiCall("can_send_image", nil)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) CanSendRecord() (json.RawMessage, error) {
	resp, err := c.apiCall("can_send_record", nil)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}
