# Metio Minecraft Manager Minecraft Setup

# Installation

* Create a GCS bucket called `minecraft-byl-tofu-state`
* Deploy opentofu stuff:

```
gcloud auth application-default login
tofu init
tofu apply -auto-approve
```

# Development

## Initial Setup for CI/CD pipeline

```
gcloud artifacts repositories create metio \
    --repository-format=docker \
    --location=europe-west3 \
    --immutable-tags \
    --disable-vulnerability-scanning
cd pipeline/setup
tofu apply -var "gcp_project_name=<PROJECT_NAME>" -var gitlab_namespace_path=<REPOSITORY_NAMESPACE> -var gitlab_project_id=<REPOSITORY_ID>
```

## Local Development

* Open the workspace in a devcontainer and run the following commands:

```
gcloud auth application-default login
air
```

TODO: select workspace for development

# Links

* https://web.archive.org/web/20190528003754/https://cloud.google.com/solutions/gaming/minecraft-server#try_an_alternative_minecraft_server
* https://cloud.google.com/blog/products/management-tools/brick-by-brick-learn-gcp-by-setting-up-a-minecraft-server?hl=en
