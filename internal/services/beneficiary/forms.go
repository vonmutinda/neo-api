package beneficiary

import (
	"fmt"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/pkg/validate"
)

type CreateBeneficiaryRequest struct {
	FullName     string `json:"fullName" validate:"required,min=2,max=200"`
	Relationship string `json:"relationship" validate:"required,oneof=spouse child parent"`
	DocumentURL  string `json:"documentUrl"`
}

func (r *CreateBeneficiaryRequest) Validate() error {
	if err := validate.Struct(r); err != nil {
		return err
	}
	switch domain.BeneficiaryRelType(r.Relationship) {
	case domain.BeneficiarySpouse, domain.BeneficiaryChild, domain.BeneficiaryParent:
		return nil
	default:
		return fmt.Errorf("invalid relationship %q: %w", r.Relationship, domain.ErrInvalidInput)
	}
}
