package mail

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strings"

	"github.com/ljunn/heromail/internal/domain"
)

const maxMailboxImportLineBytes = 1024 * 1024

var microsoftClientIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var ErrMailboxImportHeader = errors.New("邮箱导入表头")

type MailboxImportRecord struct {
	Address      string
	Provider     string
	Password     string
	TOTPURL      string
	ClientID     string
	ClientSecret string
	AccessToken  string
	RefreshToken string
	// ConnectionMethod 记录导入凭据实际使用的收件协议。
	ConnectionMethod string
	OAuthProtocol    string
}

func (r MailboxImportRecord) Credential() map[string]string {
	values := map[string]string{
		"password":       r.Password,
		"totp_url":       r.TOTPURL,
		"client_id":      r.ClientID,
		"client_secret":  r.ClientSecret,
		"access_token":   r.AccessToken,
		"refresh_token":  r.RefreshToken,
		"oauth_protocol": r.OAuthProtocol,
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result[key] = value
		}
	}
	return result
}

type MailboxLineParser interface {
	Parse(line string) (MailboxImportRecord, error)
}

type mailboxLineParser struct{}

func NewMailboxLineParser() MailboxLineParser { return mailboxLineParser{} }

func (mailboxLineParser) Parse(line string) (MailboxImportRecord, error) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
	if line == "" || strings.HasPrefix(line, "#") {
		return MailboxImportRecord{}, ErrMailboxImportHeader
	}
	if strings.HasPrefix(line, "{") {
		return parseMailboxJSONLine(line)
	}
	fields, err := splitMailboxImportFields(line)
	if err != nil {
		return MailboxImportRecord{}, err
	}
	if len(fields) > 0 && (strings.EqualFold(fields[0], "email") || strings.EqualFold(fields[0], "address")) {
		return MailboxImportRecord{}, ErrMailboxImportHeader
	}
	return mailboxRecordFromFields(fields)
}

func parseMailboxJSONLine(line string) (MailboxImportRecord, error) {
	var values map[string]string
	if err := json.Unmarshal([]byte(line), &values); err != nil {
		return MailboxImportRecord{}, errors.New("JSON Lines 格式无效")
	}
	record := MailboxImportRecord{
		Address:          firstImportValue(values, "email", "address", "username"),
		Password:         firstImportValue(values, "password", "pass"),
		TOTPURL:          firstImportValue(values, "totp_url", "totp", "otp_url", "two_factor_url", "2fa_url"),
		ClientID:         firstImportValue(values, "client_id", "clientid"),
		ClientSecret:     firstImportValue(values, "client_secret", "clientsecret"),
		AccessToken:      firstImportValue(values, "access_token", "accesstoken"),
		RefreshToken:     firstImportValue(values, "refresh_token", "refreshtoken"),
		ConnectionMethod: firstImportValue(values, "connection_method", "connection"),
		OAuthProtocol:    firstImportValue(values, "oauth_protocol", "oauth_type", "protocol", "auth_type"),
	}
	return normalizeMailboxImportRecord(record)
}

func firstImportValue(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func splitMailboxImportFields(line string) ([]string, error) {
	for _, delimiter := range []string{"----", "\t", "|"} {
		if strings.Contains(line, delimiter) {
			return trimImportFields(strings.Split(line, delimiter)), nil
		}
	}
	if strings.Contains(line, ",") {
		reader := csv.NewReader(strings.NewReader(line))
		reader.TrimLeadingSpace = true
		fields, err := reader.Read()
		if err != nil {
			return nil, errors.New("CSV 格式无效")
		}
		return trimImportFields(fields), nil
	}
	if strings.Contains(line, ":") {
		return trimImportFields(strings.SplitN(line, ":", 2)), nil
	}
	return nil, errors.New("无法识别邮箱格式")
}

func trimImportFields(fields []string) []string {
	for index := range fields {
		fields[index] = strings.TrimSpace(fields[index])
	}
	return fields
}

func mailboxRecordFromFields(fields []string) (MailboxImportRecord, error) {
	if len(fields) < 2 {
		return MailboxImportRecord{}, errors.New("邮箱记录至少需要邮箱和凭证")
	}
	record := MailboxImportRecord{Address: fields[0], Password: fields[1]}
	for _, value := range fields[2:] {
		if value == "" {
			continue
		}
		if key, fieldValue, found := strings.Cut(value, "="); found {
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "client_id", "clientid":
				record.ClientID = fieldValue
			case "client_secret", "clientsecret":
				record.ClientSecret = fieldValue
			case "access_token", "accesstoken":
				record.AccessToken = fieldValue
			case "refresh_token", "refreshtoken":
				record.RefreshToken = fieldValue
			case "totp_url", "totp", "otp_url", "two_factor_url", "2fa_url":
				record.TOTPURL = fieldValue
			case "connection_method", "connection":
				record.ConnectionMethod = fieldValue
			case "oauth_protocol", "oauth_type", "protocol", "auth_type":
				record.OAuthProtocol = fieldValue
			}
			continue
		}
		switch {
		case microsoftClientIDPattern.MatchString(value):
			record.ClientID = value
		case isTOTPURL(value):
			record.TOTPURL = value
		case record.RefreshToken == "":
			record.RefreshToken = value
		case record.AccessToken == "":
			record.AccessToken = value
		}
	}
	return normalizeMailboxImportRecord(record)
}

