package mail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// GoogleConfig 配置 Gmail OAuth 2.0 Web 应用客户端。
type GoogleConfig struct {
	ClientID         string
	ClientSecret     string
	RedirectURI      string
	AuthorizationURL string
	TokenURL         string
	UserInfoURL      string
}

// GoogleClient 负责 Google OAuth 授权和 Gmail IMAP OAuth Token 管理。
type GoogleClient struct {
	mu     sync.RWMutex
	config GoogleConfig
	http   *http.Client
}

func NewGoogleClient(config GoogleConfig) *GoogleClient {
	if config.AuthorizationURL == "" {
		config.AuthorizationURL = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	if config.TokenURL == "" {
		config.TokenURL = "https://oauth2.googleapis.com/token"
	}
	if config.UserInfoURL == "" {
		config.UserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	}
	return &GoogleClient{config: config, http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *GoogleClient) Enabled() bool {
	if c == nil {
		return false
	}
	config := c.configSnapshot()
	return config.ClientID != "" && config.ClientSecret != "" && config.RedirectURI != ""
}

func (c *GoogleClient) AuthURL(state string) string {
	config := c.configSnapshot()
	values := url.Values{
		"client_id":              {config.ClientID},
		"redirect_uri":           {config.RedirectURI},
		"response_type":          {"code"},
		"scope":                  {"openid email profile https://mail.google.com/"},
		"access_type":            {"offline"},
		"prompt":                 {"consent"},
		"include_granted_scopes": {"true"},
		"state":                  {state},
	}
	return config.AuthorizationURL + "?" + values.Encode()
}

func (c *GoogleClient) Exchange(ctx context.Context, code string) (map[string]string, time.Time, error) {
	config := c.configSnapshot()
	return c.token(ctx, url.Values{
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {config.RedirectURI},
	})
}

// RefreshCredential 实现 Gmail IMAP OAuth Token 的自动刷新。
func (c *GoogleClient) RefreshCredential(ctx context.Context, credential map[string]string) (map[string]string, time.Time, error) {
	refreshToken := strings.TrimSpace(credential["refresh_token"])
	if c == nil || strings.TrimSpace(refreshToken) == "" {
		return nil, time.Time{}, errors.New("缺少 Google OAuth refresh token")
	}
	config := c.configSnapshot()
	if config.ClientID == "" || config.ClientSecret == "" {
		return nil, time.Time{}, errors.New("Google OAuth 尚未配置")
	}
	return c.token(ctx, url.Values{
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

func (c *GoogleClient) Profile(ctx context.Context, accessToken string) (Profile, error) {
	if strings.TrimSpace(accessToken) == "" {
		return Profile{}, errors.New("缺少 Google access token")
	}
	config := c.configSnapshot()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, config.UserInfoURL, nil)
	if err != nil {
		return Profile{}, err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := c.http.Do(request)
	if err != nil {
		return Profile{}, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusOK {
		return Profile{}, fmt.Errorf("Google 用户信息接口返回 %d：%s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.Email) == "" {
		return Profile{}, errors.New("Google 用户信息响应无效")
	}
	if !payload.EmailVerified {
		return Profile{}, errors.New("Google 邮箱尚未验证")
	}
	return Profile{Address: strings.ToLower(strings.TrimSpace(payload.Email))}, nil
}

func (c *GoogleClient) token(ctx context.Context, values url.Values) (map[string]string, time.Time, error) {
	config := c.configSnapshot()
	if config.TokenURL == "" {
		return nil, time.Time{}, errors.New("Google OAuth 尚未配置")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, config.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, time.Time{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusOK {
		return nil, time.Time{}, fmt.Errorf("Google Token 接口返回 %d：%s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.AccessToken) == "" {
		return nil, time.Time{}, errors.New("Google Token 响应无效")
	}
	validUntil := time.Now().UTC().Add(time.Duration(payload.ExpiresIn) * time.Second)
	credential := map[string]string{
		"access_token": payload.AccessToken,
		"expires_at":   validUntil.Format(time.RFC3339),
	}
	if payload.RefreshToken != "" {
		credential["refresh_token"] = payload.RefreshToken
	}
	return credential, validUntil, nil
}

// Configure 在服务运行期间更新 OAuth 客户端配置，避免修改配置后重启服务。
func (c *GoogleClient) Configure(config GoogleConfig) {
	if c == nil {
		return
	}
	if config.AuthorizationURL == "" {
		config.AuthorizationURL = "https://accounts.google.com/o/oauth2/v2/auth"
	}
	if config.TokenURL == "" {
		config.TokenURL = "https://oauth2.googleapis.com/token"
	}
	if config.UserInfoURL == "" {
		config.UserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	}
	c.mu.Lock()
	c.config = config
	c.mu.Unlock()
}

// ConfigSummary 返回不包含客户端密钥的配置摘要。
func (c *GoogleClient) ConfigSummary() (clientID, redirectURI string, configured bool) {
	if c == nil {
		return "", "", false
	}
	config := c.configSnapshot()
	return config.ClientID, config.RedirectURI, config.ClientID != "" && config.ClientSecret != "" && config.RedirectURI != ""
}

func (c *GoogleClient) configSnapshot() GoogleConfig {
	if c == nil {
		return GoogleConfig{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config
}
