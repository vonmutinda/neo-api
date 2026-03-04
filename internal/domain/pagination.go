package domain

import "time"

type PaginatedResult[T any] struct {
	Data       []T            `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

type PaginationMeta struct {
	Total   int64 `json:"total"`
	Limit   int   `json:"limit"`
	Offset  int   `json:"offset"`
	HasMore bool  `json:"hasMore"`
}

func NewPaginatedResult[T any](data []T, total int64, limit, offset int) *PaginatedResult[T] {
	return &PaginatedResult[T]{
		Data: data,
		Pagination: PaginationMeta{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: int64(offset+limit) < total,
		},
	}
}

// --- Filter Structs ---

type UserFilter struct {
	Search      *string    `json:"search,omitempty"`
	KYCLevel    *KYCLevel  `json:"kycLevel,omitempty"`
	IsFrozen    *bool      `json:"isFrozen,omitempty"`
	AccountType *string    `json:"accountType,omitempty"`
	CreatedFrom *time.Time `json:"createdFrom,omitempty"`
	CreatedTo   *time.Time `json:"createdTo,omitempty"`
	SortBy      string     `json:"sortBy,omitempty"`
	SortOrder   string     `json:"sortOrder,omitempty"`
	Limit       int        `json:"limit"`
	Offset      int        `json:"offset"`
}

type TransactionFilter struct {
	Search         *string    `json:"search,omitempty"`
	UserID         *string    `json:"userId,omitempty"`
	Type           *string    `json:"type,omitempty"`
	Status         *string    `json:"status,omitempty"`
	Currency       *string    `json:"currency,omitempty"`
	MinAmountCents *int64     `json:"minAmountCents,omitempty"`
	MaxAmountCents *int64     `json:"maxAmountCents,omitempty"`
	CreatedFrom    *time.Time `json:"createdFrom,omitempty"`
	CreatedTo      *time.Time `json:"createdTo,omitempty"`
	SortBy         string     `json:"sortBy,omitempty"`
	SortOrder      string     `json:"sortOrder,omitempty"`
	Limit          int        `json:"limit"`
	Offset         int        `json:"offset"`
}

type LoanFilter struct {
	UserID      *string    `json:"userId,omitempty"`
	Status      *string    `json:"status,omitempty"`
	MinAmount   *int64     `json:"minAmount,omitempty"`
	MaxAmount   *int64     `json:"maxAmount,omitempty"`
	CreatedFrom *time.Time `json:"createdFrom,omitempty"`
	CreatedTo   *time.Time `json:"createdTo,omitempty"`
	SortBy      string     `json:"sortBy,omitempty"`
	SortOrder   string     `json:"sortOrder,omitempty"`
	Limit       int        `json:"limit"`
	Offset      int        `json:"offset"`
}

type CardFilter struct {
	UserID   *string `json:"userId,omitempty"`
	Type     *string `json:"type,omitempty"`
	Status   *string `json:"status,omitempty"`
	SortBy   string  `json:"sortBy,omitempty"`
	SortOrder string `json:"sortOrder,omitempty"`
	Limit    int     `json:"limit"`
	Offset   int     `json:"offset"`
}

type CardAuthFilter struct {
	CardID      *string    `json:"cardId,omitempty"`
	UserID      *string    `json:"userId,omitempty"`
	Status      *string    `json:"status,omitempty"`
	MCC         *string    `json:"mcc,omitempty"`
	CreatedFrom *time.Time `json:"createdFrom,omitempty"`
	CreatedTo   *time.Time `json:"createdTo,omitempty"`
	SortBy      string     `json:"sortBy,omitempty"`
	SortOrder   string     `json:"sortOrder,omitempty"`
	Limit       int        `json:"limit"`
	Offset      int        `json:"offset"`
}

type AuditFilter struct {
	Search       *string    `json:"search,omitempty"`
	Action       *string    `json:"action,omitempty"`
	ActorType    *string    `json:"actorType,omitempty"`
	ActorID      *string    `json:"actorId,omitempty"`
	ResourceType *string    `json:"resourceType,omitempty"`
	ResourceID   *string    `json:"resourceId,omitempty"`
	CreatedFrom  *time.Time `json:"createdFrom,omitempty"`
	CreatedTo    *time.Time `json:"createdTo,omitempty"`
	SortOrder    string     `json:"sortOrder,omitempty"`
	Limit        int        `json:"limit"`
	Offset       int        `json:"offset"`
}

type ExceptionFilter struct {
	Status      *string    `json:"status,omitempty"`
	ErrorType   *string    `json:"errorType,omitempty"`
	AssignedTo  *string    `json:"assignedTo,omitempty"`
	CreatedFrom *time.Time `json:"createdFrom,omitempty"`
	CreatedTo   *time.Time `json:"createdTo,omitempty"`
	SortBy      string     `json:"sortBy,omitempty"`
	SortOrder   string     `json:"sortOrder,omitempty"`
	Limit       int        `json:"limit"`
	Offset      int        `json:"offset"`
}

type CreditProfileFilter struct {
	MinTrustScore *int    `json:"minTrustScore,omitempty"`
	MaxTrustScore *int    `json:"maxTrustScore,omitempty"`
	IsBlacklisted *bool   `json:"isBlacklisted,omitempty"`
	SortBy        string  `json:"sortBy,omitempty"`
	SortOrder     string  `json:"sortOrder,omitempty"`
	Limit         int     `json:"limit"`
	Offset        int     `json:"offset"`
}

type BusinessFilter struct {
	Search      *string    `json:"search,omitempty"`
	Status      *string    `json:"status,omitempty"`
	Industry    *string    `json:"industry,omitempty"`
	IsFrozen    *bool      `json:"isFrozen,omitempty"`
	CreatedFrom *time.Time `json:"createdFrom,omitempty"`
	CreatedTo   *time.Time `json:"createdTo,omitempty"`
	SortBy      string     `json:"sortBy,omitempty"`
	SortOrder   string     `json:"sortOrder,omitempty"`
	Limit       int        `json:"limit"`
	Offset      int        `json:"offset"`
}

type FlagFilter struct {
	UserID   *string `json:"userId,omitempty"`
	FlagType *string `json:"flagType,omitempty"`
	Severity *string `json:"severity,omitempty"`
	Resolved *bool   `json:"resolved,omitempty"`
	SortOrder string `json:"sortOrder,omitempty"`
	Limit    int     `json:"limit"`
	Offset   int     `json:"offset"`
}

type StaffFilter struct {
	Search     *string `json:"search,omitempty"`
	Role       *string `json:"role,omitempty"`
	Department *string `json:"department,omitempty"`
	IsActive   *bool   `json:"isActive,omitempty"`
	SortBy     string  `json:"sortBy,omitempty"`
	SortOrder  string  `json:"sortOrder,omitempty"`
	Limit      int     `json:"limit"`
	Offset     int     `json:"offset"`
}

// NormalizePagination clamps limit/offset to safe defaults.
func NormalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