func isTOTPURL(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(strings.ToLower(value), "https://2fa.live/")
}

func normalizeMailboxImportRecord(record MailboxImportRecord) (MailboxImportRecord, error) {
	record.Address = strings.ToLower(strings.TrimSpace(record.Address))
	parsed, err := mail.ParseAddress(record.Address)
	if err != nil || !strings.EqualFold(parsed.Address, record.Address) {
		return MailboxImportRecord{}, errors.New("邮箱地址无效")
	}
	provider, supported := domain.DetectMailboxProvider(record.Address)
	if !supported {
		return MailboxImportRecord{}, errors.New("不支持该邮箱类型")
	}
	record.Provider = provider
	record.ConnectionMethod = normalizeImportConnectionMethod(record.ConnectionMethod)
	record.OAuthProtocol = normalizeImportOAuthProtocol(record.OAuthProtocol)
	if domain.IsMicrosoftMailboxProvider(provider) {
		// 旧迁移文件没有协议字段，但源项目导出的 refresh token 是
		// Outlook IMAP OAuth 凭据。按该协议兼容处理。
		if record.OAuthProtocol == "" && record.ClientID != "" && record.RefreshToken != "" {
			record.OAuthProtocol = "imap"
		}
	}
	if len(record.Credential()) == 0 {
		return MailboxImportRecord{}, errors.New("邮箱凭证不能为空")
	}
	return record, nil
}

func normalizeImportConnectionMethod(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case domain.MailboxConnectionIMAP:
		return domain.MailboxConnectionIMAP
	case domain.MailboxConnectionMicrosoftGraph:
		return domain.MailboxConnectionMicrosoftGraph
	case domain.MailboxConnectionMicrosoftOAuth:
		return domain.MailboxConnectionMicrosoftOAuth
	default:
		return domain.MailboxConnectionAuto
	}
}

func normalizeImportOAuthProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "imap", "imap_oauth", "microsoft_imap", "microsoft_imap_oauth":
		return "imap"
	case "graph", "microsoft_graph":
		return "graph"
	default:
		return ""
	}
}

type MailboxImportLineError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

type MailboxImportResult struct {
	Imported int                      `json:"imported"`
	Failed   int                      `json:"failed"`
	Skipped  int                      `json:"skipped"`
	Errors   []MailboxImportLineError `json:"errors"`
}

func StreamMailboxImport(reader io.Reader, parser MailboxLineParser, save func(MailboxImportRecord) error) (MailboxImportResult, error) {
	result := MailboxImportResult{Errors: make([]MailboxImportLineError, 0)}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), maxMailboxImportLineBytes)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		record, err := parser.Parse(scanner.Text())
		if errors.Is(err, ErrMailboxImportHeader) {
			result.Skipped++
			continue
		}
		if err == nil {
			err = save(record)
		}
		if err != nil {
			result.Failed++
			if len(result.Errors) < 100 {
				result.Errors = append(result.Errors, MailboxImportLineError{Line: lineNumber, Message: err.Error()})
			}
			continue
		}
		result.Imported++
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("读取 TXT 文件失败：%w", err)
	}
	return result, nil
}
