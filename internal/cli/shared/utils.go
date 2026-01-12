package shared

import (
	"fmt"
	"strings"

	"github.com/jackchuka/dutix/internal/domain"
)

// ParseTargetSpecs parses target specifications from flags
func ParseTargetSpecs(extensions, utis, schemes []string) ([]domain.Target, error) {
	var targets []domain.Target

	// Parse extensions
	for _, ext := range extensions {
		ext = strings.TrimSpace(strings.TrimPrefix(ext, "."))
		if ext != "" {
			targets = append(targets, domain.Target{
				Kind:       domain.TargetKindExtension,
				Identifier: ext,
			})
		}
	}

	// Parse UTIs
	for _, uti := range utis {
		uti = strings.TrimSpace(uti)
		if uti != "" {
			targets = append(targets, domain.Target{
				Kind:       domain.TargetKindUTI,
				Identifier: uti,
			})
		}
	}

	// Parse schemes
	for _, scheme := range schemes {
		scheme = strings.TrimSpace(strings.ToLower(scheme))
		if scheme != "" {
			targets = append(targets, domain.Target{
				Kind:       domain.TargetKindScheme,
				Identifier: scheme,
			})
		}
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets specified")
	}

	return targets, nil
}
