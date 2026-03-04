package domain

import "time"

type FlagSeverity string

const (
	FlagSeverityInfo     FlagSeverity = "info"
	FlagSeverityWarning  FlagSeverity = "warning"
	FlagSeverityCritical FlagSeverity = "critical"
)

type CustomerFlag struct {
	ID             string       `json:"id"`
	UserID         string       `json:"userId"`
	FlagType       string       `json:"flagType"`
	Severity       FlagSeverity `json:"severity"`
	Description    string       `json:"description"`
	CreatedBy      *string      `json:"createdBy,omitempty"`
	IsResolved     bool         `json:"isResolved"`
	ResolvedBy     *string      `json:"resolvedBy,omitempty"`
	ResolvedAt     *time.Time   `json:"resolvedAt,omitempty"`
	ResolutionNote *string      `json:"resolutionNote,omitempty"`
	CreatedAt      time.Time    `json:"createdAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
}
