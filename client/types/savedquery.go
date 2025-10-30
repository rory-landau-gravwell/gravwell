package types

import (
	"encoding/json"
)

// SavedQuery is a stored Gravwell query. This replaces SearchLibrary in the old types.
type SavedQuery struct {
	CommonFields

	Query string `db:"query"`
}

type SavedQueryListResponse struct {
	BaseListResponse
	Results []SavedQuery `json:"results"`
}

func (sq *SavedQuery) JSONMetadata() (json.RawMessage, error) {
	b, err := json.Marshal(&struct {
		Name        string
		Description string
		Query       string
	}{
		Name:        sq.Name,
		Description: sq.Description,
		Query:       sq.Query,
	})
	return json.RawMessage(b), err
}
