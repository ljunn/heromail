package domain

import (
	"strings"
	"time"
)

type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type WalletLedger struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id,omitempty"`
	Type         string    `json:"type"`
	Amount       float64   `json:"amount"`
	BalanceAfter float64   `json:"balance_after"`
	OrderID      string    `json:"order_id,omitempty"`
	PaymentID    string    `json:"payment_order_id,omitempty"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

type AuditLog struct {
	ID           string    `json:"id"`
	ActorID      string    `json:"actor_id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	Detail       string    `json:"detail"`
	IP           string    `json:"ip"`
	CreatedAt    time.Time `json:"created_at"`
}

type MailboxPool struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Provider       string    `json:"provider"`
	Region         string    `json:"region"`
	Enabled        bool      `json:"enabled"`
	DailyLimit     int       `json:"daily_limit"`
	CooldownSecond int       `json:"cooldown_seconds"`
	MailboxCount   int64     `json:"mailbox_count"`
	CreatedAt      time.Time `json:"created_at"`
}

const (
	DefaultMailboxPoolName   = "邮箱池"
	MailboxProviderOutlook   = "outlook"
	MailboxProviderOutlookDE = "outlook_de"
	MailboxProviderHotmail   = "hotmail"
	MailboxProviderGmail     = "gmail"
	MailboxProviderICloud    = "icloud"
	MailboxProviderMailCom   = "mailcom"
)

var SupportedMailboxProviders = []string{
	MailboxProviderOutlook,
	MailboxProviderOutlookDE,
	MailboxProviderHotmail,
	MailboxProviderGmail,
	MailboxProviderICloud,
	MailboxProviderMailCom,
}

var mailComDomains = map[string]struct{}{
	"2trom.com": {}, "acdcfan.com": {}, "accountant.com": {}, "activist.com": {}, "adexec.com": {},
	"africamail.com": {}, "alumni.com": {}, "angelic.com": {}, "archaeologist.com": {}, "arcticmail.com": {},
	"artlover.com": {}, "asia.com": {}, "atheist.com": {}, "australiamail.com": {}, "bartender.net": {},
	"berlin.com": {}, "bikerider.com": {}, "birdlover.com": {}, "boardermail.com": {}, "brazilmail.com": {},
	"brew-master.com": {}, "bsdmail.com": {}, "californiamail.com": {}, "catlover.com": {}, "chef.net": {},
	"chemist.com": {}, "cheerful.com": {}, "chinamail.com": {}, "clubmember.org": {}, "collector.org": {},
	"columnist.com": {}, "comic.com": {}, "consultant.com": {}, "contractor.net": {}, "counsellor.com": {},
	"cutey.com": {}, "cyberdude.com": {}, "cybergal.com": {}, "cyberservices.com": {}, "cyber-wizard.com": {},
	"dallasmail.com": {}, "dbzmail.com": {}, "diplomats.com": {}, "discofan.com": {}, "doglover.com": {},
	"doramail.com": {}, "dr.com": {}, "dublin.com": {}, "dutchmail.com": {}, "email.com": {},
	"elvisfan.com": {}, "engineer.com": {}, "englandmail.com": {}, "europe.com": {}, "europemail.com": {},
	"execs.com": {}, "financier.com": {}, "fireman.net": {}, "galaxyhit.com": {}, "gardener.com": {},
	"geologist.com": {}, "germanymail.com": {}, "graduate.org": {}, "graphic-designer.com": {}, "greenmail.net": {},
	"hackermail.com": {}, "hairdresser.net": {}, "hilarious.com": {}, "hiphopfan.com": {}, "iname.com": {},
	"innocent.com": {}, "irelandmail.com": {}, "israelmail.com": {}, "italymail.com": {}, "keromail.com": {},
	"kissfans.com": {}, "kittymail.com": {}, "koreamail.com": {}, "legislator.com": {}, "linuxmail.org": {},
	"lobbyist.com": {}, "lovecat.com": {}, "madonnafan.com": {}, "mail.com": {},
	"marchmail.com": {}, "metalfan.com": {}, "mexicomail.com": {}, "minister.com": {}, "moscowmail.com": {},
	"munich.com": {}, "musician.org": {}, "muslim.com": {}, "myself.com": {}, "ninfan.com": {},
	"nonpartisan.com": {}, "null.net": {}, "nycmail.com": {}, "optician.com": {}, "orthodontist.net": {},
	"pediatrician.com": {}, "petlover.com": {}, "photographer.net": {}, "physicist.net": {}, "polandmail.com": {},
	"politician.com": {}, "post.com": {}, "priest.com": {}, "programmer.net": {}, "protestant.com": {},
	"publicist.com": {}, "ravemail.com": {}, "realtyagent.com": {}, "reborn.com": {}, "reggaefan.com": {},
	"registerednurses.com": {}, "reincarnate.com": {}, "religious.com": {}, "repairman.com": {}, "safrica.com": {},
	"saintly.com": {}, "sanfranmail.com": {}, "scotlandmail.com": {}, "secretary.net": {}, "socialworker.net": {},
	"sociologist.com": {}, "songwriter.net": {}, "spainmail.com": {}, "swedenmail.com": {}, "swissmail.com": {},
	"teachers.org": {}, "techie.com": {}, "technologist.com": {}, "theplate.com": {}, "therapist.net": {},
	"toothfairy.com": {}, "toke.com": {}, "torontomail.com": {}, "tvstar.com": {}, "usa.com": {},
	"uymail.com": {}, "webname.com": {},
}

// DetectMailboxProvider 按精确邮箱域名识别已配置的邮箱渠道。
func DetectMailboxProvider(address string) (string, bool) {
	address = strings.ToLower(strings.TrimSpace(address))
	separator := strings.LastIndexByte(address, '@')
	if separator < 1 || separator == len(address)-1 {
		return "", false
	}
	domainName := address[separator+1:]
	switch {
	case domainName == "outlook.de":
		return MailboxProviderOutlookDE, true
	case strings.HasPrefix(domainName, "outlook."):
		return MailboxProviderOutlook, true
	case strings.HasPrefix(domainName, "hotmail."):
		return MailboxProviderHotmail, true
	case domainName == "gmail.com" || domainName == "googlemail.com":
		return MailboxProviderGmail, true
	case domainName == "icloud.com" || domainName == "me.com" || domainName == "mac.com":
		return MailboxProviderICloud, true
	default:
		_, supported := mailComDomains[domainName]
		if supported {
			return MailboxProviderMailCom, true
		}
		return "", false
	}
}

func IsMicrosoftMailboxProvider(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case MailboxProviderOutlook, MailboxProviderOutlookDE, MailboxProviderHotmail:
		return true
	default:
		return false
	}
}

func IsSupportedMailboxProvider(provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	for _, supported := range SupportedMailboxProviders {
		if provider == supported {
			return true
		}
	}
	return false
}
