package importxui

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/deposist/s-ui-rus-inst/database"
)

var ErrRemoteDisabled = errors.New("xui_remote_disabled")

type Source interface {
	Acquire(ctx context.Context) (localPath string, cleanup func(), err error)
}

func PlanFromSource(src Source, opts PlanOptions) (*MigrationPlan, error) {
	opts, err := opts.normalized()
	if err != nil {
		return nil, fmt.Errorf("xui-import: %w", err)
	}
	localPath, cleanup, err := src.Acquire(opts.Context)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return nil, fmt.Errorf("xui-import: %w", err)
	}
	return Plan(localPath, opts)
}

func ImportFromSource(src Source, opts Options) (*Report, error) {
	opts, err := opts.normalized()
	if err != nil {
		return &Report{}, fmt.Errorf("xui-import: %w", err)
	}
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	localPath, cleanup, err := src.Acquire(opts.Context)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return &Report{}, fmt.Errorf("xui-import: %w", err)
	}
	return Import(localPath, opts)
}

func ApplyFromSource(src Source, plan MigrationPlan, opts ApplyOptions) (*Report, error) {
	opts = opts.normalized()
	localPath, cleanup, err := src.Acquire(opts.Context)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return &Report{}, fmt.Errorf("xui-import: %w", err)
	}
	return Apply(localPath, plan, opts)
}

func ValidateSQLiteSource(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	ok, err := database.IsSQLiteDB(file)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("not_sqlite")
	}
	src, err := openSource(path)
	if err != nil {
		return err
	}
	defer src.close()
	var result string
	if err := src.db.Raw("PRAGMA integrity_check").Scan(&result).Error; err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("invalid sqlite integrity: %s", result)
	}
	return nil
}
