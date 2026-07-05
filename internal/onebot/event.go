package onebot

import "encoding/json"

type Event struct {
	Time        int64           `json:"time"`
	SelfID      int64           `json:"self_id"`
	PostType    string          `json:"post_type"`
	MessageType string          `json:"message_type,omitempty"`
	NoticeType  string          `json:"notice_type,omitempty"`
	RequestType string          `json:"request_type,omitempty"`
	SubType     string          `json:"sub_type,omitempty"`
	MessageID   int64           `json:"message_id,omitempty"`
	GroupID     int64           `json:"group_id,omitempty"`
	UserID      int64           `json:"user_id,omitempty"`
	OperatorID  int64           `json:"operator_id,omitempty"`
	Message     json.RawMessage `json:"message,omitempty"`
	RawMessage  string          `json:"raw_message,omitempty"`
	Comment     string          `json:"comment,omitempty"`
	Flag        string          `json:"flag,omitempty"`
	TargetID    int64           `json:"target_id,omitempty"`
	File        *FileInfo       `json:"file,omitempty"`
	Sender      Sender          `json:"sender,omitempty"`
}

type FileInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	BusID int64  `json:"busid"`
}

type Sender struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Card     string `json:"card,omitempty"`
	Role     string `json:"role,omitempty"`
}

type APIResponse struct {
	Status  string          `json:"status"`
	RetCode int             `json:"retcode"`
	Data    json.RawMessage `json:"data"`
}

type MessageData struct {
	MessageID int64 `json:"message_id"`
}
