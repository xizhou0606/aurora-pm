package designreview

import (
	"net/url"
	"regexp"
	"strings"
)

var figmaNodePattern = regexp.MustCompile(`^[0-9]+[:\-][0-9]+$`)

func ParseFigmaNodeURL(raw string) (fileKey, nodeID string, err error) {
	parsed, parseErr := url.Parse(strings.TrimSpace(raw))
	if parseErr != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "www.figma.com") || parsed.User != nil {
		return "", "", ErrInvalid
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 || (segments[0] != "design" && segments[0] != "file") || strings.TrimSpace(segments[1]) == "" {
		return "", "", ErrInvalid
	}
	nodeID = strings.ReplaceAll(strings.TrimSpace(parsed.Query().Get("node-id")), "-", ":")
	if !figmaNodePattern.MatchString(nodeID) {
		return "", "", ErrInvalid
	}
	return segments[1], nodeID, nil
}

func validHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}
