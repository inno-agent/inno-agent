terraform {
  required_providers {
    zitadel = {
      source  = "zitadel/zitadel"
      version = "~> 2.0"
    }
  }
}

provider "zitadel" {
  domain           = var.zitadel_domain
  port             = var.zitadel_port
  insecure         = true
  jwt_profile_file = var.zitadel_jwt_profile_file
}

data "zitadel_orgs" "all" {}

locals {
  default_org_id = tolist(data.zitadel_orgs.all.ids)[0]
}

resource "zitadel_project" "inno_agent" {
  name   = "inno-agent"
  org_id = local.default_org_id

  project_role_assertion   = false
  project_role_check       = false
  has_project_check        = false
  private_labeling_setting = "PRIVATE_LABELING_SETTING_UNSPECIFIED"
}

resource "zitadel_project_role" "user" {
  org_id       = local.default_org_id
  project_id   = zitadel_project.inno_agent.id
  role_key     = "user"
  display_name = "User"
}

resource "zitadel_project_role" "admin" {
  org_id       = local.default_org_id
  project_id   = zitadel_project.inno_agent.id
  role_key     = "admin"
  display_name = "Admin"
}

resource "zitadel_application_oidc" "auth_client" {
  org_id     = local.default_org_id
  project_id = zitadel_project.inno_agent.id

  name = "auth-client"

  redirect_uris             = ["http://${var.zitadel_domain}/callback"]
  response_types            = ["OIDC_RESPONSE_TYPE_CODE"]
  grant_types               = ["OIDC_GRANT_TYPE_AUTHORIZATION_CODE"]
  app_type                  = "OIDC_APP_TYPE_NATIVE"
  auth_method_type          = "OIDC_AUTH_METHOD_TYPE_NONE"
  post_logout_redirect_uris = []
  dev_mode                  = true
}
