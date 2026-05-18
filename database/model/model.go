package model

import "encoding/json"

type Setting struct {
	Id    uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

type Tls struct {
	Id     uint            `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name   string          `json:"name" form:"name"`
	Server json.RawMessage `json:"server" form:"server"`
	Client json.RawMessage `json:"client" form:"client"`
}

type User struct {
	Id         uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Username   string `json:"username" form:"username"`
	Password   string `json:"password" form:"password"`
	LastLogins string `json:"lastLogin"`
}

type Client struct {
	Id          uint            `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Enable      bool            `json:"enable" form:"enable"`
	Name        string          `json:"name" form:"name"`
	SubSecret   string          `json:"subSecret,omitempty" form:"subSecret" gorm:"index"`
	Config      json.RawMessage `json:"config,omitempty" form:"config"`
	Inbounds    json.RawMessage `json:"inbounds" form:"inbounds"`
	Links       json.RawMessage `json:"links,omitempty" form:"links"`
	Volume      int64           `json:"volume" form:"volume"`
	Expiry      int64           `json:"expiry" form:"expiry"`
	Down        int64           `json:"down" form:"down"`
	Up          int64           `json:"up" form:"up"`
	Desc        string          `json:"desc" form:"desc"`
	Group       string          `json:"group" form:"group"`
	LimitIP     int             `json:"limitIp" form:"limitIp" gorm:"default:0;not null"`
	IPLimitMode string          `json:"ipLimitMode" form:"ipLimitMode" gorm:"default:monitor;not null"`
	LastOnline  int64           `json:"lastOnline" form:"lastOnline" gorm:"default:0;not null"`
	LastIPCount int             `json:"lastIpCount" form:"lastIpCount" gorm:"default:0;not null"`

	// Delay start and periodic reset
	DelayStart bool  `json:"delayStart" form:"delayStart" gorm:"default:false;not null"`
	AutoReset  bool  `json:"autoReset" form:"autoReset" gorm:"default:false;not null"`
	ResetDays  int   `json:"resetDays" form:"resetDays" gorm:"default:0;not null"`
	NextReset  int64 `json:"nextReset" form:"nextReset" gorm:"default:0;not null"`
	TotalUp    int64 `json:"totalUp" form:"totalUp" gorm:"default:0;not null"`
	TotalDown  int64 `json:"totalDown" form:"totalDown" gorm:"default:0;not null"`
}

type ClientIP struct {
	Id         uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientName string `json:"clientName" gorm:"index:idx_client_ips_client_hash,unique"`
	// IP column kept empty for new rows; populated only on legacy backfill. ip_hash is the canonical lookup key.
	IP        string  `json:"ip"`
	IPHash    string  `json:"ipHash,omitempty" gorm:"index:idx_client_ips_client_hash,unique"`
	IPDisplay *string `json:"ipDisplay,omitempty"`
	FirstSeen int64   `json:"firstSeen"`
	LastSeen  int64   `json:"lastSeen" gorm:"index"`
}

type Stats struct {
	Id        uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	DateTime  int64  `json:"dateTime"`
	Resource  string `json:"resource"`
	Tag       string `json:"tag"`
	Direction bool   `json:"direction"`
	Traffic   int64  `json:"traffic"`
}

type Changes struct {
	Id       uint64          `json:"id" gorm:"primaryKey;autoIncrement"`
	DateTime int64           `json:"dateTime"`
	Actor    string          `json:"actor"`
	Key      string          `json:"key"`
	Action   string          `json:"action"`
	Obj      json.RawMessage `json:"obj"`
}

type AuditEvent struct {
	Id        uint64          `json:"id" gorm:"primaryKey;autoIncrement"`
	DateTime  int64           `json:"dateTime" gorm:"index"`
	Actor     string          `json:"actor" gorm:"index"`
	Event     string          `json:"event" gorm:"index"`
	Resource  string          `json:"resource"`
	Severity  string          `json:"severity" gorm:"index"`
	IP        string          `json:"ip"`
	UserAgent string          `json:"userAgent"`
	Details   json.RawMessage `json:"details"`
}

type Tokens struct {
	Id          uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Desc        string `json:"desc" form:"desc"`
	Token       string `json:"-" form:"token"`
	TokenHash   string `json:"-" gorm:"index"`
	TokenPrefix string `json:"tokenPrefix"`
	Scope       string `json:"scope" gorm:"default:admin;not null"`
	Enabled     bool   `json:"enabled" gorm:"default:true;not null"`
	Expiry      int64  `json:"expiry" form:"expiry"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
	LastUsedAt  int64  `json:"lastUsedAt"`
	LastUsedIP  string `json:"lastUsedIp"`
	UserId      uint   `json:"userId" form:"userId"`
	User        *User  `json:"user" gorm:"foreignKey:UserId;references:Id"`
}

type XUISyncProfile struct {
	Id             uint            `json:"id" gorm:"primaryKey;autoIncrement"`
	Name           string          `json:"name" gorm:"uniqueIndex"`
	SourceType     string          `json:"sourceType" gorm:"index"`
	SourceJSON     []byte          `json:"-" gorm:"column:source_json"`
	SourceSalt     []byte          `json:"-" gorm:"column:source_salt"`
	Strategy       string          `json:"strategy"`
	OnlyNew        bool            `json:"onlyNew" gorm:"default:true;not null"`
	Enabled        bool            `json:"enabled" gorm:"default:true;not null"`
	Schedule       string          `json:"schedule"`
	LastRunAt      int64           `json:"lastRunAt"`
	LastRunStatus  string          `json:"lastRunStatus"`
	LastRunSummary json.RawMessage `json:"lastRunSummary"`
	CreatedAt      int64           `json:"createdAt"`
	UpdatedAt      int64           `json:"updatedAt"`
}

func (XUISyncProfile) TableName() string {
	return "xui_sync_profiles"
}

type XUIKnownHost struct {
	Id          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Host        string `json:"host" gorm:"uniqueIndex"`
	Fingerprint string `json:"fingerprint"`
	PublicKey   string `json:"publicKey"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

func (XUIKnownHost) TableName() string {
	return "xui_known_hosts"
}
