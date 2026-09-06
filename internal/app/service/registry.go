package service

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appmanifest"
)

type Release struct {
	Raw       []byte
	Signature string
}
type Repository interface {
	List(context.Context) ([]Release, error)
}
type Registry struct{ Repo Repository }

func (s Registry) List(ctx context.Context) ([]appmanifest.Manifest, error) {
	releases, err := s.Repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]appmanifest.Manifest, 0, len(releases))
	for _, r := range releases {
		m, err := appmanifest.VerifyLocal(r.Raw, r.Signature)
		if err != nil {
			return nil, fault.Unavailable
		}
		result = append(result, m)
	}
	return result, nil
}
func (s Registry) Get(ctx context.Context, id string) (appmanifest.Manifest, error) {
	apps, err := s.List(ctx)
	if err != nil {
		return appmanifest.Manifest{}, err
	}
	for _, a := range apps {
		if a.ID == id {
			return a, nil
		}
	}
	return appmanifest.Manifest{}, fault.NotFound
}
