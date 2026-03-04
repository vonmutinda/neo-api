package admin

import (
	"context"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/repository"
	"github.com/vonmutinda/neo/pkg/geo"
)

type AnalyticsService struct {
	adminRepo repository.AdminQueryRepository
	flagRepo  repository.FlagRepository
	geo       geo.Lookuper
}

func NewAnalyticsService(adminRepo repository.AdminQueryRepository, flagRepo repository.FlagRepository, lookuper geo.Lookuper) *AnalyticsService {
	if lookuper == nil {
		lookuper = geo.NoopLookuper{}
	}
	return &AnalyticsService{adminRepo: adminRepo, flagRepo: flagRepo, geo: lookuper}
}

type PlatformOverview struct {
	TotalCustomers        int64                    `json:"totalCustomers"`
	ActiveCustomers30d    int64                    `json:"activeCustomers30d"`
	FrozenAccounts        int64                    `json:"frozenAccounts"`
	KYCBreakdown          map[domain.KYCLevel]int64 `json:"kycBreakdown"`
	ActiveLoans           int64                    `json:"activeLoans"`
	TotalLoanOutstanding  int64                    `json:"totalLoanOutstandingCents"`
	ActiveCards           int64                    `json:"activeCards"`
	ActivePots            int64                    `json:"activePots"`
	ActiveBusinesses      int64                    `json:"activeBusinesses"`
	TotalTransactions     int64                    `json:"totalTransactions"`
	OpenFlags             int64                    `json:"openFlags"`
	AsOf                  time.Time                `json:"asOf"`
}

func (s *AnalyticsService) Overview(ctx context.Context) (*PlatformOverview, error) {
	overview := &PlatformOverview{AsOf: time.Now()}

	var err error
	if overview.TotalCustomers, err = s.adminRepo.CountUsers(ctx); err != nil {
		return nil, err
	}
	if overview.ActiveCustomers30d, err = s.adminRepo.CountActiveUsers30d(ctx); err != nil {
		return nil, err
	}
	if overview.FrozenAccounts, err = s.adminRepo.CountFrozenAccounts(ctx); err != nil {
		return nil, err
	}
	if overview.KYCBreakdown, err = s.adminRepo.CountUsersByKYCLevel(ctx); err != nil {
		return nil, err
	}
	if overview.ActiveLoans, err = s.adminRepo.CountActiveLoans(ctx); err != nil {
		return nil, err
	}
	if overview.TotalLoanOutstanding, err = s.adminRepo.SumLoanOutstanding(ctx); err != nil {
		return nil, err
	}
	if overview.ActiveCards, err = s.adminRepo.CountActiveCards(ctx); err != nil {
		return nil, err
	}
	if overview.ActivePots, err = s.adminRepo.CountActivePots(ctx); err != nil {
		return nil, err
	}
	if overview.ActiveBusinesses, err = s.adminRepo.CountActiveBusinesses(ctx); err != nil {
		return nil, err
	}
	if overview.TotalTransactions, err = s.adminRepo.CountTransactions(ctx); err != nil {
		return nil, err
	}
	if overview.OpenFlags, err = s.flagRepo.CountOpen(ctx); err != nil {
		return nil, err
	}

	return overview, nil
}

type RegistrationSeries struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

func (s *AnalyticsService) Registrations(ctx context.Context, from, to time.Time) ([]RegistrationSeries, error) {
	filter := domain.UserFilter{
		CreatedFrom: &from,
		CreatedTo:   &to,
		Limit:       1,
	}
	result, err := s.adminRepo.ListUsers(ctx, filter)
	if err != nil {
		return nil, err
	}
	_ = result
	return nil, nil
}

// MoneyFlowPoint is a single transaction origin point for the map (no raw IP returned).
type MoneyFlowPoint struct {
	Lat            float64 `json:"lat"`
	Lon            float64 `json:"lon"`
	AmountCents    int64   `json:"amountCents"`
	Currency       string  `json:"currency"`
	Type           string  `json:"type"`
	CreatedAt      string  `json:"createdAt"`
	TransactionID  string  `json:"transactionId"`
	City           string  `json:"city,omitempty"`
	Country        string  `json:"country,omitempty"`
}

// MoneyFlowFlow is a flow between two locations (e.g. P2P sender → recipient).
type MoneyFlowFlow struct {
	From           MoneyFlowCoord `json:"from"`
	To             MoneyFlowCoord `json:"to"`
	AmountCents    int64          `json:"amountCents"`
	Currency       string         `json:"currency"`
	TransactionID  string         `json:"transactionId,omitempty"`
}

// MoneyFlowCoord is a lat/lon point (no raw IP).
type MoneyFlowCoord struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// MoneyFlowMapResponse is the response for GET /admin/v1/analytics/money-flow-map.
type MoneyFlowMapResponse struct {
	Points []MoneyFlowPoint `json:"points"`
	Flows  []MoneyFlowFlow  `json:"flows"`
}

func (s *AnalyticsService) MoneyFlowMap(ctx context.Context, from, to time.Time, limit int) (*MoneyFlowMapResponse, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	filter := domain.TransactionFilter{
		CreatedFrom: &from,
		CreatedTo:   &to,
		Limit:       limit,
		Offset:      0,
	}
	result, err := s.adminRepo.ListTransactions(ctx, filter)
	if err != nil {
		return nil, err
	}
	txns := result.Data
	userIDs := make([]string, 0, len(txns))
	seen := make(map[string]struct{})
	for _, t := range txns {
		if _, ok := seen[t.UserID]; !ok {
			seen[t.UserID] = struct{}{}
			userIDs = append(userIDs, t.UserID)
		}
	}
	ipByUser, err := s.adminRepo.GetLastSessionIPByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	var points []MoneyFlowPoint
	for _, t := range txns {
		ip := ipByUser[t.UserID]
		if ip == "" {
			continue
		}
		lat, lon, city, country, ok := s.geo.Lookup(ip)
		if !ok {
			continue
		}
		points = append(points, MoneyFlowPoint{
			Lat:           lat,
			Lon:           lon,
			AmountCents:   t.AmountCents,
			Currency:      t.Currency,
			Type:          string(t.Type),
			CreatedAt:     t.CreatedAt.Format(time.RFC3339),
			TransactionID: t.ID,
			City:          city,
			Country:       country,
		})
	}
	return &MoneyFlowMapResponse{Points: points, Flows: []MoneyFlowFlow{}}, nil
}
