package shared

import (
	"fmt"

	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
)

// DisplayResults shows apply results in table format with summary. It prints
// nothing when no item was changed, declined or warned about, since the
// preview already said there was nothing to change.
func DisplayResults(ctx *Context, results []domain.ApplyResult) error {
	// Build results table
	resultPlan := &domain.Plan{Items: make([]domain.PlanItem, len(results))}
	declined := false
	for i, result := range results {
		resultPlan.Items[i] = result.Item
		// Notes describe the state before applying (e.g. which copy opened an
		// item); copies may have been pruned since
		resultPlan.Items[i].Note = ""
		declined = declined || result.Declined
	}

	if resultPlan.Settled() && !declined {
		return nil
	}

	ctx.Section()
	fmt.Println("Results:")

	f := formatter.New("table", nil)
	if err := f.FormatPlan(resultPlan); err != nil {
		return fmt.Errorf("failed to format results: %w", err)
	}

	// The table's Stats line already counts each status; list the details
	var failedItems, declinedItems []domain.ApplyResult
	for _, result := range results {
		switch {
		case result.Item.Status == domain.StatusFailed:
			failedItems = append(failedItems, result)
		case result.Declined:
			declinedItems = append(declinedItems, result)
		}
	}

	if len(declinedItems) > 0 {
		fmt.Println("\nKept current app (declined in the confirmation dialog):")
		for _, result := range declinedItems {
			fmt.Printf("  • %s:%s\n", result.Item.Target.Kind, result.Item.Target.Identifier)
		}
	}

	// Show detailed errors for failed items
	if len(failedItems) > 0 {
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
