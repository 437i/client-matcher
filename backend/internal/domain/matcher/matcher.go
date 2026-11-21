package matcher

import (
	"errors"

	"github.com/437i/client-matcher/backend/internal/domain/client"
)

var (
	ErrLastNameMismatch = errors.New("фамилии не совпадают")
	ErrBirthdayMismatch = errors.New("дни рождения не совпадают")
	ErrEmptyLastName    = errors.New("фамилия не указана")
	ErrInvalidBirthday  = errors.New("некорректная дата рождения")
)

// MatchClients проверяет, что записи CRM и сайта описывают одного человека:
// совпадают фамилия (без учёта регистра, пробелов и ё/е) и дата рождения.
func MatchClients(crmClient client.CRMClient, siteClient client.SiteClient) error {
	crmLastName := client.NormalizeName(crmClient.LastName)
	siteLastName := client.NormalizeName(siteClient.LastName)

	// Проверяем наличие фамилий.
	if crmLastName == "" || siteLastName == "" {
		return ErrEmptyLastName
	}

	// Проверяем корректность дат.
	if crmClient.Birthday == nil || siteClient.Birthday == nil || !crmClient.Birthday.IsValid() || !siteClient.Birthday.IsValid() {
		return ErrInvalidBirthday
	}

	// Сравниваем фамилии.
	if crmLastName != siteLastName {
		return ErrLastNameMismatch
	}

	// Сравниваем даты рождения.
	if *crmClient.Birthday != *siteClient.Birthday {
		return ErrBirthdayMismatch
	}
	return nil
}
