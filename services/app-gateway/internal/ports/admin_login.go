package ports

import (
	"context"
	"emisell-app-platform/services/app-gateway/internal/domain"
	"time"
)

type AdminLoginRepository interface {
	FindAdminAccount(context.Context, string) (domain.AdminAccount, error)
	ConsumeAdminLoginAttempt(context.Context, string, int, time.Time) (bool, error)
	CreateAdminSession(context.Context, domain.IdentitySession, string) error
	GetAdminSession(context.Context, string, time.Time, time.Duration) (domain.IdentitySession, error)
	RevokeAdminSession(context.Context, string, time.Time) error
}
