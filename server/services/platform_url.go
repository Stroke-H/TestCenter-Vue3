package services

import (
	"net/url"
	"os"
	"strings"
)

const defaultPlatformLANHost = "www.inspdance.com"

func platformLANHost() string {
	host := strings.TrimSpace(os.Getenv("TESTCENTER_LAN_HOST"))
	if host == "" {
		return defaultPlatformLANHost
	}

	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	return strings.TrimRight(host, "/")
}

func envOrigin(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return strings.TrimRight(value, "/")
}

func PlatformFrontendOrigin() string {
	return envOrigin("TESTCENTER_FRONTEND_ORIGIN", "http://"+platformLANHost()+":5173")
}

func PlatformBackendOrigin() string {
	return envOrigin("TESTCENTER_BACKEND_ORIGIN", "http://"+platformLANHost()+":8080")
}

func PlatformFrontendURL(path string) string {
	return joinPlatformURL(PlatformFrontendOrigin(), path)
}

func PlatformBackendURL(path string) string {
	return joinPlatformURL(PlatformBackendOrigin(), path)
}

func joinPlatformURL(origin string, path string) string {
	base, err := url.Parse(strings.TrimRight(origin, "/"))
	if err != nil {
		return strings.TrimRight(origin, "/") + "/" + strings.TrimLeft(path, "/")
	}

	ref, err := url.Parse(path)
	if err != nil {
		base.Path = strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(path, "/")
		return base.String()
	}

	return base.ResolveReference(ref).String()
}
