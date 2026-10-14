package file

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/internal/infra/storage"
	"github.com/anzhiyu-c/anheyu-app/internal/pkg/types"
	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/volume"
)

type imgbedDeleteFileRepo struct {
	repository.FileRepository
	deleted bool
}

func (r *imgbedDeleteFileRepo) FindByIDUnscoped(context.Context, uint) (*model.File, error) {
	return &model.File{ID: 10, OwnerID: 1, Type: model.FileTypeFile, PrimaryEntityID: types.NullUint64{Uint64: 20, Valid: true}}, nil
}
func (r *imgbedDeleteFileRepo) HardDelete(context.Context, uint) error { r.deleted = true; return nil }

type imgbedDeleteEntityRepo struct {
	repository.EntityRepository
	deleted bool
}

func (r *imgbedDeleteEntityRepo) FindByID(context.Context, uint) (*model.FileStorageEntity, error) {
	return &model.FileStorageEntity{PolicyID: 7, Source: sql.NullString{String: "blog/a.png", Valid: true}}, nil
}
func (r *imgbedDeleteEntityRepo) HardDelete(context.Context, uint) error {
	r.deleted = true
	return nil
}

type imgbedDeleteVersionRepo struct {
	repository.FileEntityRepository
}

func (*imgbedDeleteVersionRepo) DeleteByFileID(context.Context, uint) error { return nil }

type imgbedDeleteMetadataRepo struct{ repository.MetadataRepository }

func (*imgbedDeleteMetadataRepo) DeleteByFileID(context.Context, uint) error { return nil }

type imgbedDeleteLinkRepo struct {
	repository.DirectLinkRepository
}

func (*imgbedDeleteLinkRepo) DeleteByFileID(context.Context, uint) error { return nil }

type imgbedDeletePolicySvc struct{ volume.IStoragePolicyService }

func (*imgbedDeletePolicySvc) GetPolicyByDatabaseID(context.Context, uint) (*model.StoragePolicy, error) {
	return &model.StoragePolicy{Type: constant.PolicyTypeImgBed}, nil
}

type imgbedDeleteFailureProvider struct{ storage.IStorageProvider }

func (*imgbedDeleteFailureProvider) Delete(context.Context, *model.StoragePolicy, []string) error {
	return errors.New("remote deletion denied")
}

func TestImgBedFailedDeleteRetainsFileAndEntity(t *testing.T) {
	fileRepo := &imgbedDeleteFileRepo{}
	entityRepo := &imgbedDeleteEntityRepo{}
	s := &serviceImpl{policySvc: &imgbedDeletePolicySvc{}, storageProviders: map[constant.StoragePolicyType]storage.IStorageProvider{constant.PolicyTypeImgBed: &imgbedDeleteFailureProvider{}}}
	err := s.HardDeleteRecursively(context.Background(), 1, 10, fileRepo, entityRepo, &imgbedDeleteVersionRepo{}, &imgbedDeleteMetadataRepo{}, nil, &imgbedDeleteLinkRepo{})
	if err == nil || fileRepo.deleted || entityRepo.deleted {
		t.Fatalf("failed remote deletion must retain database records: err=%v file=%v entity=%v", err, fileRepo.deleted, entityRepo.deleted)
	}
}
