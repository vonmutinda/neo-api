package admin

import (
	"net/url"
	"strconv"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
)

func parseUserFilter(q url.Values) domain.UserFilter {
	f := domain.UserFilter{
		SortBy:    q.Get("sort"),
		SortOrder: q.Get("order"),
	}
	f.Limit, f.Offset = parseLimitOffset(q)

	if s := q.Get("search"); s != "" {
		f.Search = &s
	}
	if v := q.Get("kyc_level"); v != "" {
		if level, err := strconv.Atoi(v); err == nil {
			l := domain.KYCLevel(level)
			f.KYCLevel = &l
		}
	}
	if v := q.Get("is_frozen"); v != "" {
		b := v == "true"
		f.IsFrozen = &b
	}
	if v := q.Get("account_type"); v != "" {
		f.AccountType = &v
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedFrom = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedTo = &t
		}
	}
	return f
}

func parseTransactionFilter(q url.Values) domain.TransactionFilter {
	f := domain.TransactionFilter{
		SortBy:    q.Get("sort"),
		SortOrder: q.Get("order"),
	}
	f.Limit, f.Offset = parseLimitOffset(q)

	if s := q.Get("search"); s != "" {
		f.Search = &s
	}
	if v := q.Get("user_id"); v != "" {
		f.UserID = &v
	}
	if v := q.Get("type"); v != "" {
		f.Type = &v
	}
	if v := q.Get("status"); v != "" {
		f.Status = &v
	}
	if v := q.Get("currency"); v != "" {
		f.Currency = &v
	}
	if v := q.Get("min_amount"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.MinAmountCents = &n
		}
	}
	if v := q.Get("max_amount"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.MaxAmountCents = &n
		}
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedFrom = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedTo = &t
		}
	}
	return f
}

func parseLoanFilter(q url.Values) domain.LoanFilter {
	f := domain.LoanFilter{
		SortBy:    q.Get("sort"),
		SortOrder: q.Get("order"),
	}
	f.Limit, f.Offset = parseLimitOffset(q)

	if v := q.Get("user_id"); v != "" {
		f.UserID = &v
	}
	if v := q.Get("status"); v != "" {
		f.Status = &v
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedFrom = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedTo = &t
		}
	}
	return f
}

func parseCardFilter(q url.Values) domain.CardFilter {
	f := domain.CardFilter{
		SortBy:    q.Get("sort"),
		SortOrder: q.Get("order"),
	}
	f.Limit, f.Offset = parseLimitOffset(q)

	if v := q.Get("user_id"); v != "" {
		f.UserID = &v
	}
	if v := q.Get("type"); v != "" {
		f.Type = &v
	}
	if v := q.Get("status"); v != "" {
		f.Status = &v
	}
	return f
}

func parseCardAuthFilter(q url.Values) domain.CardAuthFilter {
	f := domain.CardAuthFilter{
		SortBy:    q.Get("sort"),
		SortOrder: q.Get("order"),
	}
	f.Limit, f.Offset = parseLimitOffset(q)

	if v := q.Get("card_id"); v != "" {
		f.CardID = &v
	}
	if v := q.Get("user_id"); v != "" {
		f.UserID = &v
	}
	if v := q.Get("status"); v != "" {
		f.Status = &v
	}
	if v := q.Get("mcc"); v != "" {
		f.MCC = &v
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedFrom = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedTo = &t
		}
	}
	return f
}

func parseAuditFilter(q url.Values) domain.AuditFilter {
	f := domain.AuditFilter{
		SortOrder: q.Get("order"),
	}
	f.Limit, f.Offset = parseLimitOffset(q)

	if v := q.Get("search"); v != "" {
		f.Search = &v
	}
	if v := q.Get("action"); v != "" {
		f.Action = &v
	}
	if v := q.Get("actor_type"); v != "" {
		f.ActorType = &v
	}
	if v := q.Get("actor_id"); v != "" {
		f.ActorID = &v
	}
	if v := q.Get("resource_type"); v != "" {
		f.ResourceType = &v
	}
	if v := q.Get("resource_id"); v != "" {
		f.ResourceID = &v
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedFrom = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedTo = &t
		}
	}
	return f
}

func parseExceptionFilter(q url.Values) domain.ExceptionFilter {
	f := domain.ExceptionFilter{
		SortBy:    q.Get("sort"),
		SortOrder: q.Get("order"),
	}
	f.Limit, f.Offset = parseLimitOffset(q)

	if v := q.Get("status"); v != "" {
		f.Status = &v
	}
	if v := q.Get("error_type"); v != "" {
		f.ErrorType = &v
	}
	if v := q.Get("assigned_to"); v != "" {
		f.AssignedTo = &v
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedFrom = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.DateOnly, v); err == nil {
			f.CreatedTo = &t
		}
	}
	return f
}

func parseCreditProfileFilter(q url.Values) domain.CreditProfileFilter {
	f := domain.CreditProfileFilter{
		SortBy:    q.Get("sort"),
		SortOrder: q.Get("order"),
	}
	f.Limit, f.Offset = parseLimitOffset(q)

	if v := q.Get("min_score"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.MinTrustScore = &n
		}
	}
	if v := q.Get("max_score"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.MaxTrustScore = &n
		}
	}
	if v := q.Get("blacklisted"); v != "" {
		b := v == "true"
		f.IsBlacklisted = &b
	}
	return f
}

func parseFlagFilter(q url.Values) domain.FlagFilter {
	f := domain.FlagFilter{
		SortOrder: q.Get("order"),
	}
	f.Limit, f.Offset = parseLimitOffset(q)

	if v := q.Get("user_id"); v != "" {
		f.UserID = &v
	}
	if v := q.Get("flag_type"); v != "" {
		f.FlagType = &v
	}
	if v := q.Get("severity"); v != "" {
		f.Severity = &v
	}
	if v := q.Get("resolved"); v != "" {
		b := v == "true"
		f.Resolved = &b
	}
	return f
}

func parseStaffFilter(q url.Values) domain.StaffFilter {
	f := domain.StaffFilter{
		SortBy:    q.Get("sort"),
		SortOrder: q.Get("order"),
	}
	f.Limit, f.Offset = parseLimitOffset(q)

	if v := q.Get("search"); v != "" {
		f.Search = &v
	}
	if v := q.Get("role"); v != "" {
		f.Role = &v
	}
	if v := q.Get("department"); v != "" {
		f.Department = &v
	}
	if v := q.Get("is_active"); v != "" {
		b := v == "true"
		f.IsActive = &b
	}
	return f
}

func parseLimitOffset(q url.Values) (int, int) {
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	return domain.NormalizePagination(limit, offset)
}
