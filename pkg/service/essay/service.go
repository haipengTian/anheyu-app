/*
 * @Description: 即刻短文服务（[custom] 二开新增）
 */
package essay

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
)

const (
	maxContentRunes  = 2000
	maxImages        = 9
	maxVideos        = 4
	maxURLBytes      = 500
	maxShortRunes    = 50
	maxTitleRunes    = 100
	maxMusicIDLength = 20
	defaultPageSize  = 20
	maxPageSize      = 100
)

// ErrEssayNotFound 表示短文不存在（handler 映射为 404）
var ErrEssayNotFound = repository.ErrEssayNotFound

// ValidationError 表示请求参数不合法（handler 映射为 400），消息可直接展示给用户
type ValidationError struct {
	msg string
}

func (e *ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return &ValidationError{msg: fmt.Sprintf(format, args...)}
}

// Service 封装了即刻短文的业务逻辑
type Service struct {
	repo repository.EssayRepository
	now  func() time.Time
}

// NewService 是 Service 的构造函数
func NewService(repo repository.EssayRepository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// Create 创建短文
func (s *Service) Create(ctx context.Context, req *model.EssayRequest) (*model.EssayResponse, error) {
	e, err := s.buildEssay(req, nil)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.Create(ctx, e)
	if err != nil {
		return nil, fmt.Errorf("创建短文失败: %w", err)
	}
	return toResponse(created), nil
}

// Update 整体更新短文；PublishedAt / IsPublished 不传时保留原值
func (s *Service) Update(ctx context.Context, id uint, req *model.EssayRequest) (*model.EssayResponse, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	e, err := s.buildEssay(req, existing)
	if err != nil {
		return nil, err
	}
	e.ID = id
	updated, err := s.repo.Update(ctx, e)
	if err != nil {
		return nil, err
	}
	return toResponse(updated), nil
}

// Delete 删除短文
func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

// Get 获取单条短文（后台用，含未公开的）
func (s *Service) Get(ctx context.Context, id uint) (*model.EssayResponse, error) {
	e, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toResponse(e), nil
}

// ListAdmin 后台列表，包含未公开的短文
func (s *Service) ListAdmin(ctx context.Context, page, pageSize int) (*model.EssayListResponse, error) {
	return s.list(ctx, page, pageSize, false)
}

// ListPublic 前台列表，只返回公开的短文
func (s *Service) ListPublic(ctx context.Context, page, pageSize int) (*model.EssayListResponse, error) {
	return s.list(ctx, page, pageSize, true)
}

func (s *Service) list(ctx context.Context, page, pageSize int, onlyPublished bool) (*model.EssayListResponse, error) {
	page, pageSize = normalizePaging(page, pageSize)
	items, total, err := s.repo.List(ctx, model.ListEssaysOptions{Page: page, PageSize: pageSize, OnlyPublished: onlyPublished})
	if err != nil {
		return nil, fmt.Errorf("查询短文列表失败: %w", err)
	}
	list := make([]model.EssayResponse, len(items))
	for i, e := range items {
		list[i] = *toResponse(e)
	}
	return &model.EssayListResponse{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

func normalizePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// buildEssay 校验并规整请求；existing 非 nil 表示更新
func (s *Service) buildEssay(req *model.EssayRequest, existing *model.Essay) (*model.Essay, error) {
	e := &model.Essay{
		Content:     strings.TrimSpace(req.Content),
		Address:     strings.TrimSpace(req.Address),
		Source:      strings.TrimSpace(req.Source),
		LinkURL:     strings.TrimSpace(req.LinkURL),
		LinkTitle:   strings.TrimSpace(req.LinkTitle),
		MusicID:     strings.TrimSpace(req.MusicID),
		MusicName:   strings.TrimSpace(req.MusicName),
		MusicArtist: strings.TrimSpace(req.MusicArtist),
		MusicCover:  strings.TrimSpace(req.MusicCover),
	}

	var err error
	if e.Images, err = cleanURLList(req.Images, "图片", maxImages); err != nil {
		return nil, err
	}
	if e.Videos, err = cleanURLList(req.Videos, "视频", maxVideos); err != nil {
		return nil, err
	}
	if e.Content == "" && len(e.Images) == 0 && len(e.Videos) == 0 {
		return nil, invalid("文字、图片、视频至少填写一项")
	}
	if err := checkRunes("文字内容", e.Content, maxContentRunes); err != nil {
		return nil, err
	}
	if err := checkRunes("地点", e.Address, maxShortRunes); err != nil {
		return nil, err
	}
	if err := checkRunes("来源", e.Source, maxShortRunes); err != nil {
		return nil, err
	}
	if err := checkOptionalURL("外链地址", e.LinkURL); err != nil {
		return nil, err
	}
	if err := checkRunes("外链标题", e.LinkTitle, maxTitleRunes); err != nil {
		return nil, err
	}
	if e.LinkTitle != "" && e.LinkURL == "" {
		return nil, invalid("填写外链标题时必须填写外链地址")
	}
	if err := checkMusic(e); err != nil {
		return nil, err
	}

	switch {
	case req.PublishedAt != nil:
		e.PublishedAt = *req.PublishedAt
	case existing != nil:
		e.PublishedAt = existing.PublishedAt
	default:
		e.PublishedAt = s.now()
	}
	switch {
	case req.IsPublished != nil:
		e.IsPublished = *req.IsPublished
	case existing != nil:
		e.IsPublished = existing.IsPublished
	default:
		e.IsPublished = true
	}
	return e, nil
}

func checkMusic(e *model.Essay) error {
	if e.MusicID == "" {
		if e.MusicName != "" || e.MusicArtist != "" || e.MusicCover != "" {
			return invalid("填写歌曲信息时必须填写网易云歌曲 ID")
		}
		return nil
	}
	if len(e.MusicID) > maxMusicIDLength || strings.Trim(e.MusicID, "0123456789") != "" {
		return invalid("网易云歌曲 ID 只能是数字，且不超过 %d 位", maxMusicIDLength)
	}
	if err := checkRunes("歌名", e.MusicName, maxTitleRunes); err != nil {
		return err
	}
	if err := checkRunes("歌手", e.MusicArtist, maxTitleRunes); err != nil {
		return err
	}
	return checkOptionalURL("歌曲封面", e.MusicCover)
}

func cleanURLList(raw []string, label string, limit int) ([]string, error) {
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !isHTTPURL(item) {
			return nil, invalid("%s地址必须以 http:// 或 https:// 开头，且不超过 %d 个字符：%s", label, maxURLBytes, item)
		}
		out = append(out, item)
	}
	if len(out) > limit {
		return nil, invalid("%s最多 %d 个", label, limit)
	}
	return out, nil
}

func checkOptionalURL(label, value string) error {
	if value == "" || isHTTPURL(value) {
		return nil
	}
	return invalid("%s必须以 http:// 或 https:// 开头，且不超过 %d 个字符", label, maxURLBytes)
}

func checkRunes(label, value string, limit int) error {
	if utf8.RuneCountInString(value) > limit {
		return invalid("%s不能超过 %d 个字", label, limit)
	}
	return nil
}

func isHTTPURL(raw string) bool {
	if len(raw) > maxURLBytes {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func toResponse(e *model.Essay) *model.EssayResponse {
	return &model.EssayResponse{
		ID:          e.ID,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
		Content:     e.Content,
		PublishedAt: e.PublishedAt,
		Images:      e.Images,
		Videos:      e.Videos,
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

// IsNotFound 供 handler 判断 404
func IsNotFound(err error) bool { return errors.Is(err, ErrEssayNotFound) }
