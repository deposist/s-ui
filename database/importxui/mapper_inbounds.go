package importxui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deposist/s-ui-rus-inst/database/model"
)

type xuiInboundSettings struct {
	Clients    []xuiClientSetting `json:"clients"`
	Accounts   []xuiAccount       `json:"accounts"`
	Method     string             `json:"method"`
	Password   string             `json:"password"`
	Network    string             `json:"network"`
	Encryption string             `json:"encryption"`
}

type xuiClientSetting struct {
	Comment    string `json:"comment"`
	Email      string `json:"email"`
	Enable     *bool  `json:"enable"`
	ExpiryTime int64  `json:"expiryTime"`
	Flow       string `json:"flow"`
	ID         string `json:"id"`
	LimitIP    int    `json:"limitIp"`
	SubID      string `json:"subId"`
	TgID       any    `json:"tgId"`
	TotalGB    int64  `json:"totalGB"`
	Password   string `json:"password"`
	Security   string `json:"security"`
}

type xuiAccount struct {
	User     string `json:"user"`
	Pass     string `json:"pass"`
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type ClientRef struct {
	SrcInboundID int64
	DstInboundID uint
	SrcTag       string
	Protocol     string
	Email        string
	UUID         string
	Password     string
	Flow         string
	Comment      string
	Enable       bool
	HasEnable    bool
	ExpiryTime   int64
	TotalGB      int64
	LimitIP      int
	SubID        string
	TgID         string
}

type mappedInbound struct {
	Inbound    model.Inbound
	ClientRefs []ClientRef
	Warnings   []string
}

func mapInbound(row xuiInboundRow, tlsID uint, reality *realitySpec, server string) (*mappedInbound, error) {
	var settings xuiInboundSettings
	if err := decodeJSON(row.Settings, &settings); err != nil {
		return nil, fmt.Errorf("inbound %s settings: %w", row.Tag, err)
	}
	stream, err := parseStreamSettings(row)
	if err != nil {
		return nil, err
	}
	switch stream.Network {
	case "kcp", "mkcp", "quic":
		return &mappedInbound{Warnings: []string{fmt.Sprintf("inbound %s: transport %q is unsupported by phase 2 importer; skipped", row.Tag, stream.Network)}}, nil
	}

	inType := inboundType(row.Protocol)
	if inType == "" {
		return &mappedInbound{Warnings: []string{fmt.Sprintf("inbound %s: unsupported protocol %q; skipped", row.Tag, row.Protocol)}}, nil
	}
	if inType == "http" && len(settings.Accounts) == 0 {
		return &mappedInbound{Warnings: []string{fmt.Sprintf("inbound %s: http has no accounts; skipped", row.Tag)}}, nil
	}

	transport, transportWarnings := mapTransport(row.Tag, stream)
	tlsBlock, tlsWarnings := mapOutboundTLSBlock(stream, reality)
	warnings := append(transportWarnings, tlsWarnings...)

	options := map[string]any{
		"listen":      listenAddress(row.Listen),
		"listen_port": row.Port,
	}
	if transport != nil {
		options["transport"] = transport
	}
	flow := firstClientFlow(settings.Clients)
	switch inType {
	case "shadowsocks":
		method := firstNonEmpty(settings.Method, "none")
		options["method"] = method
		if settings.Password != "" {
			options["password"] = settings.Password
		}
	case "vmess":
		options["security"] = "auto"
	}

	optionsJSON, err := marshalJSON(options)
	if err != nil {
		return nil, err
	}
	outJSON, err := buildOutJson(inType, row.Tag, server, row.Port, tlsBlock, transport, flow)
	if err != nil {
		return nil, err
	}
	refs := clientRefsFromSettings(row, inType, settings)
	return &mappedInbound{
		Inbound: model.Inbound{
			Type:    inType,
			Tag:     row.Tag,
			TlsId:   tlsID,
			Addrs:   buildAddrs(),
			OutJson: outJSON,
			Options: optionsJSON,
		},
		ClientRefs: refs,
		Warnings:   warnings,
	}, nil
}

func inboundType(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "vless":
		return "vless"
	case "vmess":
		return "vmess"
	case "trojan":
		return "trojan"
	case "shadowsocks":
		return "shadowsocks"
	case "http":
		return "http"
	case "socks":
		return "socks"
	default:
		return ""
	}
}

func listenAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "0.0.0.0"
	}
	return value
}

func firstClientFlow(clients []xuiClientSetting) string {
	for _, client := range clients {
		if strings.TrimSpace(client.Flow) != "" {
			return strings.TrimSpace(client.Flow)
		}
	}
	return ""
}

