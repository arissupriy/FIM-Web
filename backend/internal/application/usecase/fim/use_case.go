// Package fim contains FIM-related use cases.
package fim

import (
	"context"

	"ojs-monitor/backend/internal/domain/models"
	"ojs-monitor/backend/internal/domain/repository"
)

// UseCase provides FIM operations
type UseCase struct {
	fimRepo repository.FIMEventRepository
}

// New creates a new FIM use case
func New(fimRepo repository.FIMEventRepository) *UseCase {
	return &UseCase{fimRepo: fimRepo}
}

// ListEvents returns FIM events with filters
func (uc *UseCase) ListEvents(ctx context.Context, projectID int, filters repository.FIMEventFilters) ([]*models.FIMEvent, int, error) {
	filters.Validate()
	return uc.fimRepo.GetByProjectID(ctx, projectID, filters)
}

// GetStats returns FIM event statistics
func (uc *UseCase) GetStats(ctx context.Context, projectID int) (*repository.FIMStats, error) {
	return uc.fimRepo.GetStats(ctx, projectID)
}
