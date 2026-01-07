package types

import (
	"time"
)

// SearchHistoryEntry represents a stored search history entry in the registry.
// This replaces the search history implementation from the webstore package.
type SearchHistoryEntry struct {
	CommonFields

	UserQuery      string
	EffectiveQuery string
	Launched       time.Time
}

type SearchHistoryListResponse struct {
	BaseListResponse
	Results []SearchHistoryEntry `json:"results"`
}
