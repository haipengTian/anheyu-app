package strategy

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

type ImgBedStrategy struct{}

func NewImgBedStrategy() IPolicyTypeStrategy { return &ImgBedStrategy{} }
func (*ImgBedStrategy) ValidateSettings(settings map[string]interface{}) error {
	if settings == nil {
		return errors.New("图床 settings 不能为空")
	}
	if method, exists := settings["upload_method"]; exists && method != "server" {
		return errors.New("图床仅支持服务端上传")
	}
	settings["upload_method"] = "server"
	channel, exists := settings["upload_channel"]
	if !exists {
		channel = "s3"
		settings["upload_channel"] = channel
	}
	switch channel {
	case "s3", "cfr2", "telegram", "discord", "huggingface", "webdav":
	default:
		return errors.New("请选择支持的图床上传渠道")
	}
	if value, exists := settings["channel_name"]; exists {
		name, ok := value.(string)
		if !ok || len(name) > 255 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return errors.New("图床渠道名称无效")
		}
	}
	if proxy, exists := settings["custom_proxy"]; exists && proxy != false {
		return errors.New("图床策略使用公开文件直链，不支持下载中转")
	}
	if cdn, exists := settings["cdn_domain"]; exists && cdn != "" {
		return errors.New("图床策略使用图床地址读取文件")
	}
	return nil
}
func (*ImgBedStrategy) GetAuthHandler() IPolicyAuthHandler                       { return nil }
func (*ImgBedStrategy) BeforeDelete(context.Context, *model.StoragePolicy) error { return nil }
