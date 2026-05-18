package file

import (
	"context"
	"fmt"
	"strings"

	"github.com/deposist/s-ui-rus-inst/database/importxui"
)

type Source struct {
	Path string
}

func New(path string) Source {
	return Source{Path: path}
}

func (s Source) Acquire(ctx context.Context) (string, func(), error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(s.Path) == "" {
		return "", nil, fmt.Errorf("missing source path")
	}
	if err := importxui.ValidateSQLiteSource(s.Path); err != nil {
		return "", nil, err
	}
	return s.Path, func() {}, nil
}
