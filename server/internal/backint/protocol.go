// Package backint 实现 SAP HANA Backint 协议代理。
//
// Backint 协议是 SAP HANA 与第三方备份工具之间的管道/文件协议。
// SAP HANA 通过 CLI 调用 Backint Agent，传入参数文件、输入文件、输出文件，
// Agent 根据输入文件中的 #PIPE / #EBID / #NULL 指令读取/写入数据，
// 并在输出文件中返回 #SAVED / #RESTORED / #BACKUP / #NOTFOUND / #DELETED / #ERROR。
//
// 支持的功能：BACKUP / RESTORE / INQUIRE / DELETE
// 参考规范：SAP HANA Backint Interface for Backup Tools (OSS 1642148)
package backint

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Function 代表 Backint 操作类型，对应 CLI 的 -f 参数。
type Function string

const (
	FunctionBackup   Function = "backup"
	FunctionRestore  Function = "restore"
	FunctionInquire  Function = "inquire"
	FunctionDelete   Function = "delete"
)

// BackupRequest 是 BACKUP 操作的单条请求。
//
// 两种形态：
//   - Pipe:  #PIPE <path>                (HANA 通过命名管道传输数据)
//   - File:  "<path>"                     (HANA 指向一个已完成的临时文件)
type BackupRequest struct {
	IsPipe bool
	Path   string
}

// RestoreRequest 是 RESTORE 操作的单条请求。
//
// 形态：#PIPE <ebid> "<path>"  或  <ebid> "<path>"
type RestoreRequest struct {
	IsPipe bool
	EBID   string // 之前 BACKUP 返回的备份 ID
	Path   string
}

// InquireRequest 是 INQUIRE 操作的单条请求。
//
// 形态：
//   - #NULL                        (列出所有备份)
//   - "<ebid>"                     (查询指定 ID 是否存在)
//   - #EBID "<ebid>"               (带前缀的变体)
type InquireRequest struct {
	All  bool
	EBID string
}

// DeleteRequest 是 DELETE 操作的单条请求。
//
// 形态：<ebid> 或 #EBID <ebid>
type DeleteRequest struct {
	EBID string
}

// ParseBackupRequests 解析 BACKUP 输入文件。
func ParseBackupRequests(r io.Reader) ([]BackupRequest, error) {
	var items []BackupRequest
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#PIPE") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "#PIPE"))
			if path == "" {
				return nil, fmt.Errorf("invalid #PIPE line: %q", line)
			}
			items = append(items, BackupRequest{IsPipe: true, Path: trimQuotes(path)})
			continue
		}
		items = append(items, BackupRequest{IsPipe: false, Path: trimQuotes(line)})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// ParseRestoreRequests 解析 RESTORE 输入文件。
func ParseRestoreRequests(r io.Reader) ([]RestoreRequest, error) {
	var items []RestoreRequest
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		isPipe := false
		if strings.HasPrefix(line, "#PIPE") {
			isPipe = true
			line = strings.TrimSpace(strings.TrimPrefix(line, "#PIPE"))
		}
		if strings.HasPrefix(line, "#EBID") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "#EBID"))
		}
		ebid, rest := splitFirstField(line)
		if ebid == "" || rest == "" {
			return nil, fmt.Errorf("invalid restore line: %q", line)
		}
		items = append(items, RestoreRequest{
			IsPipe: isPipe,
			EBID:   trimQuotes(ebid),
			Path:   trimQuotes(rest),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// ParseInquireRequests 解析 INQUIRE 输入文件。
func ParseInquireRequests(r io.Reader) ([]InquireRequest, error) {
	var items []InquireRequest
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "#NULL" {
			items = append(items, InquireRequest{All: true})
			continue
		}
		if strings.HasPrefix(line, "#EBID") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "#EBID"))
		}
		items = append(items, InquireRequest{EBID: trimQuotes(line)})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// ParseDeleteRequests 解析 DELETE 输入文件。
func ParseDeleteRequests(r io.Reader) ([]DeleteRequest, error) {
	var items []DeleteRequest
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EBID") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "#EBID"))
		}
		ebid := trimQuotes(strings.TrimSpace(line))
		if ebid == "" {
			return nil, fmt.Errorf("invalid delete line: %q", line)
		}
		items = append(items, DeleteRequest{EBID: ebid})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// 输出写入辅助

// WriteSaved 写入一条 BACKUP 成功响应：#SAVED <ebid> "<path>"
func WriteSaved(w io.Writer, ebid, path string) error {
	_, err := fmt.Fprintf(w, "#SAVED %s %s\n", ebid, quote(path))
	return err
}

// WriteRestored 写入一条 RESTORE 成功响应：#RESTORED "<ebid>" "<path>"
func WriteRestored(w io.Writer, ebid, path string) error {
	_, err := fmt.Fprintf(w, "#RESTORED %s %s\n", quote(ebid), quote(path))
	return err
}

// WriteBackup 写入一条 INQUIRE 命中响应：#BACKUP "<ebid>"
func WriteBackup(w io.Writer, ebid string) error {
	_, err := fmt.Fprintf(w, "#BACKUP %s\n", quote(ebid))
	return err
}

// WriteNotFound 写入一条 INQUIRE/RESTORE 未命中响应：#NOTFOUND "<path-or-ebid>"
func WriteNotFound(w io.Writer, identifier string) error {
	_, err := fmt.Fprintf(w, "#NOTFOUND %s\n", quote(identifier))
	return err
}

// WriteDeleted 写入一条 DELETE 成功响应：#DELETED "<ebid>"
func WriteDeleted(w io.Writer, ebid string) error {
	_, err := fmt.Fprintf(w, "#DELETED %s\n", quote(ebid))
	return err
}

// WriteError 写入一条错误响应：#ERROR "<path-or-ebid>"
//
// SAP HANA 会将 #ERROR 视为本条请求失败，但不会终止整个批次。
// 在 stderr 输出错误详情便于排查。
func WriteError(w io.Writer, identifier string) error {
	_, err := fmt.Fprintf(w, "#ERROR %s\n", quote(identifier))
	return err
}

// 内部工具函数

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// splitFirstField 把一行拆分为 "第一个字段" 和 "剩余部分"。
// 支持带引号的字段：`"abc def" "path"` → `abc def` / `"path"`。
func splitFirstField(line string) (first, rest string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	if line[0] == '"' {
		idx := strings.Index(line[1:], `"`)
		if idx < 0 {
			return line, ""
		}
		return line[1 : idx+1], strings.TrimSpace(line[idx+2:])
	}
	idx := strings.IndexAny(line, " \t")
	if idx < 0 {
		return line, ""
	}
	return line[:idx], strings.TrimSpace(line[idx+1:])
}

// ParseFunction 将 CLI 的 -f 参数字符串规范化为 Function。
func ParseFunction(s string) (Function, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "backup":
		return FunctionBackup, nil
	case "restore":
		return FunctionRestore, nil
	case "inquire":
		return FunctionInquire, nil
	case "delete":
		return FunctionDelete, nil
	default:
		return "", errors.New("unsupported backint function: " + s)
	}
}
