package client

import "errors"

// Ошибки, которые обязаны возвращать любые реализации ports.CRMReader и
// ports.SiteReader (CSV, HTTP API, PostgreSQL), чтобы слои выше не зависели
// от конкретного источника данных.
var (
	ErrCRMNotFound  = errors.New("crm client not found")
	ErrSiteNotFound = errors.New("site client not found")
)
