package client

import "strings"

type CRMClient struct {
	UnstructuredName string
	ID               string
	LastName         string
	FirstName        string
	Patronymic       string // пустая строка, если отчества нет
	Birthday         *Date
	Clinic           string
}

// FullName возвращает ФИО в порядке «Фамилия Имя Отчество».
func (c CRMClient) FullName() string {
	if c.UnstructuredName != "" {
		return c.UnstructuredName
	}
	parts := []string{c.LastName, c.FirstName}
	if c.Patronymic != "" {
		parts = append(parts, c.Patronymic)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}
