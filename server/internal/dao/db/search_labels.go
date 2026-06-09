package db

import "strings"

func localizedEnumMatches(keyword string, labels map[string]string) []string {
	value := strings.ToLower(strings.TrimSpace(keyword))
	if value == "" {
		return nil
	}
	matches := make([]string, 0, len(labels))
	for enumValue, label := range labels {
		if strings.Contains(strings.ToLower(enumValue), value) || strings.Contains(strings.ToLower(label), value) {
			matches = append(matches, enumValue)
		}
	}
	return matches
}
