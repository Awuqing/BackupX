package backup

import (
	"archive/tar"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifyReport 是 quick 模式的验证结果摘要。
type VerifyReport struct {
	TotalEntries int    `json:"totalEntries,omitempty"`
	FileBytes    int64  `json:"fileBytes,omitempty"`
	ChecksumOK   bool   `json:"checksumOk,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

// VerifyTarArchive 遍历 tar 归档的每个 header + reader，不写盘。
// 能检测归档截断、条目损坏、层级不对等常见问题。
// expectedChecksum 非空时额外对整个文件校验 SHA-256（不做解压）。
func VerifyTarArchive(artifactPath string, expectedChecksum string) (*VerifyReport, error) {
	file, err := os.Open(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("open tar artifact: %w", err)
	}
	defer file.Close()
	report := &VerifyReport{}
	h := sha256.New()
	reader := io.TeeReader(file, h)
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return report, fmt.Errorf("read tar entry: %w", err)
		}
		report.TotalEntries++
		// 读完条目数据以触发完整性校验（tar 内部 CRC 不严格，但断流会报错）
		if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
			n, copyErr := io.Copy(io.Discard, tr)
			if copyErr != nil {
				return report, fmt.Errorf("read entry %s: %w", header.Name, copyErr)
			}
			report.FileBytes += n
		}
	}
	// 读完 tar 后继续把剩余字节喂给 hash（tar 结束后可能有零填充尾）
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return report, fmt.Errorf("drain remainder: %w", err)
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if strings.TrimSpace(expectedChecksum) != "" {
		report.ChecksumOK = strings.EqualFold(actual, expectedChecksum)
		if !report.ChecksumOK {
			return report, fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actual)
		}
	} else {
		report.ChecksumOK = true
	}
	report.Detail = fmt.Sprintf("tar 包完整（%d 条目，有效字节 %d）", report.TotalEntries, report.FileBytes)
	return report, nil
}

// VerifySQLiteFile 校验 SQLite 文件头魔数。
// 官方格式：前 16 字节为 "SQLite format 3\000"。
func VerifySQLiteFile(artifactPath string) (*VerifyReport, error) {
	file, err := os.Open(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite artifact: %w", err)
	}
	defer file.Close()
	header := make([]byte, 16)
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, fmt.Errorf("read sqlite header: %w", err)
	}
	const magic = "SQLite format 3\x00"
	if string(header) != magic {
		return &VerifyReport{Detail: "非法的 SQLite 文件头"}, fmt.Errorf("invalid sqlite magic header")
	}
	info, _ := file.Stat()
	var size int64
	if info != nil {
		size = info.Size()
	}
	return &VerifyReport{
		FileBytes: size,
		Detail:    fmt.Sprintf("SQLite 文件头合法（总大小 %d 字节）", size),
	}, nil
}

// VerifyMySQLDump 校验 MySQL dump 文件头部是否为合法 mysqldump 输出。
// 头部 1024 字节包含以下任一关键字即通过：
//   - "-- MySQL dump"
//   - "-- Server version"
//   - "-- MariaDB dump"
func VerifyMySQLDump(artifactPath string) (*VerifyReport, error) {
	return verifyDumpHeader(artifactPath, []string{"-- MySQL dump", "-- Server version", "-- MariaDB dump"}, "MySQL/MariaDB")
}

// VerifyPostgreSQLDump 校验 PostgreSQL plain text dump 头部。
// 典型标记："-- PostgreSQL database dump" 或 "-- Dumped from database version"。
func VerifyPostgreSQLDump(artifactPath string) (*VerifyReport, error) {
	return verifyDumpHeader(artifactPath, []string{"-- PostgreSQL database dump", "-- Dumped from database version", "SET statement_timeout"}, "PostgreSQL")
}

func verifyDumpHeader(artifactPath string, markers []string, label string) (*VerifyReport, error) {
	file, err := os.Open(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("open dump artifact: %w", err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	buf := make([]byte, 4096)
	n, _ := io.ReadFull(reader, buf)
	sample := string(buf[:n])
	matched := ""
	for _, m := range markers {
		if strings.Contains(sample, m) {
			matched = m
			break
		}
	}
	if matched == "" {
		return &VerifyReport{Detail: fmt.Sprintf("未在前 %d 字节中发现 %s dump 特征", n, label)}, fmt.Errorf("no %s dump marker in header", label)
	}
	info, _ := file.Stat()
	var size int64
	if info != nil {
		size = info.Size()
	}
	return &VerifyReport{
		FileBytes: size,
		Detail:    fmt.Sprintf("%s dump 头部识别标志: %q（文件 %d 字节）", label, matched, size),
	}, nil
}

// VerifySAPHANAArchive 校验 SAP HANA 归档 tar 中是否包含 databackup/logbackup 标志文件。
func VerifySAPHANAArchive(artifactPath string) (*VerifyReport, error) {
	file, err := os.Open(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("open hana archive: %w", err)
	}
	defer file.Close()
	tr := tar.NewReader(file)
	report := &VerifyReport{}
	var foundDataBackup bool
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return report, fmt.Errorf("read tar entry: %w", err)
		}
		report.TotalEntries++
		name := strings.ToLower(header.Name)
		if strings.Contains(name, "databackup") || strings.Contains(name, "logbackup") || strings.HasPrefix(name, "hana_") {
			foundDataBackup = true
		}
		if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
			n, copyErr := io.Copy(io.Discard, tr)
			if copyErr != nil {
				return report, fmt.Errorf("read entry %s: %w", header.Name, copyErr)
			}
			report.FileBytes += n
		}
	}
	if !foundDataBackup {
		return report, fmt.Errorf("HANA archive missing databackup/logbackup markers")
	}
	report.Detail = fmt.Sprintf("HANA 归档包含 %d 条目（%d 字节），已识别备份标志文件", report.TotalEntries, report.FileBytes)
	return report, nil
}
