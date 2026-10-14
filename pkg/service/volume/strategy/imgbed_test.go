package strategy

import (
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"testing"
)

func TestImgBedSettings(t *testing.T) {
	s := NewImgBedStrategy()
	if s == nil {
		t.Fatal("ImgBed strategy is missing")
	}
	settings := map[string]interface{}{}
	if err := s.ValidateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if settings[constant.UploadMethodSettingKey] != "server" || settings["upload_channel"] != "s3" {
		t.Fatalf("unsafe defaults: %v", settings)
	}
	for _, invalid := range []map[string]interface{}{nil, {"upload_method": "client"}, {"upload_channel": "external"}, {"upload_channel": 123}, {"channel_name": true}, {"custom_proxy": true}, {"cdn_domain": "https://other.example"}} {
		if err := s.ValidateSettings(invalid); err == nil {
			t.Errorf("invalid settings accepted: %v", invalid)
		}
	}
	for _, channel := range []string{"s3", "cfr2", "telegram", "discord", "huggingface", "webdav"} {
		if err := s.ValidateSettings(map[string]interface{}{"upload_channel": channel}); err != nil {
			t.Errorf("channel %s: %v", channel, err)
		}
	}
}
