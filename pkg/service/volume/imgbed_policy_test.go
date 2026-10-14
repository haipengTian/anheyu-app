package volume

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/volume/strategy"
)

type imgbedPolicyRepo struct {
	repository.StoragePolicyRepository
	policy *model.StoragePolicy
}

func (r *imgbedPolicyRepo) FindByID(context.Context, uint) (*model.StoragePolicy, error) {
	return r.policy, nil
}

type imgbedPolicyFileRepo struct {
	repository.FileRepository
	children []repository.SyncItem
}

func (*imgbedPolicyFileRepo) FindByID(context.Context, uint) (*model.File, error) {
	return &model.File{ChildrenCount: 0}, nil
}

func (r *imgbedPolicyFileRepo) ListByParentIDUnscoped(context.Context, uint) ([]repository.SyncItem, error) {
	return r.children, nil
}

type imgbedPolicyEntityRepo struct {
	repository.EntityRepository
	used bool
	err  error
}

func (r *imgbedPolicyEntityRepo) IsStoragePolicyUsedByEntities(context.Context, uint) (bool, error) {
	return r.used, r.err
}

type imgbedPolicyTx struct {
	repos   repository.Repositories
	touched bool
}

func (tx *imgbedPolicyTx) Do(_ context.Context, fn func(repository.Repositories) error) error {
	tx.touched = true
	return fn(tx.repos)
}

func TestImgBedPolicyCannotRepointExistingFiles(t *testing.T) {
	node := uint(8)
	for _, scenario := range []string{"entity", "child_with_zero_count", "orphan_entity_without_mount", "query_failure"} {
		for _, change := range []string{"type", "server", "base_path", "local_to_imgbed"} {
			t.Run(scenario+"/"+change, func(t *testing.T) {
				target := &model.StoragePolicy{ID: 7, Type: constant.PolicyTypeImgBed, Server: "https://img.thpvip.xyz", BasePath: "blog/articles", NodeID: &node, VirtualPath: "/imgbed", SecretKey: "synthetic-token"}
				incoming := *target
				switch change {
				case "type":
					incoming.Type = constant.PolicyTypeLocal
				case "server":
					incoming.Server = "https://other.example"
				case "base_path":
					incoming.BasePath = "blog/new"
				case "local_to_imgbed":
					target.Type = constant.PolicyTypeLocal
				}
				files := &imgbedPolicyFileRepo{}
				entities := &imgbedPolicyEntityRepo{used: true}
				if scenario == "child_with_zero_count" {
					entities.used = false
					files.children = []repository.SyncItem{{}}
				}
				if scenario == "orphan_entity_without_mount" {
					target.NodeID = nil
				}
				if scenario == "query_failure" {
					entities.err = errors.New("query failed")
				}
				repo := &imgbedPolicyRepo{policy: target}
				tx := &imgbedPolicyTx{repos: repository.Repositories{StoragePolicy: repo, File: files, Entity: entities}}
				s := &storagePolicyService{repo: repo, fileRepo: files, txManager: tx, strategyManager: strategy.NewManager()}
				err := s.UpdatePolicy(context.Background(), &incoming)
				want := "此策略已有文件"
				if scenario == "query_failure" {
					want = "检查已有文件失败"
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("unsafe update was not blocked by actual records: %v", err)
				}
			})
		}
	}
}
