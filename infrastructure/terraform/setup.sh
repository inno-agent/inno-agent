#!/bin/sh
set -e

KEY_FILE="/machinekey/terraform.json"

echo "Waiting for machine key from Zitadel setup..."
until [ -f "$KEY_FILE" ]; do
  sleep 2
done

echo "Machine key found. Running terraform..."
rm -f terraform.tfstate terraform.tfstate.backup
terraform init -no-color

success=false
for i in 1 2 3; do
  terraform apply -no-color -auto-approve -var="zitadel_jwt_profile_file=${KEY_FILE}" && success=true && break
  [ "$i" -lt 3 ] && echo "Retry $i/3..." && sleep 5
done
if [ "$success" != "true" ]; then
  echo "ERROR: terraform apply failed after 3 attempts"
  exit 1
fi

printf 'ZITADEL_CLIENT_ID=%s\n' "$(terraform output -no-color -raw client_id)" > /setup/auth.env
echo "Terraform setup complete."
