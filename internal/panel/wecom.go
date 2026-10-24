package panel

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/skip2/go-qrcode"
)

// wecomHTTP 企业微信推送专用 client：短超时，避免卡住 API 请求。
var wecomHTTP = &http.Client{Timeout: 10 * time.Second}

// sendWeComLoginQR 将登录授权链接与对应二维码推送到企业微信机器人。
// 先发 Markdown 文本（含账号标识与授权链接），再发 PNG 二维码图片（base64 + md5）。
func sendWeComLoginQR(webhook, nickname, uid, authURL string) error {
	if webhook == "" {
		return fmt.Errorf("wecom webhook not configured")
	}
	if nickname == "" {
		nickname = uid
	}
	// 1) 文本/Markdown 消息：便于手机端直接点链接。
	md := fmt.Sprintf("### 账号重新登录\n账号：%s\nuid：`%s`\n\n请将下方二维码转发给账号持有人，扫码登录：\n%s", nickname, uid, authURL)
	body := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]any{
			"content": md,
		},
	}
	if err := postWecom(webhook, body); err != nil {
		return err
	}

	// 2) 二维码图片消息。
	png, err := qrcode.Encode(authURL, qrcode.Medium, 256)
	if err != nil {
		return fmt.Errorf("qrcode encode: %w", err)
	}
	sum := md5.Sum(png)
	imgBody := map[string]any{
		"msgtype": "image",
		"image": map[string]any{
			"base64": base64.StdEncoding.EncodeToString(png),
			"md5":    hex.EncodeToString(sum[:]),
		},
	}
	if err := postWecom(webhook, imgBody); err != nil {
		return err
	}
	return nil
}

func postWecom(webhook string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, webhook, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := wecomHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("wecom http %d: %s", resp.StatusCode, string(raw))
	}
	var env struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	_ = json.Unmarshal(raw, &env)
	if env.ErrCode != 0 {
		return fmt.Errorf("wecom errcode=%d errmsg=%s", env.ErrCode, env.ErrMsg)
	}
	return nil
}
