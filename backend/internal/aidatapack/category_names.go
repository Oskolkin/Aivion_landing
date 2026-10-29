package aidatapack

import (
	"regexp"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

var genericCategoryNamePattern = regexp.MustCompile(`(?i)^category\s+\d+$`)

func resolveCategoryDisplayName(idStr string, cat dbgen.DescriptionCategory) string {
	name := "Category " + idStr
	if cat.CategoryName != "" {
		name = cat.CategoryName
	}
	if isGenericCategoryDisplayName(name) {
		if alias := categoryDisplayNameAlias(idStr); alias != "" {
			return alias
		}
	}
	return name
}

func isGenericCategoryDisplayName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}
	return genericCategoryNamePattern.MatchString(name)
}

func categoryDisplayNameAlias(categoryID string) string {
	switch strings.TrimSpace(categoryID) {
	case "17990001":
		return "Товары для ванной"
	default:
		return ""
	}
}