func clientRefsFromSettings(row xuiInboundRow, protocol string, settings xuiInboundSettings) []ClientRef {
	refs := make([]ClientRef, 0, len(settings.Clients)+len(settings.Accounts))
	for _, client := range settings.Clients {
		email := strings.TrimSpace(client.Email)
		if email == "" {
			continue
		}
		enable := row.Enable
		hasEnable := false
		if client.Enable != nil {
			enable = enable && *client.Enable
			hasEnable = true
		}
		ref := ClientRef{
			SrcInboundID: row.ID,
			SrcTag:       row.Tag,
			Protocol:     protocol,
			Email:        email,
			UUID:         strings.TrimSpace(client.ID),
			Password:     strings.TrimSpace(client.Password),
			Flow:         strings.TrimSpace(client.Flow),
			Comment:      client.Comment,
			Enable:       enable,
			HasEnable:    hasEnable || !row.Enable,
			ExpiryTime:   client.ExpiryTime,
			TotalGB:      client.TotalGB,
			LimitIP:      client.LimitIP,
			SubID:        strings.TrimSpace(client.SubID),
			TgID:         stringifyTgID(client.TgID),
		}
		refs = append(refs, ref)
	}
	for _, account := range settings.Accounts {
		user := firstNonEmpty(account.Email, account.User, account.Username)
		if user == "" {
			continue
		}
		refs = append(refs, ClientRef{
			SrcInboundID: row.ID,
			SrcTag:       row.Tag,
			Protocol:     protocol,
			Email:        user,
			Password:     firstNonEmpty(account.Pass, account.Password),
			Enable:       row.Enable,
			HasEnable:    !row.Enable,
		})
	}
	return refs
}

func stringifyTgID(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == 0 {
			return ""
		}
		return fmt.Sprintf("%.0f", v)
	case json.Number:
		return v.String()
	default:
		text := fmt.Sprint(v)
		if text == "0" {
			return ""
		}
		return strings.TrimSpace(text)
	}
}

func mapTransport(tag string, stream xuiStreamSettings) (map[string]any, []string) {
	network := strings.ToLower(strings.TrimSpace(stream.Network))
	switch network {
	case "", "tcp":
		return nil, nil
	case "ws":
		transport := map[string]any{"type": "ws"}
		if path, ok := stringFromMap(stream.WSSettings, "path"); ok && path != "" {
			transport["path"] = path
		}
		if headers, ok := mapFromMap(stream.WSSettings, "headers"); ok {
			if host, ok := stringFromMap(headers, "Host"); ok && host != "" {
				transport["headers"] = map[string]any{"Host": host}
			}
		}
		return transport, nil
	case "grpc":
		transport := map[string]any{"type": "grpc"}
		if serviceName, ok := stringFromMap(stream.GRPCSettings, "serviceName"); ok && serviceName != "" {
			transport["service_name"] = serviceName
		}
		return transport, nil
	case "h2", "http":
		transport := map[string]any{"type": "http"}
		if hosts, ok := stringSliceFromMap(stream.HTTPSettings, "host"); ok {
			transport["host"] = hosts
		}
		if path, ok := stringFromMap(stream.HTTPSettings, "path"); ok && path != "" {
			transport["path"] = path
		}
		return transport, nil
	case "httpupgrade":
		transport := map[string]any{"type": "httpupgrade"}
		if host, ok := stringFromMap(stream.HTTPUPSettings, "host"); ok && host != "" {
			transport["host"] = host
		}
		if path, ok := stringFromMap(stream.HTTPUPSettings, "path"); ok && path != "" {
			transport["path"] = path
		}
		return transport, nil
	case "splithttp", "xhttp":
		transport := map[string]any{"type": "httpupgrade"}
		if path, ok := stringFromMap(stream.HTTPUPSettings, "path"); ok && path != "" {
			transport["path"] = path
		}
		if host, ok := stringFromMap(stream.HTTPUPSettings, "host"); ok && host != "" {
			transport["host"] = host
		}
		return transport, []string{fmt.Sprintf("inbound %s: transport %q mapped to httpupgrade; manual review recommended", tag, network)}
	default:
		return nil, []string{fmt.Sprintf("inbound %s: transport %q requires manual review", tag, network)}
	}
}

func mapOutboundTLSBlock(stream xuiStreamSettings, reality *realitySpec) (map[string]any, []string) {
	switch stream.Security {
	case "":
		return nil, nil
	case "none":
		return nil, nil
	case "reality":
		if reality == nil {
			return nil, []string{"reality TLS settings are incomplete; outbound preview has no TLS block"}
		}
		return map[string]any{
			"enabled":     true,
			"server_name": reality.ServerName,
			"utls": map[string]any{
				"enabled":     true,
				"fingerprint": reality.Fingerprint,
			},
			"reality": map[string]any{
				"enabled":    true,
				"public_key": reality.PublicKey,
				"short_id":   firstString(reality.ShortIDs),
			},
		}, nil
	case "tls":
		return map[string]any{"enabled": true, "insecure": false}, []string{"non-reality TLS requires manual certificate/key upload"}
	default:
		return nil, []string{fmt.Sprintf("TLS security %q requires manual review", stream.Security)}
	}
}

func stringFromMap(values map[string]any, key string) (string, bool) {
	if values == nil {
		return "", false
	}
	value, ok := values[key]
	if !ok {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v), true
	default:
		return strings.TrimSpace(fmt.Sprint(v)), true
	}
}

func mapFromMap(values map[string]any, key string) (map[string]any, bool) {
	if values == nil {
		return nil, false
	}
	value, ok := values[key]
	if !ok {
		return nil, false
	}
	casted, ok := value.(map[string]any)
	return casted, ok
}

func stringSliceFromMap(values map[string]any, key string) ([]string, bool) {
	if values == nil {
		return nil, false
	}
	value, ok := values[key]
	if !ok {
		return nil, false
	}
	switch v := value.(type) {
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				result = append(result, text)
			}
		}
		return result, len(result) > 0
	case []string:
		return v, len(v) > 0
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, false
		}
		return []string{strings.TrimSpace(v)}, true
	default:
		text := strings.TrimSpace(fmt.Sprint(v))
		if text == "" {
			return nil, false
		}
		return []string{text}, true
	}
}
