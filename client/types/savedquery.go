package types

// SavedQuery is a stored Gravwell query. This replaces SearchLibrary in the old types.
type SavedQuery struct {
	CommonFields

	Query string `db:"query"`
}

type SavedQueryListResponse struct {
	BaseListResponse
	Results []SavedQuery `json:"results"`
}
