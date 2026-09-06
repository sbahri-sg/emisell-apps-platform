package identity

import "context"

// MerchantDirectory is for trusted Admin verification only, never developer search.
type MerchantDirectory struct {
	Repo interface {
		KnownMerchant(context.Context, string) (bool, error)
	}
}

func (s MerchantDirectory) KnownMerchant(ctx context.Context, id string) (bool, error) {
	return s.Repo.KnownMerchant(ctx, id)
}
