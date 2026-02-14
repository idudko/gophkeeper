// Package commands provides display utilities for GophKeeper client commands.
package commands

import (
	"strings"

	"github.com/idudko/gophkeeper/internal/models"
)

// getTypeSymbol returns a symbol for the data type.
func getTypeSymbol(dataType models.DataType) string {
	switch dataType {
	case models.DataTypePassword:
		return "🔑"
	case models.DataTypeText:
		return "📝"
	case models.DataTypeBinary:
		return "📦"
	case models.DataTypeCard:
		return "💳"
	default:
		return "📄"
	}
}

// maskCardNumber masks a card number showing only last 4 digits.
func maskCardNumber(number string) string {
	if len(number) <= 4 {
		return number
	}
	return strings.Repeat("*", len(number)-4) + number[len(number)-4:]
}

// sortItems sorts items by the specified field.
func sortItems(items []models.Item, sortBy string) {
	switch sortBy {
	case "created_at":
		for i := 0; i < len(items)-1; i++ {
			for j := i + 1; j < len(items); j++ {
				if items[i].CreatedAt.After(items[j].CreatedAt) {
					items[i], items[j] = items[j], items[i]
				}
			}
		}
	case "updated_at":
		for i := 0; i < len(items)-1; i++ {
			for j := i + 1; j < len(items); j++ {
				if items[i].UpdatedAt.After(items[j].UpdatedAt) {
					items[i], items[j] = items[j], items[i]
				}
			}
		}
	case "type":
		for i := 0; i < len(items)-1; i++ {
			for j := i + 1; j < len(items); j++ {
				if items[i].Type > items[j].Type {
					items[i], items[j] = items[j], items[i]
				}
			}
		}
	case "meta":
		for i := 0; i < len(items)-1; i++ {
			for j := i + 1; j < len(items); j++ {
				if items[i].Meta > items[j].Meta {
					items[i], items[j] = items[j], items[i]
				}
			}
		}
	}
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
