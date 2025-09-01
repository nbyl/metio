# Configure OpenID Connect (OIDC) between GCP and GitLab

## Use-cases

* Retrieve temporary credentials from GCP to access cloud services
* Use credentials to retrieve secrets or deploy to an environment
* Scope role to branch or project

For additional details, see [documentation here](https://cloud.google.com/iam/docs/configuring-workload-identity-federation#oidc)


## Steps

1. Execute Terraform:

> `terraform apply -var "gcp_project_name=<PROJECT_NAME>" -var gitlab_namespace_path=<REPOSITORY_NAMESPACE> -var gitlab_project_id=<REPOSITORY_ID>`

2. Populate the following CI variables from the Terraform output
   * `GCP_WORKLOAD_IDENTITY_PROVIDER`
   * `POOL_ID`
   * `PROJECT_NUMBER`
   * `PROVIDER_ID`
   * `SERVICE_ACCOUNT_EMAIL`


## Resources

- https://cloud.google.com/iam/docs/configuring-workload-identity-federation#oidc
