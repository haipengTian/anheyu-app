/*
 * @Description: 即刻短文表（[custom] 二开新增）
 */
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Essay holds the schema definition for the Essay entity.
type Essay struct {
	ent.Schema
}

// Annotations of the Essay.
func (Essay) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.WithComments(true),
		schema.Comment("即刻短文表"),
	}
}

// Fields of the Essay.
// 长度限制统一由 service 层按「字数」校验，这里不用 MaxLen（ent 的 MaxLen 按字节计）。
func (Essay) Fields() []ent.Field {
	return []ent.Field{
		field.Uint("id"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
		field.Text("content").Default("").Comment("文字内容"),
		field.Time("published_at").Default(time.Now).Comment("发布时间，可手动指定"),
		field.JSON("images", []string{}).Optional().Comment("图片 URL 列表"),
		field.JSON("videos", []string{}).Optional().Comment("视频 URL 列表"),
		field.String("address").Default("").Comment("地点"),
		field.String("source").Default("").Comment("来源"),
		field.String("link_url").Default("").Comment("外链地址"),
		field.String("link_title").Default("").Comment("外链标题"),
		field.String("music_id").Default("").Comment("网易云歌曲 ID"),
		field.String("music_name").Default("").Comment("歌名"),
		field.String("music_artist").Default("").Comment("歌手"),
		field.String("music_cover").Default("").Comment("封面 URL"),
		field.Bool("is_published").Default(true).Comment("是否公开"),
	}
}

// Indexes of the Essay.
func (Essay) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("is_published", "published_at"),
	}
}
