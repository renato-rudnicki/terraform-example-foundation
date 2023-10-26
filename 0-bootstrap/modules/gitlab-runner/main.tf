/**
 * Copyright 2023 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

locals {
  service_account = var.service_account == "" ? google_service_account.runner_service_account[0].email : var.service_account
}

/*****************************************
  Optional Runner Networking
 *****************************************/
module "vpc_network" {
  source       = "terraform-google-modules/network/google"
  version      = "~> 7.0"
  project_id   = var.project_id
  network_name = "my-network"
  mtu          = 1460

  subnets = [
    {
      subnet_name   = "subnet-01"
      subnet_ip     = "10.10.10.0/24"
      subnet_region = "us-central1"
      #   subnet_private_access = "true"
      #   subnet_flow_logs          = "true"
      #   subnet_flow_logs_interval = "INTERVAL_10_MIN"
      #   subnet_flow_logs_sampling = 0.7
      #   subnet_flow_logs_metadata = "INCLUDE_ALL_METADATA"
      #   subnet_flow_logs_filter   = "false"
    },
  ]
}

resource "google_compute_router" "default" {
  count = var.create_network ? 1 : 0

  name    = "${var.network_name}-router"
  network = module.vpc_network.network_self_link
  region  = var.region
  project = var.project_id
}

// Nat is being used here since internet access is required for the Runner Network. Other internet access can be setup instead of NAT resource (e.g: Secure Web Proxy)
resource "google_compute_router_nat" "nat" {
  count = var.create_network ? 1 : 0

  project                            = var.project_id
  name                               = "${var.network_name}-nat"
  router                             = google_compute_router.default[0].name
  region                             = google_compute_router.default[0].region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

resource "google_dns_policy" "default_policy" {
  project                   = var.project_id
  name                      = "dns-gl-runner-default-policy"
  enable_inbound_forwarding = true
  enable_logging            = true

  networks {
    network_url = module.vpc_network.network_self_link
  }
}

/*****************************************
  IAM Bindings GCE SVC
 *****************************************/
resource "google_service_account" "runner_service_account" {
  count = var.service_account == "" ? 1 : 0

  project      = var.project_id
  account_id   = "runner-service-account"
  display_name = "GitLab Runner GCE Service Account"
}

/*****************************************
  Runner Secrets
 *****************************************/
resource "google_secret_manager_secret" "gl-secret" {
  provider = google-beta

  project   = var.project_id
  secret_id = "gl-token"

  labels = {
    label = "gl-token"
  }

  replication {
    user_managed {
      replicas {
        location = var.region
      }
    }
  }
}
resource "google_secret_manager_secret_version" "gl-secret-version" {
  provider = google-beta

  secret = google_secret_manager_secret.gl-secret.id
  secret_data = jsonencode({
    "REPO_NAME"    = var.repo_name
    "REPO_OWNER"   = var.repo_owner
    "GITLAB_TOKEN" = var.gitlab_token
    "LABELS"       = join(",", var.gl_runner_labels)
  })
}

resource "google_secret_manager_secret_iam_member" "gl-secret-member" {
  provider = google-beta

  project   = var.project_id
  secret_id = google_secret_manager_secret.gl-secret.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${local.service_account}"
}

/*****************************************
  Runner GCE Instance Template
 *****************************************/
module "mig_template" {
  source  = "terraform-google-modules/vm/google//modules/instance_template"
  version = "~> 7.0"

  project_id         = var.project_id
  machine_type       = var.machine_type
  network_ip         = var.network_ip
  network            = module.vpc_network.network_name
  subnetwork         = module.vpc_network.subnets_names[0]
  region             = var.region
  subnetwork_project = var.project_id
  service_account = {
    email = local.service_account
    scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
  }
  disk_size_gb         = 100
  disk_type            = "pd-ssd"
  auto_delete          = true
  name_prefix          = "gl-runner"
  source_image_family  = var.source_image_family
  source_image_project = var.source_image_project
  startup_script       = file("${abspath(path.module)}/startup_script.sh")
  source_image         = var.source_image
  metadata = merge({
    "secret-id" = google_secret_manager_secret_version.gl-secret-version.name
  }, var.custom_metadata)
  tags = [
    "gl-runner-vm"
  ]
}
/*****************************************
  Runner MIG
 *****************************************/
module "mig" {
  source  = "terraform-google-modules/vm/google//modules/mig"
  version = "~> 7.0"

  project_id         = var.project_id
  subnetwork_project = var.project_id
  hostname           = var.instance_name
  region             = var.region
  instance_template  = module.mig_template.self_link

  /* autoscaler */
  autoscaling_enabled = true
  min_replicas        = var.min_replicas
  max_replicas        = var.max_replicas
  cooldown_period     = var.cooldown_period
}