// Package profiles is the device-profile domain: pure Go business logic
// wired to storage and transport through the Repo port.
package profiles

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type DeviceType string

const (
	DeviceDesktop DeviceType = "desktop"
	DeviceMobile  DeviceType = "mobile"
)

func (d DeviceType) Valid() bool { return d == DeviceDesktop || d == DeviceMobile }

type Header struct {
	Key   string
	Value string
}

type Profile struct {
	ID            string
	UserID        string
	Name          string
	DeviceType    DeviceType
	WindowWidth   int
	WindowHeight  int
	UserAgent     string
	CountryCode   string
	CustomHeaders []Header
	Extra         map[string]any
	TemplateSlug  *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Input struct {
	Name          string
	DeviceType    DeviceType
	WindowWidth   int
	WindowHeight  int
	UserAgent     string
	CountryCode   string
	CustomHeaders []Header
	Extra         map[string]any
	TemplateSlug  *string // if set, Create fills unset fields from the template
}

// Patch is a partial update: nil fields are left unchanged.
type Patch struct {
	Name          *string
	DeviceType    *DeviceType
	WindowWidth   *int
	WindowHeight  *int
	UserAgent     *string
	CountryCode   *string
	CustomHeaders *[]Header
	Extra         *map[string]any
}

type Page struct {
	Limit  int
	Offset int
}

func (p Page) Normalize() Page {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}

const (
	maxWindowDim = 10000
	maxUAChars   = 1024
	maxHeaders   = 50
	maxNameChars = 100
)

var countryRE = regexp.MustCompile(`^[A-Z]{2}$`)

func (p Profile) Validate() error {
	return validateFields(p.Name, p.DeviceType, p.WindowWidth, p.WindowHeight, p.UserAgent, p.CountryCode, p.CustomHeaders)
}

func validateFields(name string, dt DeviceType, w, h int, ua, cc string, headers []Header) error {
	var errs []error
	if strings.TrimSpace(name) == "" || len(name) > maxNameChars {
		errs = append(errs, fmt.Errorf("name: must be non-empty and <= %d chars", maxNameChars))
	}
	if !dt.Valid() {
		errs = append(errs, errors.New("device_type: must be desktop or mobile"))
	}
	if w < 1 || w > maxWindowDim {
		errs = append(errs, fmt.Errorf("window_width: must be 1..%d", maxWindowDim))
	}
	if h < 1 || h > maxWindowDim {
		errs = append(errs, fmt.Errorf("window_height: must be 1..%d", maxWindowDim))
	}
	if ua == "" || len(ua) > maxUAChars {
		errs = append(errs, fmt.Errorf("user_agent: must be non-empty and <= %d chars", maxUAChars))
	}
	if !countryRE.MatchString(cc) {
		errs = append(errs, errors.New("country_code: must match ISO-3166 alpha-2 (^[A-Z]{2}$)"))
	}
	if len(headers) > maxHeaders {
		errs = append(errs, fmt.Errorf("custom_headers: max %d entries", maxHeaders))
	}
	seen := map[string]struct{}{}
	for i, h := range headers {
		if strings.TrimSpace(h.Key) == "" {
			errs = append(errs, fmt.Errorf("custom_headers[%d].key: must be non-empty", i))
		}
		if _, dup := seen[h.Key]; dup {
			errs = append(errs, fmt.Errorf("custom_headers[%d].key: duplicate %q", i, h.Key))
		}
		seen[h.Key] = struct{}{}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrInvalidInput, errors.Join(errs...))
}
