package httpapi

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ljunn/heromail/internal/buildinfo"
)

type upgradeBackupFunc func(context.Context) (string, error)

func newPostgresUpgradeBackup(databaseURL, backupDir, pgDumpCommand string) upgradeBackupFunc {
	databaseURL = strings.TrimSpace(databaseURL)
	backupDir = strings.TrimSpace(backupDir)
	pgDumpCommand = strings.TrimSpace(pgDumpCommand)
	if databaseURL == "" || backupDir == "" || pgDumpCommand == "" {
		return nil
	}
	return func(ctx context.Context) (string, error) {
		return createPostgresUpgradeBackup(ctx, databaseURL, backupDir, pgDumpCommand)
	}
}

func createPostgresUpgradeBackup(ctx context.Context, databaseURL, backupDir, pgDumpCommand string) (string, error) {
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %w", err)
	}
	temporary, err := os.CreateTemp(backupDir, ".heromail-backup-*.sql.gz")
	if err != nil {
		return "", fmt.Errorf("创建临时备份文件失败: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return "", fmt.Errorf("设置备份文件权限失败: %w", err)
	}

	compressed := gzip.NewWriter(temporary)
	command := exec.CommandContext(ctx, pgDumpCommand, "--no-owner", "--no-privileges")
	command.Env = postgresDumpEnvironment(databaseURL)
	command.Stdout = compressed
	if err := command.Run(); err != nil {
		compressed.Close()
		temporary.Close()
		return "", fmt.Errorf("pg_dump 执行失败: %w", err)
	}
	if err := compressed.Close(); err != nil {
		temporary.Close()
		return "", fmt.Errorf("压缩数据库备份失败: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("写入数据库备份失败: %w", err)
	}
	if err := verifyGzipFile(temporaryPath); err != nil {
		return "", fmt.Errorf("校验数据库备份失败: %w", err)
	}

	filename := fmt.Sprintf("heromail-preupgrade-%s-%s.sql.gz", backupVersionToken(buildinfo.Version), time.Now().UTC().Format("20060102T150405.000000000Z"))
	finalPath := filepath.Join(backupDir, filename)
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return "", fmt.Errorf("保存数据库备份失败: %w", err)
	}
	return finalPath, nil
}

func postgresDumpEnvironment(databaseURL string) []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "PGDATABASE=") {
			environment = append(environment, item)
		}
	}
	return append(environment, "PGDATABASE="+databaseURL)
}

func verifyGzipFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		reader.Close()
		return err
	}
	return reader.Close()
}

func backupVersionToken(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "unknown"
	}
	var token strings.Builder
	for _, character := range version {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '.' || character == '-' || character == '_' {
			token.WriteRune(character)
		} else {
			token.WriteByte('-')
		}
	}
	if token.Len() == 0 {
		return "unknown"
	}
	return token.String()
}
