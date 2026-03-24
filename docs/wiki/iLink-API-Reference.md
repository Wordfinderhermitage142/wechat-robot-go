# iLink Bot API 参考文档

本文档详细描述了 wechat-robot-go SDK 所使用的 iLink Bot API，包括认证、消息收发、媒体处理等核心接口。

## 目录

- [概述](#概述)
- [认证 API](#认证-api)
- [消息 API](#消息-api)
- [媒体 API](#媒体-api)
- [状态指示 API](#状态指示-api)
- [数据类型参考](#数据类型参考)
- [错误码](#错误码)

---

## 概述

iLink API 是企业微信机器人的底层通信协议，采用 RESTful + JSON 格式。SDK 通过长轮询机制接收消息，通过 POST 请求发送消息。

### 基础信息

| 项目 | 值 |
|------|-----|
| 协议 | HTTPS |
| 数据格式 | JSON |
| 认证方式 | Bot Token (Bearer) |
| 字符编码 | UTF-8 |

### API 端点格式

```
{BASE_URL}/ilink/bot/{API_PATH}
```

### 请求头

所有 API 请求都需要包含以下 Header：

```http
Authorization: Bearer {bot_token}
Content-Type: application/json
```

### 通用请求结构

大多数 POST 请求需要包含 `base_info` 字段：

```json
{
  "base_info": {
    "channel_version": "1.0.3"
  }
}
```

### 通用响应结构

```json
{
  "ret": 0,
  "errmsg": "ok",
  "errcode": 0
}
```

---

## 认证 API

### 1. 获取登录二维码

获取一个新的二维码用于扫码登录。

**请求**

```http
GET /ilink/bot/get_bot_qrcode?bot_type=3
```

**参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| bot_type | int | 是 | 固定值 `3` |

**响应**

```json
{
  "qrcode": "uuid-string",
  "qrcode_img_url": "https://example.com/qrcode.png",
  "qrcode_img_content": "base64-encoded-image-data"
}
```

**字段说明**

| 字段 | 类型 | 说明 |
|------|------|------|
| qrcode | string | 二维码唯一标识，用于后续轮询状态 |
| qrcode_img_url | string | 二维码图片 URL（可选） |
| qrcode_img_content | string | Base64 编码的二维码图片数据 |

---

### 2. 轮询二维码状态

轮询扫码状态，直到用户确认或二维码过期。

**请求**

```http
GET /ilink/bot/get_qrcode_status?qrcode={qrcode}
```

**参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| qrcode | string | 是 | 从 `get_bot_qrcode` 获取的二维码标识 |

**响应**

```json
{
  "status": "confirmed",
  "bot_token": "eyJhbGciOiJIUzI1NiIs...",
  "baseurl": "https://api.example.com"
}
```

**状态值**

| 状态 | 说明 |
|------|------|
| `wait` | 等待扫码 |
| `scaned` | 已扫码，等待确认 |
| `confirmed` | 用户已确认，登录成功 |
| `expired` | 二维码已过期 |

---

### 3. 获取配置信息

获取 Bot 配置信息，包括 `typing_ticket`。

**请求**

```http
POST /ilink/bot/getconfig
```

**请求体**

```json
{}
```

**响应**

```json
{
  "ret": 0,
  "typing_ticket": "ticket-string"
}
```

**说明**

`typing_ticket` 用于发送"正在输入"状态指示，有效期约 24 小时。

---

## 消息 API

### 4. 长轮询获取消息

使用长轮询方式接收用户消息。这是 Bot 的核心接口。

**请求**

```http
POST /ilink/bot/getupdates
```

**请求体**

```json
{
  "get_updates_buf": "",
  "base_info": {
    "channel_version": "1.0.3"
  }
}
```

**字段说明**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| get_updates_buf | string | 是 | 游标字符串，首次请求为空字符串，之后使用上次响应中的值 |
| base_info | object | 是 | 基础信息 |

**响应**

```json
{
  "ret": 0,
  "msgs": [
    {
      "from_user_id": "user-id-123",
      "to_user_id": "bot-id-456",
      "client_id": "client-789",
      "message_type": 1,
      "message_state": 2,
      "context_token": "ctx-token-abc",
      "group_id": "",
      "item_list": [
        {
          "type": 1,
          "text_item": {
            "text": "用户发送的文本内容"
          }
        }
      ]
    }
  ],
  "get_updates_buf": "next-cursor-string",
  "longpolling_timeout_ms": 35000
}
```

**响应字段说明**

| 字段 | 类型 | 说明 |
|------|------|------|
| msgs | array | 消息数组，空数组表示超时（正常行为） |
| get_updates_buf | string | 下一次请求使用的游标 |
| longpolling_timeout_ms | int | 服务器建议的超时时间（毫秒） |

**消息类型 (message_type)**

| 值 | 说明 |
|----|------|
| 1 | 用户消息 |
| 2 | Bot 消息 |

**消息状态 (message_state)**

| 值 | 说明 |
|----|------|
| 0 | 新消息 |
| 1 | 生成中（如 AI 正在生成回复） |
| 2 | 完成 |

**内容类型 (type)**

| 值 | 内容类型 | 说明 |
|----|----------|------|
| 1 | text | 文本消息 |
| 2 | image | 图片 |
| 3 | voice | 语音 |
| 4 | file | 文件 |
| 5 | video | 视频 |

---

### 5. 发送消息

向用户发送文本、媒体等各类消息。

**请求**

```http
POST /ilink/bot/sendmessage
```

**请求体**

```json
{
  "msg": {
    "from_user_id": "",
    "to_user_id": "user-id-123",
    "client_id": "openclaw-weixin-timestamp-random",
    "message_type": 2,
    "message_state": 2,
    "context_token": "ctx-token-abc",
    "item_list": [
      {
        "type": 1,
        "text_item": {
          "text": "这是回复内容"
        }
      }
    ]
  },
  "base_info": {
    "channel_version": "1.0.3"
  }
}
```

**字段说明**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| msg.from_user_id | string | 是 | 必须为空字符串 `""` |
| msg.to_user_id | string | 是 | 接收者用户 ID |
| msg.client_id | string | 是 | 客户端生成的唯一 ID，格式：`openclaw-weixin-{timestamp}-{random}` |
| msg.message_type | int | 是 | 固定值 `2` (Bot 消息) |
| msg.message_state | int | 是 | 固定值 `2` (完成) |
| msg.context_token | string | 是 | 用于关联会话的上下文令牌 |
| msg.item_list | array | 是 | 消息内容项数组 |
| base_info | object | 是 | 基础信息 |

**响应**

```json
{
  "ret": 0,
  "errcode": 0,
  "errmsg": "ok"
}
```

---

## 媒体 API

### 6. 获取上传 URL

获取 CDN 上传地址和参数。

**请求**

```http
POST /ilink/bot/getuploadurl
```

**请求体**

```json
{
  "filekey": "16字节随机hex字符串",
  "to_user_id": "user-id-123",
  "media_type": 1,
  "rawsize": 102400,
  "rawfilemd5": "md5-of-original-file",
  "filesize": 102592,
  "no_need_thumb": true,
  "aeskey": "hex-encoded-16-byte-aes-key",
  "base_info": {
    "channel_version": "1.0.3"
  }
}
```

**字段说明**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| filekey | string | 是 | 16 字节随机 hex 字符串 |
| to_user_id | string | 是 | 接收者用户 ID |
| media_type | int | 是 | 媒体类型：1=图片, 2=视频, 3=文件, 4=语音 |
| rawsize | int | 是 | 原始文件大小（字节） |
| rawfilemd5 | string | 是 | 原始文件的 MD5（32 位 hex） |
| filesize | int | 是 | 加密后的文件大小 |
| no_need_thumb | bool | 是 | 是否需要缩略图，设为 true |
| aeskey | string | 是 | AES-128 密钥（32 位 hex） |
| base_info | object | 是 | 基础信息 |

**响应**

```json
{
  "ret": 0,
  "upload_url": "https://cdn.example.com/upload",
  "upload_param": "encrypted-param-string"
}
```

---

### 7. 上传媒体文件到 CDN

将加密后的媒体文件上传到 CDN。

**请求**

```http
POST {upload_url}/upload?encrypted_query_param={upload_param}&filekey={filekey}
```

**请求头**

```http
Content-Type: application/octet-stream
```

**请求体**

二进制数据（加密后的文件内容）

**响应**

从响应头获取：

```http
x-encrypted-param: {encrypted-param-for-download}
```

---

### 8. 下载媒体文件

从 CDN 下载并解密媒体文件。

**请求**

```http
GET {cdn_base_url}/download?encrypted_query_param={encrypt_query_param}
```

**响应**

二进制数据（加密的媒体文件）

**解密流程**

1. 从 CDN 下载加密数据
2. 使用 AES-128-ECB + PKCS7 解密
3. 得到原始文件内容

---

## 状态指示 API

### 9. 发送 Typing 状态

向用户发送"正在输入"状态指示。

**请求**

```http
POST /ilink/bot/sendtyping
```

**请求体**

```json
{
  "to_user_id": "user-id-123",
  "typing_ticket": "ticket-from-getconfig",
  "status": 1
}
```

**字段说明**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| to_user_id | string | 是 | 用户 ID |
| typing_ticket | string | 是 | 从 `getconfig` 获取的票据 |
| status | int | 是 | 1=开始输入, 2=停止输入 |

**响应**

```json
{
  "ret": 0,
  "errcode": 0,
  "errmsg": "ok"
}
```

---

## 数据类型参考

### Message 消息体

```go
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
```

### MessageItem 内容项

```go
type MessageItem struct {
    Type      ItemType   `json:"type"`
    TextItem  *TextItem  `json:"text_item,omitempty"`
    ImageItem *ImageItem `json:"image_item,omitempty"`
    VoiceItem *VoiceItem `json:"voice_item,omitempty"`
    FileItem  *FileItem  `json:"file_item,omitempty"`
    VideoItem *VideoItem `json:"video_item,omitempty"`
}
```

### CDNMedia CDN 媒体引用

```go
type CDNMedia struct {
    EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
    AESKey            string `json:"aes_key,omitempty"` // base64 encoded
    EncryptType       int    `json:"encrypt_type,omitempty"`
}
```

### ImageItem 图片项

```go
type ImageItem struct {
    Media       *CDNMedia `json:"media,omitempty"`
    ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
    AESKey      string    `json:"aes_key,omitempty"` // hex string for inbound decryption
    URL         string    `json:"url,omitempty"`
    MidSize     int       `json:"mid_size,omitempty"`
    ThumbSize   int       `json:"thumb_size,omitempty"`
    ThumbWidth  int       `json:"thumb_width,omitempty"`
    ThumbHeight int       `json:"thumb_height,omitempty"`
    HDSize      int       `json:"hd_size,omitempty"`
}
```

### VoiceItem 语音项

```go
type VoiceItem struct {
    Media         *CDNMedia `json:"media,omitempty"`
    EncodeType    int       `json:"encode_type,omitempty"`
    BitsPerSample int       `json:"bits_per_sample,omitempty"`
    SampleRate    int       `json:"sample_rate,omitempty"`
    Duration      int       `json:"duration,omitempty"` // milliseconds
    FileSize      int       `json:"file_size,omitempty"`
    Text          string    `json:"text,omitempty"` // transcribed text from server
}
```

### FileItem 文件项

```go
type FileItem struct {
    Media    *CDNMedia `json:"media,omitempty"`
    FileName string    `json:"file_name,omitempty"`
    MD5      string    `json:"md5,omitempty"`
    Length   string    `json:"len,omitempty"` // plaintext bytes as string
}
```

### VideoItem 视频项

```go
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
```

---

## 错误码

### 通用错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| -1 | 通用错误 |
| -2 | 参数错误 |
| -3 | 频率限制 |
| -14 | Session 过期 |
| -100 | 权限错误 |
| -101 | Token 无效 |

### 媒体上传错误码

| 错误码 | 说明 |
|--------|------|
| 40001 | 文件类型不支持 |
| 40002 | 文件大小超限 |
| 40003 | AES 密钥格式错误 |
| 40004 | MD5 校验失败 |
| 40005 | 文件 key 格式错误 |

---

## 附录

### A. AES-128-ECB 加解密

iLink API 使用 AES-128-ECB 模式对媒体文件进行加解密：

- **算法**: AES-128-ECB
- **填充**: PKCS7
- **密钥**: 16 字节随机生成，hex 编码传输

### B. ClientID 生成规则

```
openclaw-weixin-{timestamp_hex}-{random_hex}
```

示例：`openclaw-weixin-4-a1b2c3d4`

### C. Channel Version

当前版本：`1.0.3`

此字段必须包含在大多数 API 请求的 `base_info` 中。

---

## 相关文档

- [架构设计](Architecture)
- [中间件指南](Middleware-Guide)
- [GitHub 仓库](https://github.com/SpellingDragon/wechat-robot-go)
