package repositories

import (
	"context"

	"testcenter-server/models"
)

type ProjectRepository interface {
	List(ctx context.Context) ([]models.Project, error)
	GetByID(ctx context.Context, id string) (*models.Project, error)
	GetByCode(ctx context.Context, code string) (*models.Project, error)
	Save(ctx context.Context, project models.Project) error
	Delete(ctx context.Context, id string) error
}
