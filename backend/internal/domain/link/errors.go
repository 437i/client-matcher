package link

import "errors"

var (
	ErrNotFound          = errors.New("link not found")
	ErrCRMAlreadyLinked  = errors.New("crm client is already linked")
	ErrSiteAlreadyLinked = errors.New("site client is already linked")
)
