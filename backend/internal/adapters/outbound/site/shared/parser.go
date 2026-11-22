package shared

import "strings"

// ParseTitle applies the common site title parsing rules to CSV and PostgreSQL sources.
func ParseTitle(title string) (unstructured, shortName, lastName string) {
	title = strings.TrimSpace(title)
	parts := strings.Fields(title)
	if len(parts) < 2 {
		return title, "", ""
	}
	return "", parts[0], strings.Join(parts[1:], " ")
}
