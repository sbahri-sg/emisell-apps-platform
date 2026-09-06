package fault

import "errors"

var (
	Invalid         = errors.New("invalid_request")
	Unauthenticated = errors.New("unauthenticated")
	Forbidden       = errors.New("forbidden")
	NotFound        = errors.New("not_found")
	Conflict        = errors.New("conflict")
	Unavailable     = errors.New("unavailable")
)
