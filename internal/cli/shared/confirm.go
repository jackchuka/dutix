package shared

import (
	"fmt"
	"os"

	"github.com/jackchuka/dutix/internal/domain"
)

// PrintConfirmationNotice tells the user how many system confirmation dialogs
// to expect when macOS asks to confirm each default app change
func PrintConfirmationNotice(ctx *Context, plan *domain.Plan) {
	n := plan.ConfirmableChanges()
	if n == 0 || !ctx.Bridge.PromptsForDefaultAppChanges() {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "macOS will ask you to confirm %d change(s) in a system dialog; choose \"Use\" for each (unanswered dialogs time out after 2 minutes).\n\n", n)
}
