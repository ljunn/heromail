package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ljunn/heromail/internal/buildinfo"
	"github.com/ljunn/heromail/internal/store"
)

type upgradeStatus struct {
	State     string    `json:"state"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
}

type upgradeRequest struct {
	Target      string    `json:"target"`
	RequestedAt time.Time `json:"requested_at"`
}

type versionView struct {
	CurrentVersion       string        `json:"current_version"`
	Commit               string        `json:"commit"`
	BuildTime            string        `json:"build_time"`
	Image                string        `json:"image"`
	OnlineUpgradeEnabled bool          `json:"online_upgrade_enabled"`
	Upgrade              upgradeStatus `json:"upgrade"`
	LatestRelease        releaseView   `json:"latest_release"`
}

type releaseView struct {
	Tag         string    `json:"tag"`
	Name        string    `json:"name"`
	Notes       string    `json:"notes"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

func (s *Server) adminVersion(c *gin.Context) {
	status := upgradeStatus{State: "idle", Message: "尚未执行在线升级"}
	if current, err := readUpgradeStatus(s.UpgradeStatusPath); err == nil {
		status = current
	}
	c.JSON(http.StatusOK, gin.H{"data": versionView{
		CurrentVersion:       buildinfo.Version,
		Commit:               buildinfo.Commit,
		BuildTime:            buildinfo.BuildTime,
		Image:                "ghcr.io/ljunn/heromail:latest",
		OnlineUpgradeEnabled: s.UpgradeRequestPath != "" && s.UpgradeStatusPath != "" && s.UpgradeBackup != nil,
		Upgrade:              status,
		LatestRelease:        latestGitHubRelease(c.Request.Context()),
	}})
}

func latestGitHubRelease(ctx context.Context) releaseView {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/ljunn/heromail/releases/latest", nil)
	if err != nil {
		return releaseView{}
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := (&http.Client{Timeout: 4 * time.Second}).Do(request)
	if err != nil {
		return releaseView{}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return releaseView{}
	}
	var payload struct {
		Tag         string    `json:"tag_name"`
		Name        string    `json:"name"`
		Notes       string    `json:"body"`
		URL         string    `json:"html_url"`
		PublishedAt time.Time `json:"published_at"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return releaseView{}
	}
	return releaseView{Tag: payload.Tag, Name: payload.Name, Notes: payload.Notes, URL: payload.URL, PublishedAt: payload.PublishedAt}
}

func (s *Server) adminUpgrade(c *gin.Context) {
	if s.UpgradeRequestPath == "" || s.UpgradeStatusPath == "" {
		writeError(c, http.StatusServiceUnavailable, "online_upgrade_disabled", "在线升级未启用")
		return
	}
	if current, err := readUpgradeStatus(s.UpgradeStatusPath); err == nil && (current.State == "backing_up" || current.State == "queued" || current.State == "updating") {
		writeError(c, http.StatusConflict, "upgrade_in_progress", "已有升级任务正在执行")
		return
	}
	targetVersion := strings.TrimSpace(c.GetHeader("X-HeroMail-Target-Version"))
	if targetVersion != "" && targetVersion != "latest" && sameReleaseVersion(buildinfo.Version, targetVersion) {
		writeError(c, http.StatusConflict, "already_latest", "当前已是最新正式版本")
		return
	}
	if s.UpgradeBackup == nil {
		writeError(c, http.StatusServiceUnavailable, "upgrade_backup_unavailable", "数据库备份未配置，在线升级已禁用")
		return
	}

	now := time.Now().UTC()
	status := upgradeStatus{State: "backing_up", Message: "正在创建升级前数据库备份", UpdatedAt: now}
	if err := atomicWriteJSON(s.UpgradeStatusPath, status); err != nil {
		writeError(c, http.StatusInternalServerError, "upgrade_status_failed", "无法写入升级状态")
		return
	}
	backupContext, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()
	if _, err := s.UpgradeBackup(backupContext); err != nil {
		failed := upgradeStatus{State: "failed", Message: "数据库备份失败，升级已取消", UpdatedAt: time.Now().UTC()}
		_ = atomicWriteJSON(s.UpgradeStatusPath, failed)
		if repository, ok := s.Store.(store.AccountRepository); ok {
			_ = repository.WriteAudit(demoUser(c), "system.upgrade.backup_failed", "system", "heromail", "在线升级前数据库备份失败，升级已取消", c.ClientIP())
		}
		writeError(c, http.StatusServiceUnavailable, "upgrade_backup_failed", "数据库备份失败，在线升级已取消")
		return
	}
	if repository, ok := s.Store.(store.AccountRepository); ok {
		_ = repository.WriteAudit(demoUser(c), "system.upgrade.backup", "system", "heromail", "在线升级前数据库备份完成", c.ClientIP())
	}
	status = upgradeStatus{State: "queued", Message: "升级前备份已完成，任务已进入队列", UpdatedAt: time.Now().UTC()}
	if err := atomicWriteJSON(s.UpgradeStatusPath, status); err != nil {
		failed := upgradeStatus{State: "failed", Message: "无法写入升级状态", UpdatedAt: time.Now().UTC()}
		_ = atomicWriteJSON(s.UpgradeStatusPath, failed)
		writeError(c, http.StatusInternalServerError, "upgrade_status_failed", "无法写入升级状态")
		return
	}
	target := targetVersion
	if target == "" {
		target = "latest"
	}
	request := upgradeRequest{Target: target, RequestedAt: now}
	if err := atomicWriteJSON(s.UpgradeRequestPath, request); err != nil {
		failed := upgradeStatus{State: "failed", Message: "无法创建升级任务", UpdatedAt: time.Now().UTC()}
		_ = atomicWriteJSON(s.UpgradeStatusPath, failed)
		writeError(c, http.StatusInternalServerError, "upgrade_request_failed", "无法创建升级任务")
		return
	}
	if repository, ok := s.Store.(store.AccountRepository); ok {
		_ = repository.WriteAudit(demoUser(c), "system.upgrade.request", "system", "heromail", "管理员提交在线升级任务", c.ClientIP())
	}
	c.JSON(http.StatusAccepted, gin.H{"data": status})
}

func sameReleaseVersion(current, target string) bool {
	normalize := func(value string) string {
		return strings.TrimPrefix(strings.TrimSpace(value), "v")
	}
	return normalize(current) != "" && normalize(current) == normalize(target)
}

func readUpgradeStatus(path string) (upgradeStatus, error) {
	if path == "" {
		return upgradeStatus{}, errors.New("升级状态路径为空")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return upgradeStatus{}, err
	}
	var status upgradeStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return upgradeStatus{}, err
	}
	return status, nil
}

func atomicWriteJSON(path string, value any) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".heromail-upgrade-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o640); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
