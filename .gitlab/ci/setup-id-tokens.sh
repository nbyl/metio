#!/bin/bash

set -e
set -x

apk add --update --no-cache curl ca-certificates bash python3

cd /
curl -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-linux-x86_64.tar.gz
tar xzf google-cloud-cli-linux-x86_64.tar.gz
/google-cloud-sdk/install.sh
cd "${CI_PROJECT_DIR}" || exit 1

export PATH="/google-cloud-sdk/bin:${PATH}"

echo "${GITLAB_TOKEN}" > "${CI_PROJECT_DIR}/.ci_job_jwt_file"
gcloud iam workload-identity-pools create-cred-config "${GCP_WORKLOAD_IDENTITY_PROVIDER}" \
      --service-account="${GCP_SERVICE_ACCOUNT}" \
      --output-file="${CI_PROJECT_DIR}/.gcp_temp_cred.json" \
      --credential-source-file="${CI_PROJECT_DIR}/.ci_job_jwt_file"
gcloud auth login --cred-file="${CI_PROJECT_DIR}/.gcp_temp_cred.json"
gcloud auth list

cat "${CI_PROJECT_DIR}/.gcp_temp_cred.json"
cat "${CI_PROJECT_DIR}/.ci_job_jwt_file"

export GOOGLE_APPLICATION_CREDENTIALS=${CI_PROJECT_DIR}/.gcp_temp_cred.json
