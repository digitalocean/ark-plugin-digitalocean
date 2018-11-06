package main

import (
	"context"
	"os"

	"github.com/digitalocean/godo"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/satori/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/runtime"
)

type TokenSource struct {
	AccessToken string
}

// Plugin for containing state for the blockstore plugin
type BlockStore struct {
	client    *godo.Client
	config    map[string]string
	logrus.FieldLogger
}

var _ cloudprovider.BlockStore = (*BlockStore)(nil)

// Init the plugin
func (b *BlockStore) Init(config map[string]string) error {
	b.Infof("BlockStore.Init called")
	b.config = config

	tokenSource := &TokenSource{
		AccessToken: os.Getenv("DIGITALOCEAN_TOKEN"),
	}

	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	b.client = godo.NewClient(oauthClient)

	return nil
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}

	return token, nil
}

func (b *BlockStore) CreateVolumeFromSnapshot(snapshotID string, volumeType string, volumeAZ string, iops *int64) (volumeID string, err error) {
	b.Infof("CreateVolumeFromSnapshot called")
	ctx := context.TODO()

	snapshot, _, err := b.client.Storage.GetSnapshot(ctx, snapshotID)
	if err != nil {
		b.Errorf("Storage.GetSnapshot returned error: %v", err)
	}

	diskSize := snapshot.MinDiskSize

	createRequest := &godo.VolumeCreateRequest{
		Name:          "restore-" + uuid.NewV4().String(),
		SnapshotID:    snapshotID,
		SizeGigaBytes: int64(diskSize),
	}

	newVolume, _, err := b.client.Storage.CreateVolume(ctx, createRequest)
	if err != nil {
		b.Errorf("Storage.CreateVolume returned error: %v", err)
	}

	return newVolume.ID, nil
}

func (b *BlockStore) GetVolumeInfo(volumeID string, volumeAZ string) (string, *int64, error) {
	b.Infof("GetVolumeInfo called")
	ctx := context.TODO()

	volume, _, err := b.client.Storage.GetVolume(ctx, volumeID)
	if err != nil {
		b.Errorf("Storage.GetVolumeInfo returned error: %v", err)
	}

	return volume.FilesystemType, nil, nil
}

func (b *BlockStore) IsVolumeReady(volumeID string, volumeAZ string) (ready bool, err error) {
	return true, nil
}

func (b *BlockStore) CreateSnapshot(volumeID string, volumeAZ string, tags map[string]string) (string, error) {
	b.Infof("CreateSnapshot called")
	var snapshotName string

	snapshotName = "pvs-" + volumeID + "-" + uuid.NewV4().String()

	createRequest := &godo.SnapshotCreateRequest{
		VolumeID:    volumeID,
		Name:        snapshotName,
		Description: "Ark snapshot of pv-" + volumeID,
	}

	ctx := context.TODO()

	b.Infof("CreateSnapshot trying to create snapshot")
	newSnapshot, _, err := b.client.Storage.CreateSnapshot(ctx, createRequest)
	if err != nil {
		b.Errorf("Storage.CreateSnapshot returned error: %v", err)
	}

	return newSnapshot.ID, nil
}

func (b *BlockStore) DeleteSnapshot(snapshotID string) error {
	b.Infof("DeleteSnapshot called")
	ctx := context.TODO()

	_, err := b.client.Storage.DeleteSnapshot(ctx, snapshotID)
	if err != nil {
		b.Errorf("Storage.DeleteSnapshot returned error: %v", err)
	}

	return err
}

func (b *BlockStore) GetVolumeID(pv runtime.Unstructured) (string, error) {
	b.Infof("GetVolumeID called")
	if !collections.Exists(pv.UnstructuredContent(), "spec.csi") {
		b.Error("Plugin failed to get volume ID.")
		return "", nil
	}

	volumeID, err := collections.GetString(pv.UnstructuredContent(), "spec.csi.volumeHandle")
	if err != nil {
		return "", err
	}

	return volumeID, nil
}

func (b *BlockStore) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	b.Infof("SetVolumeID called")
	do, err := collections.GetMap(pv.UnstructuredContent(), "spec.csi")
	if err != nil {
		b.Error("Could not find pv data in UnstructuredContent")
		return nil, err
	}

	do["volumeHandle"] = volumeID

	return pv, nil
}
