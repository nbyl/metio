#!/bin/bash

set -e
set -x

echo "${GITLAB_OIDC_TOKEN}" > "${CI_PROJECT_DIR}/.ci_job_jwt_file"
gcloud iam workload-identity-pools create-cred-config "${GCP_WORKLOAD_IDENTITY_PROVIDER}" \
      --service-account="${GCP_SERVICE_ACCOUNT}" \
      --output-file="${CI_PROJECT_DIR}/.gcp_temp_cred.json" \
      --credential-source-file="${CI_PROJECT_DIR}/.ci_job_jwt_file"
gcloud auth login --cred-file="${CI_PROJECT_DIR}/.gcp_temp_cred.json"
gcloud auth list
