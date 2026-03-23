package wechat

// MessageType represents the type of message sender.
type MessageType int

const (
	MessageTypeNone MessageType = 0
	MessageTypeUser MessageType = 1 // Message from user
	MessageTypeBot  MessageType = 2 // Message from bot
)

// MessageState represents the state of a message.
type MessageState int

const (
	MessageStateNew        MessageState = 0
	MessageStateGenerating MessageState = 1
	MessageStateFinish     MessageState = 2
)

// ItemType represents the type of message content.
type ItemType int

const (
	ItemTypeText  ItemType = 1
	ItemTypeImage ItemType = 2
	ItemTypeVoice ItemType = 3
	ItemTypeFile  ItemType = 4
	ItemTypeVideo ItemType = 5
)

// Message represents a WeChat message from the iLink API.
type Message struct {
	FromUserID   string        `json:"from_user_id,omitempty"`
	ToUserID     string        `json:"to_user_id,omitempty"`
	ClientID     string        `json:"client_id,omitempty"`
	MessageType  MessageType   `json:"message_type"`
	MessageState MessageState  `json:"message_state"`
	ContextToken string        `json:"context_token,omitempty"`
	GroupID      string        `json:"group_id,omitempty"`
	ItemList     []MessageItem `json:"item_list,omitempty"`
}

// Text returns the text content of the first text item, or empty string.
func (m *Message) Text() string {
	for _, item := range m.ItemList {
		if item.Type == ItemTypeText && item.TextItem != nil {
			return item.TextItem.Text
		}
	}
	return ""
}

// GetImageItem returns the first image item in the message, or nil.
func (m *Message) GetImageItem() *ImageItem {
	for _, item := range m.ItemList {
		if item.Type == ItemTypeImage && item.ImageItem != nil {
			return item.ImageItem
		}
	}
	return nil
}

// GetVoiceItem returns the first voice item in the message, or nil.
func (m *Message) GetVoiceItem() *VoiceItem {
	for _, item := range m.ItemList {
		if item.Type == ItemTypeVoice && item.VoiceItem != nil {
			return item.VoiceItem
		}
	}
	return nil
}

// GetFileItem returns the first file item in the message, or nil.
func (m *Message) GetFileItem() *FileItem {
	for _, item := range m.ItemList {
		if item.Type == ItemTypeFile && item.FileItem != nil {
			return item.FileItem
		}
	}
	return nil
}

// GetVideoItem returns the first video item in the message, or nil.
func (m *Message) GetVideoItem() *VideoItem {
	for _, item := range m.ItemList {
		if item.Type == ItemTypeVideo && item.VideoItem != nil {
			return item.VideoItem
		}
	}
	return nil
}

// IsImage returns true if the message contains an image.
func (m *Message) IsImage() bool {
	return m.GetImageItem() != nil
}

// IsVoice returns true if the message contains a voice message.
func (m *Message) IsVoice() bool {
	return m.GetVoiceItem() != nil
}

// IsFile returns true if the message contains a file attachment.
func (m *Message) IsFile() bool {
	return m.GetFileItem() != nil
}

// IsVideo returns true if the message contains a video.
func (m *Message) IsVideo() bool {
	return m.GetVideoItem() != nil
}

// IsFromUser reports whether this message was sent by a user (not a bot).
func (m *Message) IsFromUser() bool {
	return m.MessageType == MessageTypeUser
}

// MessageItem represents a single content item in a message.
type MessageItem struct {
	Type      ItemType   `json:"type"`
	TextItem  *TextItem  `json:"text_item,omitempty"`
	ImageItem *ImageItem `json:"image_item,omitempty"`
	VoiceItem *VoiceItem `json:"voice_item,omitempty"`
	FileItem  *FileItem  `json:"file_item,omitempty"`
	VideoItem *VideoItem `json:"video_item,omitempty"`
}

// TextItem represents text content.
type TextItem struct {
	Text string `json:"text"`
}

// CDNMedia represents CDN media reference with encryption info.
type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AESKey            string `json:"aes_key,omitempty"` // base64 encoded
	EncryptType       int    `json:"encrypt_type,omitempty"`
}

// ImageItem represents image content with CDN reference.
type ImageItem struct {
	Media       *CDNMedia `json:"media,omitempty"`
	ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
	AESKey      string    `json:"aes_key,omitempty"` // hex string for inbound decryption
	URL         string    `json:"url,omitempty"`
	MidSize     int       `json:"mid_size,omitempty"` // ciphertext file size
	ThumbSize   int       `json:"thumb_size,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
	HDSize      int       `json:"hd_size,omitempty"`
}

// VoiceItem represents voice content.
type VoiceItem struct {
	Media         *CDNMedia `json:"media,omitempty"`
	EncodeType    int       `json:"encode_type,omitempty"`
	BitsPerSample int       `json:"bits_per_sample,omitempty"`
	SampleRate    int       `json:"sample_rate,omitempty"`
	Duration      int       `json:"duration,omitempty"` // in milliseconds
	FileSize      int       `json:"file_size,omitempty"`
	Text          string    `json:"text,omitempty"` // transcribed text from server
}

// FileItem represents a file attachment.
type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	MD5      string    `json:"md5,omitempty"`
	Length   string    `json:"len,omitempty"` // plaintext bytes as string
}

// VideoItem represents video content.
type VideoItem struct {
	Media       *CDNMedia `json:"media,omitempty"`
	VideoSize   int       `json:"video_size,omitempty"`
	PlayLength  int       `json:"play_length,omitempty"`
	VideoMD5    string    `json:"video_md5,omitempty"`
	ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
	ThumbSize   int       `json:"thumb_size,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
}

// --- API Request/Response types ---

// GetUpdatesRequest is the request body for POST /ilink/bot/getupdates.
type GetUpdatesRequest struct {
	GetUpdatesBuf string    `json:"get_updates_buf"`
	BaseInfo      *BaseInfo `json:"base_info,omitempty"`
}

// BaseInfo carries channel version metadata.
type BaseInfo struct {
	ChannelVersion string `json:"channel_version"`
}

// GetUpdatesResponse is the response from POST /ilink/bot/getupdates.
type GetUpdatesResponse struct {
	Ret                  int       `json:"ret"`
	Messages             []Message `json:"msgs"`
	GetUpdatesBuf        string    `json:"get_updates_buf"`
	LongPollingTimeoutMs int       `json:"longpolling_timeout_ms"`
}

// SendMessageRequest is the request body for POST /ilink/bot/sendmessage.
type SendMessageRequest struct {
	Msg      *Message  `json:"msg"`
	BaseInfo *BaseInfo `json:"base_info,omitempty"`
}

// SendMessageResponse is the response from POST /ilink/bot/sendmessage.
type SendMessageResponse struct {
	Ret     int    `json:"ret"`
	ErrCode int    `json:"errcode,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
}
