output "client_id" {
  value       = zitadel_application_oidc.auth_client.client_id
  description = "Set as ZITADEL_CLIENT_ID in infrastructure/.env"
  sensitive   = true
}

output "project_id" {
  value = zitadel_project.inno_agent.id
}
