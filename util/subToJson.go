package util

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/admin8800/s-ui/logger"
	"github.com/admin8800/s-ui/util/common"
)

const maxExternalSubBytes = 4 << 20

func GetExternalLink(rawURL string) (string, error) {
	if err := validateExternalURL(rawURL); err != nil {
		logger.Warning("sub: invalid external URL:", err)
		return "", err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Get(rawURL)
	if err != nil {
		logger.Warning("sub: Error making HTTP request:", err)
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", common.NewErrorf("unexpected status code: %d", response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxExternalSubBytes+1))
	if err != nil {
		logger.Warning("sub: Error reading response body:", err)
		return "", err
	}
	if len(body) > maxExternalSubBytes {
		return "", common.NewError("response is too large")
	}

	data := StrOrBase64Encoded(string(body))
	return data, nil
}

func GetExternalSub(url string) ([]map[string]interface{}, error) {
	var err error
	var result []map[string]interface{}

	if len(url) == 0 {
		return nil, common.NewError("no url")
	}

	data, err := GetExternalLink(url)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, common.NewError("no result")
	}

	// if the data is a JSON object
	if strings.HasPrefix(data, "{") && strings.HasSuffix(data, "}") {
		var jsonData map[string]interface{}
		err = json.Unmarshal([]byte(data), &jsonData)
		if err != nil {
			logger.Warning("sub: Error unmarshalling JSON:", err)
			return nil, err
		}
		outbounds, ok := jsonData["outbounds"].([]any)
		if !ok {
			logger.Warning("sub: Error getting outbounds:", err)
			return nil, err
		}
		for _, outbound := range outbounds {
			outboundMap, ok := outbound.(map[string]interface{})
			if ok && len(outboundMap) > 0 {
				oType, _ := outboundMap["type"].(string)
				switch oType {
				case "urltest":
				case "direct":
				case "selector":
				case "block":
					continue
				default:
					result = append(result, outboundMap)
				}
			}
		}
		if len(result) == 0 {
			return nil, common.NewError("no result")
		}
		return result, nil
	} else {
		// if data is a text
		links := strings.Split(data, "\n")
		for _, link := range links {
			linkToJson, _, err := GetOutbound(link, 0)
			if err == nil {
				result = append(result, *linkToJson)
			}
		}
	}
	if len(result) == 0 {
		return nil, common.NewError("no result")
	}
	return result, nil
}

func validateExternalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return common.NewError("unsupported url scheme")
	}
	host := parsed.Hostname()
	if host == "" {
		return common.NewError("missing url host")
	}
	if strings.EqualFold(host, "localhost") {
		return common.NewError("localhost url is not allowed")
	}
	if os.Getenv("SUI_ALLOW_PRIVATE_SUB_URLS") == "true" {
		return nil
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if isBlockedExternalAddr(addr) {
			return common.NewError("private url host is not allowed")
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return common.NewError("url host did not resolve")
	}
	for _, ipAddr := range addrs {
		addr, ok := netip.AddrFromSlice(ipAddr.IP)
		if !ok || isBlockedExternalAddr(addr) {
			return common.NewError("private url host is not allowed")
		}
	}
	return nil
}

func isBlockedExternalAddr(addr netip.Addr) bool {
	return addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() || addr.IsUnspecified()
}
