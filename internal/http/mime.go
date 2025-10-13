package http

import (
	"mime"
	"strings"
)

const defaultSBOMVersion = "1.6"

func CheckContentType(contentType string) (bool, string) {
	if strings.TrimSpace(contentType) == "" {
		return false, ""
	}

	t, p, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false, ""
	}
	if t != "application/vnd.cyclonedx+json" {
		return false, ""
	}
	version, ok := p["version"]
	if !ok {
		version = defaultSBOMVersion
	}

	return true, version
}
