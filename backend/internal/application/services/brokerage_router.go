// application/services/brokerage_router.go
package services

import (
	"strings"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// NormalizeAssetCategory maps Claude's free-form type strings to a canonical AssetCategory.
// Unrecognised strings map to AssetCategoryEquity (broadest net before the default fallback).
func NormalizeAssetCategory(allocType string) models.AssetCategory {
	switch strings.ToLower(strings.TrimSpace(allocType)) {
	case "bond", "bond etf", "fixed income", "treasury", "tips",
		"municipal bond", "high yield bond", "bonds":
		return models.AssetCategoryBond
	default:
		// stock, etf, equity, growth etf, dividend etf, index etf, reit, crypto etf, etc.
		return models.AssetCategoryEquity
	}
}

// RouteAllocation selects the best connection for a given allocation:
//  1. First connected connection whose AssetCategories includes the allocation's category.
//  2. First connected connection whose AssetCategories includes AssetCategoryDefault.
//  3. nil — caller logs and skips this allocation.
func RouteAllocation(alloc models.Allocation, connections []models.BrokerageConnection) *models.BrokerageConnection {
	want := NormalizeAssetCategory(alloc.Type)

	var defaultConn *models.BrokerageConnection
	for i := range connections {
		c := &connections[i]
		if !c.Connected {
			continue
		}
		for _, cat := range c.AssetCategories {
			if cat == want {
				return c
			}
			if cat == models.AssetCategoryDefault && defaultConn == nil {
				defaultConn = c
			}
		}
	}
	return defaultConn
}
