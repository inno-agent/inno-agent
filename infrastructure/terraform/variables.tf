variable "zitadel_domain" {
  type    = string
  default = "localhost"
}

variable "zitadel_port" {
  type    = string
  default = "8080"
}

variable "app_scheme" {
  type    = string
  default = "http"
}

variable "zitadel_jwt_profile_file" {
  type        = string
  description = "Path to Zitadel machine key JSON. Written by zitadel setup to the machinekey volume; for manual runs: -var=\"zitadel_jwt_profile_file=/path/to/machinekey/terraform.json\""
}
