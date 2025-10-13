package details

import (
	"net/http"

	pd "github.com/kodeart/go-problem/v2"
)

// TODO: Need to read the RFC to better understand what _exactly_ instance field is for,
//		 maybe we could use r.URL.Path

func Conflict(w http.ResponseWriter, detail string, extensions map[string]any) {
	p := pd.Problem{
		Status:     http.StatusConflict,
		Type:       "tag:example@example,2025:Conflict",
		Title:      http.StatusText(http.StatusConflict),
		Detail:     detail,
		Extensions: extensions,
	}

	p.JSON(w)
}

func BadRequest(w http.ResponseWriter, detail string, extensions map[string]any) {
	p := pd.Problem{
		Status:     http.StatusBadRequest,
		Type:       "tag:example@example,2025:BadRequest",
		Title:      http.StatusText(http.StatusBadRequest),
		Detail:     detail,
		Extensions: extensions,
	}

	p.JSON(w)
}

func UnsupportedMediaType(w http.ResponseWriter, detail string, supportedMedia []string) {
	p := pd.Problem{
		Status: http.StatusUnsupportedMediaType,
		Type:   "tag:example@example,2025:UnsupportedMediaType",
		Title:  http.StatusText(http.StatusUnsupportedMediaType),
		Detail: detail,
		Extensions: map[string]any{
			"supported-media": supportedMedia,
		},
	}

	p.JSON(w)
}

func MethodNotAllowed(w http.ResponseWriter, detail string, allowedMethods []string) {
	p := pd.Problem{
		Status: http.StatusMethodNotAllowed,
		Type:   "tag:example@example,2025:MethodNotAllowed",
		Title:  http.StatusText(http.StatusMethodNotAllowed),
		Detail: detail,
		Extensions: map[string]any{
			"allowed-methods": allowedMethods,
		},
	}

	p.JSON(w)
}

func NotFound(w http.ResponseWriter, detail string) {
	p := pd.Problem{
		Status: http.StatusNotFound,
		Type:   "tag:example@example,2025:NotFound",
		Title:  http.StatusText(http.StatusNotFound),
		Detail: detail,
	}
	p.JSON(w)
}

func Internal(w http.ResponseWriter, detail string, extensions map[string]any) {
	p := pd.Problem{
		Status:     http.StatusInternalServerError,
		Type:       "tag:example@example,2025:Internal",
		Title:      http.StatusText(http.StatusInternalServerError),
		Detail:     detail,
		Extensions: extensions,
	}

	p.JSON(w)
}
