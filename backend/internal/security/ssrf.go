package security

import (
	"net"
	"net/url"
	"strings"
)

func IsSafeHost(host string) bool {
	if host == "" {
		return false
	}
	host = strings.Trim(host, "[]")
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
			return false
		}
	}
	return true
}

func IsSafeURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return IsSafeHost(u.Hostname())
}

func IsSafePathOrURL(pathOrURL string) bool {
	if pathOrURL == "" {
		return true
	}
	if strings.HasPrefix(pathOrURL, "ssh://") {
		hostPart := strings.SplitN(pathOrURL[6:], "/", 2)[0]
		host := strings.Split(hostPart, "@")
		h := host[len(host)-1]
		if idx := strings.Index(h, ":"); idx >= 0 {
			h = h[:idx]
		}
		return IsSafeHost(strings.Trim(h, "[]"))
	}
	if strings.HasPrefix(pathOrURL, "s3://") {
		return true
	}
	return true
}

var browseAllowedPrefixes = []string{"/app/data", "/app/backup", "/backup"}

func IsSafeBrowsePath(path string) bool {
	if path == "" {
		return true
	}
	clean := strings.TrimRight(path, "/")
	if clean == "" {
		return true
	}
	for _, prefix := range browseAllowedPrefixes {
		if clean == prefix || strings.HasPrefix(clean+"/", prefix+"/") {
			return true
		}
	}
	return false
}
