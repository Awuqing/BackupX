package notify

import (
	"strings"
	"testing"
)

// TestBuildRawMessageStripsHeaderInjection 验证用户可控内容（如备份任务名
// 进入 Subject）中的 CR/LF 被剔除，无法注入额外头部或伪造正文。
func TestBuildRawMessageStripsHeaderInjection(t *testing.T) {
	msg := Message{
		Title: "备份失败\r\nBcc: attacker@evil.com\r\n\r\n伪造正文",
		Body:  "正文第一行\n正文第二行",
	}
	raw := string(buildRawMessage("sender@example.com", []string{"ops@example.com"}, msg))

	parts := strings.SplitN(raw, "\r\n\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("缺少头部/正文分隔符，原文=%q", raw)
	}
	headerBlock, body := parts[0], parts[1]

	// 头部区不得出现独立的注入头行。
	for _, line := range strings.Split(headerBlock, "\r\n") {
		if strings.HasPrefix(line, "Bcc:") {
			t.Fatalf("检测到头注入：出现独立 Bcc 头行 %q", line)
		}
	}
	// 头部区应恰好是固定的 5 行（From/To/Subject/MIME-Version/Content-Type）。
	if got := len(strings.Split(headerBlock, "\r\n")); got != 5 {
		t.Fatalf("头部行数=%d，期望 5；headerBlock=%q", got, headerBlock)
	}
	// 正文必须保持原样（正文中的 \n 合法，不应被处理）。
	if body != "正文第一行\n正文第二行" {
		t.Fatalf("正文被篡改：%q", body)
	}
	// Subject 行必须包含原始标题文本（CRLF 被移除后拼接在同一行）。
	if !strings.Contains(headerBlock, "Subject: 备份失败Bcc: attacker@evil.com伪造正文") {
		t.Fatalf("Subject 行不符合预期：%q", headerBlock)
	}
}

func TestSanitizeHeaderValue(t *testing.T) {
	cases := map[string]string{
		"  normal  ":       "normal",
		"a\r\nb":           "ab",
		"x\ny\rz":          "xyz",
		"no-control-chars": "no-control-chars",
	}
	for in, want := range cases {
		if got := sanitizeHeaderValue(in); got != want {
			t.Errorf("sanitizeHeaderValue(%q) = %q, want %q", in, got, want)
		}
	}
}
