/*
Copyright 2020 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/digitalocean/godo"
	"github.com/pkg/errors"
	"github.com/satori/uuid"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"golang.org/x/oauth2"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// VolumeSnapshotter handles talking to DigitalOcean API & logging
type VolumeSnapshotter struct {
	client *godo.Client
	config map[string]string
	logrus.FieldLogger
}

var _ velero.VolumeSnapshotter = (*VolumeSnapshotter)(nil)

// TokenSource is a DigitalOcean API token
type TokenSource struct {
	AccessToken string
}

// Init the plugin
func (b *VolumeSnapshotter) Init(config map[string]string) error {
	b.Infof("BlockStore.Init called")
	b.config = config

	tokenSource := &TokenSource{
		AccessToken: os.Getenv("DIGITALOCEAN_TOKEN"),
	}

	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	b.client = godo.NewClient(oauthClient)

	return nil
}

// Token returns an oauth2 token from an API key
func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}

	return token, nil
}

// CreateVolumeFromSnapshot makes a volume from a stored backup snapshot
func (b *VolumeSnapshotter) CreateVolumeFromSnapshot(snapshotID string, volumeType string, volumeAZ string, iops *int64) (volumeID string, err error) {
	b.Infof("CreateVolumeFromSnapshot called with snapshotID %s", snapshotID)

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

// GetVolumeInfo fetches volume information from the DigitalOcean API
func (b *VolumeSnapshotter) GetVolumeInfo(volumeID string, volumeAZ string) (string, *int64, error) {
	b.Infof("GetVolumeInfo called with volumeID %s", volumeID)

	ctx := context.TODO()

	volume, _, err := b.client.Storage.GetVolume(ctx, volumeID)
	if err != nil {
		b.Errorf("Storage.GetVolumeInfo returned error: %v", err)
	}

	return volume.FilesystemType, nil, nil
}

// IsVolumeReady just returns true
func (b *VolumeSnapshotter) IsVolumeReady(volumeID string, volumeAZ string) (ready bool, err error) {
	return true, nil
}

// CreateSnapshot makes a snapshot of a persistent volume using the DigitalOcean API
func (b *VolumeSnapshotter) CreateSnapshot(volumeID string, volumeAZ string, tags map[string]string) (string, error) {
	b.Infof("CreateSnapshot called with volumeID %s", volumeID)

	var snapshotName string

	snapshotName = "pvs-" + volumeID + "-" + uuid.NewV4().String()

	createRequest := &godo.SnapshotCreateRequest{
		VolumeID:    volumeID,
		Name:        snapshotName,
		Description: "velero snapshot of pv-" + volumeID,
	}

	ctx := context.TODO()

	b.Infof("CreateSnapshot trying to create snapshot")
	newSnapshot, _, err := b.client.Storage.CreateSnapshot(ctx, createRequest)
	if err != nil {
		b.Errorf("Storage.CreateSnapshot returned error: %v", err)
	}

	return newSnapshot.ID, nil
}

// DeleteSnapshot deletes a backup snapshot
func (b *VolumeSnapshotter) DeleteSnapshot(snapshotID string) error {
	b.Infof("DeleteSnapshot called with snapshotID %v", snapshotID)

	ctx := context.TODO()

	_, err := b.client.Storage.DeleteSnapshot(ctx, snapshotID)
	if err != nil {
		b.Errorf("Storage.DeleteSnapshot returned error: %v", err)
	}

	return err
}

// GetVolumeID Get the volume ID from the spec
func (b *VolumeSnapshotter) GetVolumeID(unstructuredPV runtime.Unstructured) (string, error) {
	b.Infof("GetVolumeID called with unstructuredPV %v", unstructuredPV)

	pv := new(v1.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.UnstructuredContent(), pv); err != nil {
		return "", errors.WithStack(err)
	}
	if pv.Spec.CSI == nil {
		return "", fmt.Errorf("unable to retrieve CSI Spec from pv %+v", pv)
	}
	if pv.Spec.CSI.VolumeHandle == "" {
		return "", fmt.Errorf("unable to retrieve Volume handle from pv %+v", pv)
	}
	return pv.Spec.CSI.VolumeHandle, nil
}

// SetVolumeID Set the volume ID in the spec
func (b *VolumeSnapshotter) SetVolumeID(unstructuredPV runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	b.Infof("SetVolumeID called with unstructuredPV %v and volumeID %s", unstructuredPV, volumeID)

	pv := new(v1.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.UnstructuredContent(), pv); err != nil {
		return nil, errors.WithStack(err)
	}

	if pv.Spec.CSI == nil {
		return nil, fmt.Errorf("spec.CSI not found from pv %+v", pv)
	}

	pv.Spec.CSI.VolumeHandle = volumeID

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pv)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: res}, nil
}
