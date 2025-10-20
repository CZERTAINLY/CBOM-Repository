package http

import (
	"mime"
	"strings"
)

const defaultBOMVersion = "1.6"

// HeaderContentType is the canonical key used when reading the request header for content type.
const HeaderContentType = "content-type"

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
		version = defaultBOMVersion
	}

	return true, version
}
