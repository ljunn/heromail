package httpapi

import (
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostgresUpgradeBackupCreatesVerifiedArchive(t *testing.T) {
	directory := t.TempDir()
	commandPath := filepath.Join(directory, "pg_dump")
	if err := os.WriteFile(commandPath, []byte("#!/bin/sh\nprintf 'CREATE TABLE demo (id text);\\n'\n"), 0o700); err != nil {
		t.Fatalf("创建 pg_dump 测试替身失败：%v", err)
	}

	backup := newPostgresUpgradeBackup("postgres://user:secret@postgres:5432/heromail?sslmode=disable", directory, commandPath)
	backupPath, err := backup(context.Background())
	if err != nil {
		t.Fatalf("创建升级前备份失败：%v", err)
	}
	if !strings.Contains(filepath.Base(backupPath), "heromail-preupgrade-") || !strings.HasSuffix(backupPath, ".sql.gz") {
		t.Fatalf("备份文件名不正确：%s", backupPath)
	}
	file, err := os.Open(backupPath)
	if err != nil {
		t.Fatalf("打开备份文件失败：%v", err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("打开 gzip 备份失败：%v", err)
	}
	content, err := io.ReadAll(reader)
	if closeErr := reader.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("读取 gzip 备份失败：%v", err)
	}
	if string(content) != "CREATE TABLE demo (id text);\n" {
		t.Fatalf("备份内容不正确：%q", content)
	}
}

func TestPostgresUpgradeBackupRemovesPartialFileOnFailure(t *testing.T) {
	directory := t.TempDir()
	commandPath := filepath.Join(directory, "pg_dump")
	if err := os.WriteFile(commandPath, []byte("#!/bin/sh\nprintf 'partial'\nexit 1\n"), 0o700); err != nil {
		t.Fatalf("创建 pg_dump 测试替身失败：%v", err)
	}

	backup := newPostgresUpgradeBackup("postgres://user:secret@postgres:5432/heromail?sslmode=disable", directory, commandPath)
	if _, err := backup(context.Background()); err == nil {
		t.Fatal("pg_dump 失败时备份不应成功")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("读取备份目录失败：%v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "heromail-preupgrade-") || strings.HasPrefix(entry.Name(), ".heromail-backup-") {
			t.Fatalf("pg_dump 失败后残留备份文件：%s", entry.Name())
		}
	}
}

func TestPostgresDumpEnvironmentParsesConnectionURL(t *testing.T) {
	environment := postgresDumpEnvironment("postgres://report:secret%21@db.internal:5433/heromail?sslmode=require")
	values := make(map[string]string)
	for _, item := range environment {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			values[parts[0]] = parts[1]
		}
	}
	for key, want := range map[string]string{
		"PGHOST": "db.internal", "PGPORT": "5433", "PGUSER": "report", "PGPASSWORD": "secret!", "PGDATABASE": "heromail", "PGSSLMODE": "require",
	} {
		if values[key] != want {
			t.Fatalf("环境变量 %s = %q，期望 %q", key, values[key], want)
		}
	}
}
