package client

import "strings"

type CRMClient struct {
	ID         string
	LastName   string
	FirstName  string
	Patronymic string // пустая строка, если отчества нет
	Birthday   Date
	Clinic     string
}

// FullName возвращает ФИО в порядке «Фамилия Имя Отчество».
func (c CRMClient) FullName() string {
	parts := []string{c.LastName, c.FirstName}
	if c.Patronymic != "" {
		parts = append(parts, c.Patronymic)
	}
	return strings.Join(parts, " ")
}
