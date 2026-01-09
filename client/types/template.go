package types

import "encoding/json"

// Template is a stored Gravwell query template with variables.
type Template struct {
	CommonFields

	Query     string
	Variables []TemplateVariable
}

type TemplateVariable struct {
	Name         string
	Label        string
	Description  string
	Required     bool
	DefaultValue string
	PreviewValue string
}

type TemplateListResponse struct {
	BaseListResponse
	Results []Template `json:"results"`
}

func (t *Template) JSONMetadata() (json.RawMessage, error) {
	b, err := json.Marshal(&struct {
		Name        string
		Description string
	}{
		Name:        t.Name,
		Description: t.Description,
	})
	return json.RawMessage(b), err
}
