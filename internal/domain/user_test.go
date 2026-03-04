package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_FullName(t *testing.T) {
	abebe := "Abebe"
	fikadu := "Fikadu"
	bikila := "Bikila"

	tests := []struct {
		name string
		user *User
		want string
	}{
		{
			name: "all three names set",
			user: &User{FirstName: &abebe, MiddleName: &fikadu, LastName: &bikila},
			want: "Abebe Fikadu Bikila",
		},
		{
			name: "first and last only",
			user: &User{FirstName: &abebe, LastName: &bikila},
			want: "Abebe Bikila",
		},
		{
			name: "first only",
			user: &User{FirstName: &abebe},
			want: "Abebe",
		},
		{
			name: "all nil",
			user: &User{},
			want: "",
		},
		{
			name: "only last",
			user: &User{LastName: &bikila},
			want: " Bikila",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.user.FullName()
			assert.Equal(t, tt.want, got)
		})
	}
}
