#!/bin/bash

apk add --update --no-cache curl ca-certificates bash python3

curl -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-linux-x86_64.tar.gz
tar xzf google-cloud-cli-linux-x86_64.tar.gz
/google-cloud-sdk/install.sh

export PATH="/google-cloud-sdk/bin:${PATH}"

gcloud iam workload-identity-pools create-cred-config "${GCP_WORKLOAD_IDENTITY_PROVIDER}" \
      --service-account="${GCP_SERVICE_ACCOUNT}" \
      --output-file="${CI_PROJECT_DIR}/.gcp_temp_cred.json" \
      --credential-source-file="${CI_PROJECT_DIR}/.ci_job_jwt_file"

export GOOGLE_APPLICATION_CREDENTIALS=${CI_PROJECT_DIR}/.gcp_temp_cred.json
