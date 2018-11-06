## Getting Started

The following will describe how to install and configure the DigitalOcean block store plugin for Ark and provide a usage example.

* [Prerequisites](#prerequisites)
* [Quickstart](#quickstart)
* [Block store](#block-store)
* [Object store](#object-store)
* [Backup and restore example](#backup-and-restore-example)
* [Build image](#build-image)

### Prerequisites

* [Kubernetes cluster](https://stackpoint.io/clusters/new?provider=do)
* DigitalOcean account and resources
  * [API personal access token](https://www.digitalocean.com/docs/api/create-personal-access-token/)
  * [Spaces access keys](https://www.digitalocean.com/docs/spaces/how-to/administrative-access/)
  * Spaces bucket
  * Spaces bucket region
* [Heptio Ark](https://heptio.github.io/ark/master/quickstart.html) v0.10.x prerequisites

### Quickstart

This quickstart will describe the installation and configuration of the DigitalOcean block store plugin for Ark as well as the built-in object store using DigitalOcean Spaces. Please review the [Block store](#block-store) and [Object store](#object-store) sections further down in the README for more details on each component.

1. Complete the Heptio Ark prerequisites mentioned above. This generally involves applying the `00-prereqs.yaml` available from the Ark repository:

    ```
    kubectl apply -f examples/00-prereqs.yaml
    ```

2. Update the `examples/credentials-ark` with your Spaces access and secret keys. The file will look like the following:

    ```
    [default]
    aws_access_key_id=<AWS_ACCESS_KEY_ID>
    aws_secret_access_key=<AWS_SECRET_ACCESS_KEY>
    ```

3. Create a Kubernetes `cloud-credentials` secret containing the `credentials-ark` and DigitalOcean API token.

    ```
    kubectl create secret generic cloud-credentials \
        --namespace heptio-ark \
        --from-file cloud=examples/credentials-ark \
        --from-literal digitalocean_token=<DIGITALOCEAN_TOKEN>
    ```

4. Update the `examples/05-ark-backupstoragelocation.yaml` with the DigitalOcean Spaces API URL, bucket, and region and apply the `BackupStorageLocation` configuration.

    ```
    kubectl apply -f examples/05-ark-backupstoragelocation.yaml
    ```

5. Next apply the `VolumeSnapshotLocation` configuration. No updates are required to the YAML.

    ```
    kubectl apply -f examples/06-ark-volumesnapshotlocation.yaml
    ```

6. Now apply the Ark deployment.

    ```
    kubectl apply -f examples/10-deployment.yaml
    ```

7. Finally add the `ark-blockstore-digitalocean` plugin to Ark.

    ```
    ark plugin add quay.io/stackpoint/ark-blockstore-digitalocean:v0.2.0
    ```

### Block store

The block store provider manages snapshots for DigitalOcean persistent volumes.

1. The block store provider requires a personal access token to create and restore snapshots through the DigitalOcean API. This token can be generated through the DigitalOcean Control Panel as describe [here](https://www.digitalocean.com/docs/api/create-personal-access-token/).

2. Once the token is available, create a Secret using the new token.

    ```
    kubectl create secret generic cloud-credentials \
        --namespace heptio-ark \
        --from-literal digitalocean_token=<DIGITALOCEAN_TOKEN>
    ```

3. Ark must be aware of the cloud provider to use with persistent volumes. This is done by adding the `persistentVolumeProvider` to the default Ark Config.

    ```
    kubectl -n heptio-ark edit config default
    ```

    A sample `persistentVolumeProvider` YAML Config section looks like the following:

    ```
    persistentVolumeProvider:
      name: digitalocean
    ```

4. Next the Deployment should be updated with the `cloud-credentials` Secret.

    ```
    kubectl -n heptio-ark edit deployment ark
    ```

    A full Deployment YAML example defining the Secret can be found in `examples/20-deployment.yaml`.

5. Finally, add the `ark-blockstore-digitalocean` plugin to Ark.

    ```
    ark plugin add quay.io/stackpoint/ark-blockstore-digitalocean:latest
    ```

### Object store

The object store uses [DigitalOcean Spaces](https://www.digitalocean.com/products/spaces/) to store the backup files. As Spaces is an S3-compatible object storage solution, the object store will use the Ark built-in `aws` provider.

1. First generate the Spaces access key and secret key in the DigitalOcean Control Panel as described [here](https://www.digitalocean.com/docs/spaces/how-to/administrative-access/).

2. A Spaces bucket must also be created through the DigitalOcean Control Panel before proceeding with Ark configuration. Make note of the bucket name and region as these will be required later.

3. Once the access and secret keys are available, create an S3-compatible `credentials-ark` file with the new keys.

    ```
    [default]
    aws_access_key_id=<DO_ACCESS_KEY_ID>
    aws_secret_access_key=<DO_SECRET_ACCESS_KEY>
    ```

4. The `credentials-ark` file must then be added to the `cloud-credentinals` Secret:

    ```
    kubectl create secret generic cloud-credentials \
        --namespace heptio-ark \
        --from-file cloud=./credentials-ark
    ```

5. Now add the Ark `backupStorageProvider` to the Ark default Config.

    ```
    kubectl -n heptio-ark edit config default
    ```

    Below is a sample `backupStorageProvider` YAML Config section. Be sure to change the bucket and region placeholder values accordingly.

    ```
    backupStorageProvider:
      name: aws
      bucket: <YOUR_BUCKET>
      config:
        region: <REGION>
        s3ForcePathStyle: "true"
        s3Url: https://<REGION>.digitaloceanspaces.com
    ```

6. Finally, the Deployment can be updated with the `cloud-credentials` Secret.

    ```
    kubectl -n heptio-ark edit deployment ark
    ```

    A full Deployment YAML example can be found in `examples/20-deployment.yaml`.


### Backup and restore example

1. Apply the Nginx `examples/nginx-pv.yml` config that uses persistent storage for the log path.

    ```
    kubectl apply -f examples/nginx-pv.yml
    ```

2. Once Nginx deployment is running and available, create a backup using Ark.

    ```
    ark backup create nginx-backup --selector app=nginx
    ark backup describe nginx-backup
    ```

3. The config files should appear in the Spaces bucket and a snapshot taken of the persistent volume. Now you can simulate a disaster by deleting the `nginx-example` namespace.

    ```
    kubectl delete namespace nginx-example
    ```

4. The `nginx-data` backup can now be restored.

    ```
    ark restore create --from-backup nginx-backup
    ```

### Build image

```
make clean
make container IMAGE=quay.io/stackpoint/ark-blockstore-digitalocean
```
