package repositories

import (
	"context"

	"testcenter-server/models"
)

type SandboxAccountRepository interface {
	List(ctx context.Context) ([]models.SandboxAccount, error)
	GetByID(ctx context.Context, id string) (*models.SandboxAccount, error)
	Save(ctx context.Context, account models.SandboxAccount) error
	Delete(ctx context.Context, id string) error
}
