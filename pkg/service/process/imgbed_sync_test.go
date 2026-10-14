package process

import (
	"context"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

func TestImgBedSyncPreservesBlogVirtualFiles(t *testing.T) {
	// Remote identifiers are immutable and cannot be reconciled by display name.
	// No repositories/providers should be touched by a directory sync.
	s := &syncService{}
	policy := &model.StoragePolicy{Type: constant.StoragePolicyType("imgbed"), VirtualPath: "/imgbed"}
	if err := s.SyncDirectory(context.Background(), 1, policy, "/imgbed/photos"); err != nil {
		t.Fatalf("ImgBed directory must retain blog-managed files: %v", err)
	}
}
