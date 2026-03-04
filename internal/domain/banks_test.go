package domain_test

import (
	"testing"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupBank_KnownCode(t *testing.T) {
	b := domain.LookupBank("CBE")
	require.NotNil(t, b)
	assert.Equal(t, "CBE", b.InstitutionCode)
	assert.Equal(t, "Commercial Bank of Ethiopia", b.Name)
	assert.Equal(t, "CBETETAA", b.SwiftBIC)
	assert.Equal(t, "ET", b.CountryCode)
}

func TestLookupBank_UnknownCode(t *testing.T) {
	b := domain.LookupBank("DOESNOTEXIST")
	assert.Nil(t, b)
}

func TestLookupBank_AllEntries(t *testing.T) {
	for code, expected := range domain.EthiopianBanks {
		b := domain.LookupBank(code)
		require.NotNil(t, b, "LookupBank(%q) returned nil", code)
		assert.Equal(t, expected.InstitutionCode, b.InstitutionCode)
		assert.Equal(t, expected.Name, b.Name)
	}
}

func TestListBanks_ReturnsSortedByName(t *testing.T) {
	banks := domain.ListBanks()
	require.Len(t, banks, len(domain.EthiopianBanks))

	for i := 1; i < len(banks); i++ {
		assert.LessOrEqual(t, banks[i-1].Name, banks[i].Name,
			"banks not sorted: %q should come before %q", banks[i-1].Name, banks[i].Name)
	}
}

func TestListBanks_ContainsNeobank(t *testing.T) {
	banks := domain.ListBanks()
	found := false
	for _, b := range banks {
		if b.InstitutionCode == "NEOBANK" {
			found = true
			assert.Equal(t, "Neo", b.Name)
			break
		}
	}
	assert.True(t, found, "NEOBANK not found in ListBanks()")
}
