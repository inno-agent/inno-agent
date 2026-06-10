#!/bin/sh
set -e

KEY_FILE="/machinekey/terraform.json"

rm -f /setup/identity.env

echo "Waiting for machine key from Zitadel setup..."
until [ -f "$KEY_FILE" ]; do
  sleep 2
done

echo "Machine key found. Running terraform..."
terraform init -no-color -plugin-dir=/workspace/.terraform/providers

set +e
terraform apply -no-color -auto-approve -var="zitadel_jwt_profile_file=${KEY_FILE}"
APPLY_EXIT=$?
set -e

if [ $APPLY_EXIT -ne 0 ]; then
  echo "First apply failed, clearing stale state and retrying..."
  rm -f terraform.tfstate terraform.tfstate.backup
  terraform apply -no-color -auto-approve -var="zitadel_jwt_profile_file=${KEY_FILE}"
fi

printf 'ZITADEL_CLIENT_ID=%s\n' "$(terraform output -no-color -raw client_id)" > /setup/identity.env
echo "Terraform setup complete."
