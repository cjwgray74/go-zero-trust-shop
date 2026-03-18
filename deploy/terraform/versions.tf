terraform {
  required_version = ">= 1.5.0"

  required_providers {
    vault = {
      source  = "hashicorp/vault"
      version = "~> 4.4"
    }
    postgresql = {
      source  = "cyrilgdn/postgresql"
      version = "~> 1.23"
    }
    # Optional: use Docker provider if you want Terraform to manage containers
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0"
    }
  }
}