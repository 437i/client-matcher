package matcher

import (
	"github.com/437i/client-matcher/backend/internal/domain/client"
	"slices"
)

const RuleVersion = 1

type Status string

const (
	StatusReady                Status = "ready"
	StatusRequiresConfirmation Status = "requires_confirmation"
	StatusBlocked              Status = "blocked"
)

type Field string

const (
	FieldLastName  Field = "last_name"
	FieldFirstName Field = "first_name"
	FieldBirthday  Field = "birthday"
)

type Code string

const (
	CodeExact           Code = "exact"
	CodeNormalizedMatch Code = "normalized_match"
	CodeAliasMatch      Code = "alias_match"
	CodeProbableMatch   Code = "probable_match"
	CodeMismatch        Code = "mismatch"
	CodeMissing         Code = "missing"
)

type FieldCheck struct {
	Field               Field
	Code                Code
	CRMValue, SiteValue *string
}
type Assessment struct {
	Status               Status
	Checks               []FieldCheck
	ConfirmationRequired bool
}

func Assess(crm client.CRMClient, site client.SiteClient) Assessment {
	if crm.UnstructuredName != "" {
		crm.LastName, crm.FirstName = "", ""
	}
	if site.UnstructuredName != "" {
		site.LastName, site.ShortName = "", ""
	}
	checks := []FieldCheck{
		{FieldLastName, compareName(crm.LastName, site.LastName, false), value(crm.LastName), value(site.LastName)},
		{FieldFirstName, compareName(crm.FirstName, site.ShortName, true), value(crm.FirstName), value(site.ShortName)},
		{FieldBirthday, compareBirthday(crm.Birthday, site.Birthday), dateValue(crm.Birthday), dateValue(site.Birthday)},
	}
	status := StatusReady
	for _, check := range checks {
		if check.Code != CodeExact && check.Code != CodeNormalizedMatch {
			status = StatusRequiresConfirmation
			break
		}
	}
	return Assessment{Status: status, Checks: checks, ConfirmationRequired: status == StatusRequiresConfirmation}
}

func compareName(a, b string, given bool) Code {
	rawA, rawB := a, b
	a, b = client.NormalizeName(a), client.NormalizeName(b)
	switch {
	case a == "" || b == "":
		return CodeMissing
	case rawA == rawB:
		return CodeExact
	case a == b:
		return CodeNormalizedMatch
	case given && client.GivenNameAliases(a, b):
		return CodeAliasMatch
	case client.ProbableNameMatch(a, b):
		return CodeProbableMatch
	default:
		return CodeMismatch
	}
}

func value(s string) *string {
	if client.NormalizeName(s) == "" {
		return nil
	}
	return &s
}

func dateValue(d *client.Date) *string {
	if d == nil {
		return nil
	}
	s := d.String()
	return &s
}

func compareBirthday(a, b *client.Date) Code {
	if a == nil || b == nil {
		return CodeMissing
	}
	if !a.IsValid() || !b.IsValid() {
		return CodeMismatch
	}
	if *a == *b {
		return CodeExact
	}
	return CodeMismatch
}

func (a Assessment) ConfirmationCodes() []string {
	codes := []string{}
	for _, check := range a.Checks {
		if check.Code == CodeExact || check.Code == CodeNormalizedMatch {
			continue
		}
		field := "name"
		if check.Field == FieldBirthday {
			field = "birthday"
		}
		codes = append(codes, field+":"+string(check.Code))
	}
	slices.Sort(codes)
	return slices.Compact(codes)
}
