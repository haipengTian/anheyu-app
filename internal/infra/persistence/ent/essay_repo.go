/*
 * @Description: 即刻短文仓储实现（[custom] 二开新增）
 */
package ent

import (
	"context"

	"github.com/anzhiyu-c/anheyu-app/ent"
	"github.com/anzhiyu-c/anheyu-app/ent/essay"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
)

type essayRepo struct {
	db *ent.Client
}

// NewEssayRepo 是 essayRepo 的构造函数。
func NewEssayRepo(db *ent.Client) repository.EssayRepository {
	return &essayRepo{db: db}
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (r *essayRepo) toModel(e *ent.Essay) *model.Essay {
	return &model.Essay{
		ID:          e.ID,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
		Content:     e.Content,
		PublishedAt: e.PublishedAt,
		Images:      nonNilStrings(e.Images),
		Videos:      nonNilStrings(e.Videos),
		Address:     e.Address,
		Source:      e.Source,
		LinkURL:     e.LinkURL,
		LinkTitle:   e.LinkTitle,
		MusicID:     e.MusicID,
		MusicName:   e.MusicName,
		MusicArtist: e.MusicArtist,
		MusicCover:  e.MusicCover,
		IsPublished: e.IsPublished,
	}
}

func mapEssayErr(err error) error {
	if ent.IsNotFound(err) {
		return repository.ErrEssayNotFound
	}
	return err
}

// Create 创建一条短文
func (r *essayRepo) Create(ctx context.Context, m *model.Essay) (*model.Essay, error) {
	created, err := r.db.Essay.Create().
		SetContent(m.Content).
		SetPublishedAt(m.PublishedAt).
		SetImages(nonNilStrings(m.Images)).
		SetVideos(nonNilStrings(m.Videos)).
		SetAddress(m.Address).
		SetSource(m.Source).
		SetLinkURL(m.LinkURL).
		SetLinkTitle(m.LinkTitle).
		SetMusicID(m.MusicID).
		SetMusicName(m.MusicName).
		SetMusicArtist(m.MusicArtist).
		SetMusicCover(m.MusicCover).
		SetIsPublished(m.IsPublished).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(created), nil
}

// Update 整体覆盖一条短文
func (r *essayRepo) Update(ctx context.Context, m *model.Essay) (*model.Essay, error) {
	updated, err := r.db.Essay.UpdateOneID(m.ID).
		SetContent(m.Content).
		SetPublishedAt(m.PublishedAt).
		SetImages(nonNilStrings(m.Images)).
		SetVideos(nonNilStrings(m.Videos)).
		SetAddress(m.Address).
		SetSource(m.Source).
		SetLinkURL(m.LinkURL).
		SetLinkTitle(m.LinkTitle).
		SetMusicID(m.MusicID).
		SetMusicName(m.MusicName).
		SetMusicArtist(m.MusicArtist).
		SetMusicCover(m.MusicCover).
		SetIsPublished(m.IsPublished).
		Save(ctx)
	if err != nil {
		return nil, mapEssayErr(err)
	}
	return r.toModel(updated), nil
}

// Delete 删除一条短文
func (r *essayRepo) Delete(ctx context.Context, id uint) error {
	return mapEssayErr(r.db.Essay.DeleteOneID(id).Exec(ctx))
}

// FindByID 按 ID 查询
func (r *essayRepo) FindByID(ctx context.Context, id uint) (*model.Essay, error) {
	e, err := r.db.Essay.Get(ctx, id)
	if err != nil {
		return nil, mapEssayErr(err)
	}
	return r.toModel(e), nil
}

// List 分页查询，按发布时间倒序
func (r *essayRepo) List(ctx context.Context, opts model.ListEssaysOptions) ([]*model.Essay, int64, error) {
	query := r.db.Essay.Query()
	if opts.OnlyPublished {
		query = query.Where(essay.IsPublished(true))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	entities, err := query.
		Order(ent.Desc(essay.FieldPublishedAt), ent.Desc(essay.FieldID)).
		Offset((opts.Page - 1) * opts.PageSize).
		Limit(opts.PageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	items := make([]*model.Essay, len(entities))
	for i, e := range entities {
		items[i] = r.toModel(e)
	}
	return items, int64(total), nil
}
