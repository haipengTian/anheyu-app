/*
 * @Description: 即刻短文领域模型（[custom] 二开新增）
 */
package model

import "time"

// Essay 是即刻短文的核心领域模型
type Essay struct {
	ID          uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Content     string
	PublishedAt time.Time
	Images      []string
	Videos      []string
	Address     string
	Source      string
	LinkURL     string
	LinkTitle   string
	MusicID     string
	MusicName   string
	MusicArtist string
	MusicCover  string
	IsPublished bool
}

// EssayRequest 是创建/更新即刻短文的请求体。
// 更新时整体覆盖，只有 PublishedAt / IsPublished 为 nil 时保留原值。
type EssayRequest struct {
	Content     string     `json:"content"`
	PublishedAt *time.Time `json:"published_at"`
	Images      []string   `json:"images"`
	Videos      []string   `json:"videos"`
	Address     string     `json:"address"`
	Source      string     `json:"source"`
	LinkURL     string     `json:"link_url"`
	LinkTitle   string     `json:"link_title"`
	MusicID     string     `json:"music_id"`
	MusicName   string     `json:"music_name"`
	MusicArtist string     `json:"music_artist"`
	MusicCover  string     `json:"music_cover"`
	IsPublished *bool      `json:"is_published"`
}

// EssayResponse 是即刻短文的 API 响应结构
type EssayResponse struct {
	ID          uint      `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Content     string    `json:"content"`
	PublishedAt time.Time `json:"published_at"`
	Images      []string  `json:"images"`
	Videos      []string  `json:"videos"`
	Address     string    `json:"address"`
	Source      string    `json:"source"`
	LinkURL     string    `json:"link_url"`
	LinkTitle   string    `json:"link_title"`
	MusicID     string    `json:"music_id"`
	MusicName   string    `json:"music_name"`
	MusicArtist string    `json:"music_artist"`
	MusicCover  string    `json:"music_cover"`
	IsPublished bool      `json:"is_published"`
}

// EssayListResponse 是即刻短文列表的 API 响应结构
type EssayListResponse struct {
	List     []EssayResponse `json:"list"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
}

// ListEssaysOptions 定义了查询短文列表的选项（Page/PageSize 由 service 规整后传入）
type ListEssaysOptions struct {
	Page          int
	PageSize      int
	OnlyPublished bool
}
