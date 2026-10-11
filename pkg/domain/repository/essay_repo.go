/*
 * @Description: 即刻短文仓储接口（[custom] 二开新增）
 */
package repository

import (
	"context"
	"errors"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

// ErrEssayNotFound 表示短文不存在
var ErrEssayNotFound = errors.New("短文不存在")

// EssayRepository 定义了即刻短文的数据操作契约。
// 列表按 published_at 倒序、id 倒序返回。
type EssayRepository interface {
	Create(ctx context.Context, essay *model.Essay) (*model.Essay, error)
	Update(ctx context.Context, essay *model.Essay) (*model.Essay, error)
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*model.Essay, error)
	List(ctx context.Context, opts model.ListEssaysOptions) ([]*model.Essay, int64, error)
}
