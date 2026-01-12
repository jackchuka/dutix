package shared

import (
	"fmt"

	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
)

// DisplayResults shows apply results in table format with summary
func DisplayResults(results []domain.ApplyResult) error {
	fmt.Println("\nResults:")

	// Build results table
	resultPlan := &domain.Plan{Items: make([]domain.PlanItem, len(results))}
	for i, result := range results {
		resultPlan.Items[i] = result.Item
	}

	f := formatter.New("table", nil)
	if err := f.FormatPlan(resultPlan); err != nil {
		return fmt.Errorf("failed to format results: %w", err)
	}

	// Show summary and collect errors
	successCount := 0
	failCount := 0
	skippedCount := 0
	var failedItems []domain.ApplyResult

	for _, result := range results {
		switch result.Item.Status {
		case domain.StatusSuccess:
			successCount++
		case domain.StatusFailed:
			failCount++
			failedItems = append(failedItems, result)
		case domain.StatusSkipped:
			skippedCount++
		}
	}

	fmt.Printf("\nSuccess: %d, Failed: %d, Skipped: %d\n", successCount, failCount, skippedCount)

	// Show detailed errors for failed items
	if failCount > 0 {
		fmt.Println("\nFailed items:")
		for _, result := range failedItems {
			targetStr := fmt.Sprintf("%s:%s", result.Item.Target.Kind, result.Item.Target.Identifier)
			if result.Error != nil {
				fmt.Printf("  • %s: %v\n", targetStr, result.Error)
			} else {
				fmt.Printf("  • %s: unknown error\n", targetStr)
			}
		}
	}

	return nil
}
