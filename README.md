## Getting Started

This README explains how to install and configure the DigitalOcean Block Storage provider plugin for [Velero](https://velero.io). The plugin is designed to create filesystem  snapshots of Block Storage backed [PersistentVolumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) that are used in a Kubernetes cluster running on DigitalOcean.

- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Credentials setup](#credentials-setup)
  - [Velero installation](#velero-installation)
  - [Snapshot configuration](#snapshot-configuration)
  - [Backup and restore example](#backup-and-restore-example)
  - [Build the plugin](#build-the-plugin)

### Prerequisites

* A Kubernetes cluster running on DigitalOcean. It can be a managed cluster or self-hosted
* DigitalOcean account and resources
  * [API personal access token](https://www.digitalocean.com/docs/api/create-personal-access-token/)
  * [Spaces access keys](https://www.digitalocean.com/docs/spaces/how-to/administrative-access/)
  * Spaces bucket
  * Spaces bucket region
* [Velero](https://velero.io/docs/v1.2.0/basic-install/) v1.20 or newer & prerequisites

### Credentials setup

1. To use this plugin with Velero to create persistent volume snapshots, you will need a [DigitalOcean API token](https://www.digitalocean.com/docs/api/create-personal-access-token/). Create one before proceeding with the rest of these steps.

2. For the object storage Velero component, generate a [Spaces access key and secret key](https://www.digitalocean.com/docs/spaces/how-to/administrative-access/)


### Velero installation

1. Complete the Prerequisites and Credentials setup steps mentioned above.
   
2. Clone this repository. `cd` into the `examples` directory and edit the `cloud-credentials` file. The file will look like this:

    ```
    [default]
    aws_access_key_id=<AWS_ACCESS_KEY_ID>
    aws_secret_access_key=<AWS_SECRET_ACCESS_KEY>
    ```

Edit the `<AWS_ACCESS_KEY_ID>` and `<AWS_SECRET_ACCESS_KEY>` placeholders to use your DigitalOcean Spaces keys. Be sure to remove the `<` and `>` characters.

3. Still in the `examples` directory, edit the `01-velero-secret.patch.yaml` file. It should look like this:

    ```
    ---
    apiVersion: v1
    kind: Secret
    stringData:
    digitalocean_token: <DIGITALOCEAN_API_TOKEN>
    type: Opaque
    ```

   * Change the entire `<DIGITALOCEAN_API_TOKEN>` portion to use your DigitalOcean personal API token. The line should look something like `digitalocean_token: 18a0d730c0e0....`


4. Now you're ready to install velero, configure the snapshot storage location, and work with backups. Ensure that you edit each of the following settings to match your Spaces configuration befor running the `velero install` command:
   
   * `--bucket velero-backups` - Ensure you change the `velero-backups` value to match the name of your Space.
   * `--backup-location-config s3Url=https://nyc3.digitaloceanspaces.com,region=nyc3` - Change the URL and region to match your Space's settings. Specifically, edit the `nyc3` portion in both to match the region where your Space is hosted. Use one of `nyc3`, `sfo2`, `sgp1`, or `fra1` depending on your region.

5. Now run the install command:

    ```
    velero install \
        --provider velero.io/aws \
        --bucket velero-backups \
        --plugins velero/velero-plugin-for-aws:v1.0.0,digitalocean/velero-plugin:v1.0.0 \
        --backup-location-config s3Url=https://nyc3.digitaloceanspaces.com,region=nyc3 \
        --use-volume-snapshots=false \
        --secret-file=./cloud-credentials
    ```

### Snapshot configuration

1. Enable the `digitalocean/velero-plugin:v1.0.0` snapshot provider. This command will configure Velero to use the plugin for persistent volume snapshots.

    ```
    velero snapshot-location create default --provider digitalocean.com/velero
    ```

2. Patch the `cloud-credentials` Kubernetes Secret object that the `velero install` command installed in the cluster. This command will add your DigitalOcean API token to the `cloud-credentials` object so that this plugin can use the DigitalOcean API:


    ```
    kubectl patch secret cloud-credentials -p "$(cat 01-velero-secret.patch.yaml)" --namespace velero
    ```

3. Patch the `velero` Kubernetes Deployment to expose your API token to the Velero pod(s). Velero needs this change in order to authenticate to the DigitalOcean API when manipulating snapshots:

    ```
    kubectl patch deployment velero -p "$(cat 02-velero-deployment.patch.yaml)" --namespace velero
    ```


### Backup and restore example

1. Install the Nginx `examples/nginx-example.yaml` Deployment into your cluster. The example uses a persistent volume for Nginx logs. It also creates a LoadBalancer with a public IP address:

    ```
    kubectl apply -f examples/nginx-example.yaml
    ```

2. Ensure that your Nginx Deployment is running and there is a Service with an `EXTERNAL-IP` (`kubectl get service --namespace nginx-example`). Browse the IP a few times to write some log entries to the persistent volume. Then create a backup with Velero:

    ```
    velero backup create nginx-backup --selector app=nginx --snapshot-volumes=true
    velero backup describe nginx-backup --details
    ```

3. The various backup files will be in your Spaces bucket. A snapshot of the persistent volume will be listed in the DigitalOcean control panel under the *Images* link. Now you can simulate a disaster by deleting the `nginx-example` namespace.

    ```
    kubectl delete namespace nginx-example
    ```

4. Once the delete finishes, restore the `nginx-backup` backup:

    ```
    velero restore create --from-backup nginx-backup
    ```

5. Check the restored PersistentVolume, Deployment, and Service are back using `kubectl`:
    ```
    kubectl get persistentvolume --namespace nginx-example
    kubectl get service --namespace nginx-example
    kubectl get deployment --namespace nginx-example
    ```

### Build the plugin

```
make clean
make container IMAGE=digitalocean/velero-plugin:dev
```
