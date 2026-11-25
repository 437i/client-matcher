package client

import "strings"

type CRMClient struct {
	SourceFullName   string
	UnstructuredName string
	ID               string
	LastName         string
	FirstName        string
	Patronymic       string // пустая строка, если отчества нет
	Birthday         *Date
	Clinic           string
}

// FullName возвращает исходное имя CRM или собирает его из доступных частей.
func (c CRMClient) FullName() string {
	if strings.TrimSpace(c.SourceFullName) != "" {
		return c.SourceFullName
	}
	if c.UnstructuredName != "" {
		return c.UnstructuredName
	}
	parts := []string{c.LastName, c.FirstName}
	if c.Patronymic != "" {
		parts = append(parts, c.Patronymic)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}
