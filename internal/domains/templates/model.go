// Package templates is the read-only predefined-template domain.
package templates

type Header struct {
	Key   string
	Value string
}

type Template struct {
	Slug          string
	Name          string
	DeviceType    string // desktop | mobile
	WindowWidth   int
	WindowHeight  int
	UserAgent     string
	CountryCode   string
	CustomHeaders []Header
}
