package validator

import (
	"net/url"
	"strings"
)

func IsYouTubeURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}

	host := strings.ToLower(u.Hostname())
	validHosts := []string{
		"youtube.com",
		"www.youtube.com",
		"m.youtube.com",
		"youtu.be",
	}

	for _, h := range validHosts {
		if host == h {
			return true
		}
	}

	return false
}

func IsInstagramURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}

	host := strings.ToLower(u.Hostname())
	validHosts := []string{
		"instagram.com",
		"www.instagram.com",
		"m.instagram.com",
	}

	for _, h := range validHosts {
		if host == h {
			return true
		}
	}

	return false
}

func IsFacebookURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}

	host := strings.ToLower(u.Hostname())
	validHosts := []string{
		"facebook.com",
		"www.facebook.com",
		"m.facebook.com",
		"web.facebook.com",
		"fb.watch",
	}

	for _, h := range validHosts {
		if host == h {
			return true
		}
	}

	return false
}
