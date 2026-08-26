package mail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGoogleClientBuildsAuthorizationURLAndExchangesToken(t *testing.T) {
	var tokenCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			if r.URL.Path == "/userinfo" {
				if r.Header.Get("Authorization") != "Bearer access-token" {
					http.Error(w, "missing authorization", http.StatusUnauthorized)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"email": "Google-Test@gmail.com", "email_verified": true})
				return
			}
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		tokenCalls++
		if tokenCalls == 1 && r.Form.Get("grant_type") != "authorization_code" {
			http.Error(w, "invalid grant type", http.StatusBadRequest)
			return
		}
		if tokenCalls == 2 && r.Form.Get("grant_type") != "refresh_token" {
			http.Error(w, "invalid grant type", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-token", "refresh_token": "refresh-token", "expires_in": 3600})
	}))
	defer server.Close()

	client := NewGoogleClient(GoogleConfig{
		ClientID:         "client-id",
		ClientSecret:     "client-secret",
		RedirectURI:      "https://heromail.cc/api/v1/oauth/google/callback",
		AuthorizationURL: server.URL + "/authorize",
		TokenURL:         server.URL + "/token",
		UserInfoURL:      server.URL + "/userinfo",
	})
	if !client.Enabled() {
		t.Fatal("完整配置未启用 Google OAuth")
	}
	auth, err := url.Parse(client.AuthURL("state-value"))
	if err != nil {
		t.Fatalf("授权地址无效：%v", err)
	}
	query := auth.Query()
	if query.Get("client_id") != "client-id" || query.Get("state") != "state-value" || query.Get("access_type") != "offline" || !strings.Contains(query.Get("scope"), "https://mail.google.com/") {
		t.Fatalf("授权参数错误：%v", query)
	}
	credential, _, err := client.Exchange(context.Background(), "authorization-code")
	if err != nil || credential["access_token"] != "access-token" || credential["refresh_token"] != "refresh-token" {
		t.Fatalf("授权码交换失败：credential=%v err=%v", credential, err)
	}
	refreshed, _, err := client.RefreshCredential(context.Background(), credential)
	if err != nil || refreshed["access_token"] != "access-token" {
		t.Fatalf("Token 刷新失败：credential=%v err=%v", refreshed, err)
	}
	profile, err := client.Profile(context.Background(), credential["access_token"])
	if err != nil || profile.Address != "google-test@gmail.com" {
		t.Fatalf("Google 账户信息读取失败：profile=%+v err=%v", profile, err)
	}
}
