package eventsvc

import (
	"net"
	"net/http"
	"strings"
)

func extractIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		ips := strings.Split(ip, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func summarizeUserAgent(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "unknown"
	}

	lower := strings.ToLower(raw)
	browser, version, engine := detectBrowser(raw, lower)
	osName := detectOS(lower)

	if version == "" {
		version = "?"
	}
	if engine == "" {
		engine = "unknown"
	}
	if osName == "" {
		osName = "unknown"
	}

	return strings.TrimSpace(browser + " " + version + " / " + engine + " / " + osName)
}

func extractVersion(raw, marker string) string {
	idx := strings.Index(raw, marker)
	if idx == -1 {
		return ""
	}

	start := idx + len(marker)
	end := start
	for end < len(raw) {
		switch raw[end] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.', '_':
			end++
		default:
			return raw[start:end]
		}
	}
	return raw[start:end]
}

func detectBrowser(raw, lower string) (string, string, string) {
	switch {
	case strings.Contains(lower, "edg/"):
		return "Edge", extractVersion(raw, "Edg/"), "V8"
	case strings.Contains(lower, "opr/") || strings.Contains(lower, "opera"):
		version := extractVersion(raw, "OPR/")
		if version == "" {
			version = extractVersion(raw, "Opera/")
		}
		return "Opera", version, "V8"
	case strings.Contains(lower, "crios/"):
		return "Chrome iOS", extractVersion(raw, "CriOS/"), "JavaScriptCore"
	case strings.Contains(lower, "fxios/"):
		return "Firefox iOS", extractVersion(raw, "FxiOS/"), "JavaScriptCore"
	case strings.Contains(lower, "chrome/") && !strings.Contains(lower, "chromium"):
		return "Chrome", extractVersion(raw, "Chrome/"), "V8"
	case strings.Contains(lower, "firefox/"):
		return "Firefox", extractVersion(raw, "Firefox/"), "SpiderMonkey"
	case strings.Contains(lower, "safari/") && strings.Contains(lower, "version/") && !strings.Contains(lower, "chrome"):
		return "Safari", extractVersion(raw, "Version/"), "JavaScriptCore"
	case strings.Contains(lower, "postmanruntime/"):
		return "Postman", extractVersion(raw, "PostmanRuntime/"), "Node.js"
	case strings.Contains(lower, "curl/"):
		return "curl", extractVersion(raw, "curl/"), "libcurl"
	default:
		token := raw
		if space := strings.Index(raw, " "); space > 0 {
			token = raw[:space]
		}
		return strings.TrimSpace(token), "", "unknown"
	}
}

func detectOS(lower string) string {
	switch {
	case strings.Contains(lower, "windows nt"):
		version := extractVersionToken(lower, "windows nt ")
		if friendly := windowsVersionName(version); friendly != "" {
			return friendly
		}
		if version != "" {
			return "Windows NT " + version
		}
		return "Windows"
	case strings.Contains(lower, "mac os x"):
		version := strings.ReplaceAll(extractVersionToken(lower, "mac os x "), "_", ".")
		if version != "" {
			return "macOS " + version
		}
		return "macOS"
	case strings.Contains(lower, "iphone os"):
		version := strings.ReplaceAll(extractVersionToken(lower, "iphone os "), "_", ".")
		if version != "" {
			return "iOS " + version
		}
		return "iOS"
	case strings.Contains(lower, "ipad; cpu os"):
		version := strings.ReplaceAll(extractVersionToken(lower, "cpu os "), "_", ".")
		if version != "" {
			return "iPadOS " + version
		}
		return "iPadOS"
	case strings.Contains(lower, "android"):
		version := extractVersionToken(lower, "android ")
		if version != "" {
			return "Android " + version
		}
		return "Android"
	case strings.Contains(lower, "linux"):
		return "Linux"
	default:
		return ""
	}
}

func extractVersionToken(lower, marker string) string {
	idx := strings.Index(lower, marker)
	if idx == -1 {
		return ""
	}
	start := idx + len(marker)
	end := start
	for end < len(lower) {
		ch := lower[end]
		if (ch >= '0' && ch <= '9') || ch == '.' || ch == '_' {
			end++
		} else {
			break
		}
	}
	return lower[start:end]
}

func windowsVersionName(nt string) string {
	switch strings.TrimSpace(nt) {
	case "10.0":
		return "Windows 10"
	case "11.0":
		return "Windows 11"
	case "6.3":
		return "Windows 8.1"
	case "6.2":
		return "Windows 8"
	case "6.1":
		return "Windows 7"
	case "6.0":
		return "Windows Vista"
	case "5.1":
		return "Windows XP"
	default:
		return ""
	}
}
