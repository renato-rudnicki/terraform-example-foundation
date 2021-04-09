# 1-org

The purpose of this step is to set up top level shared folders, monitoring & networking projects, org level logging and set baseline security settings through organizational policy. This step also creates the shared repository that host the terraform validator policies.

## Prerequisites

1. 0-bootstrap executed successfully.
2. Cloud Identity / Google Workspace group for security admins.
3. Membership in the security admins group for user running terraform.

## Usage

**Disclaimer:** This step enables [Data Access logs](https://cloud.google.com/logging/docs/audit#data-access) for all services in your organization.
Enabling Data Access logs might result in your project being charged for the additional logs usage.
For details on costs you might incur, go to [Pricing](https://cloud.google.com/stackdriver/pricing).
You can choose not to enable the Data Access logs by setting variable `data_access_logs_enabled` to false.

**Note:** This module creates a sink to export all logs to Google Storage. It also creates sinks to export a subset of security related logs
to Bigquery and Pub/Sub. This will result in additional charges for those copies of logs.
You can change the filters & sinks by modifying the configuration in `envs/shared/log_sinks.tf`.

**Note:** Currently, this module does not enable [bucket policy retention](https://cloud.google.com/storage/docs/bucket-lock) for organization logs, please, enable it if needed.

**Note:** It is possible to enable an organization policy for [OS Login](https://cloud.google.com/compute/docs/oslogin/manage-oslogin-in-an-org) with this module.
OS Login has some [limitations](https://cloud.google.com/compute/docs/instances/managing-instance-access#limitations).
If those limitations do not apply to your workload/environment you can choose to enable the OS Login policy by setting variable `enable_os_login_policy` to `true`.

### Setup to run via Cloud Build

1. Clone repo `gcloud source repos clone gcp-policies --project=YOUR_CLOUD_BUILD_PROJECT_ID` (this is from the terraform output from the previous section, run `terraform output cloudbuild_project_id` in the `0-bootstrap` folder).
1. Navigate into the repo `cd gcp-policies`.
1. Copy contents of policy-library to new repo:
   1. If you are running Linux use: `cp -RT ../terraform-example-foundation/policy-library/ .` (modify accordingly based on your current directory).
   1. If you are running Mac OS use: `cp -pR ../terraform-example-foundation/policy-library/* .` (modify accordingly based on your current directory).
1. Commit changes with `git add .` and `git commit -m 'Your message'`.
1. Push your master branch to the new repo `git push --set-upstream origin master`.
1. Navigate out of the repo `cd ..`.
1. Clone repo `gcloud source repos clone gcp-org --project=YOUR_CLOUD_BUILD_PROJECT_ID`.
1. Navigate into the repo `cd gcp-org` and change to a non production branch `git checkout -b plan`.
1. Copy contents of foundation to new repo `cp -RT ../terraform-example-foundation/1-org/ .` (modify accordingly based on your current directory).
1. Copy cloud build configuration files for terraform `cp ../terraform-example-foundation/build/cloudbuild-tf-* . ` (modify accordingly based on your current directory).
1. Copy terraform wrapper script `cp ../terraform-example-foundation/build/tf-wrapper.sh . ` to the root of your new repository (modify accordingly based on your current directory).
1. Ensure wrapper script can be executed `chmod 755 ./tf-wrapper.sh`.
1. Check if your organization already has a Access Context Manager Policy `gcloud access-context-manager policies list --organization YOUR_ORGANIZATION_ID --format="value(name)"`.
1. Rename `./envs/shared/terraform.example.tfvars` to `./envs/shared/terraform.tfvars` and update the file with values from your environment and bootstrap step (you can re-run `terraform output` in the 0-bootstrap directory to find these values). Make sure that `default_region` is set to a valid [BigQuery dataset region](https://cloud.google.com/bigquery/docs/locations). Also if the previous step showed a numeric value, make sure to un-comment the variable `create_access_context_manager_access_policy = false`. See the shared folder [README.md](./envs/shared/README.md) for additional information on the values in the `terraform.tfvars` file.
1. Commit changes with `git add .` and `git commit -m 'Your message'`.
1. Push your plan branch to trigger a plan `git push --set-upstream origin plan` (the branch `plan` is not a special one. Any branch which name is different from `development`, `non-production` or `production` will trigger a terraform plan).
    1. Review the plan output in your cloud build project https://console.cloud.google.com/cloud-build/builds?project=YOUR_CLOUD_BUILD_PROJECT_ID
1. Merge changes to production branch with `git checkout -b production` and `git push origin production`.
    1. Review the apply output in your cloud build project https://console.cloud.google.com/cloud-build/builds?project=YOUR_CLOUD_BUILD_PROJECT_ID

### Setup to run via Jenkins

1. Clone the repo you created manually in bootstrap: `git clone <YOUR_NEW_REPO-1-org>`.
1. Navigate into the repo `cd YOUR_NEW_REPO_CLONE-1-org` and change to a non production branch `git checkout -b plan`.
1. Copy contents of foundation to new repo `cp -RT ../terraform-example-foundation/1-org/ .` (modify accordingly based on your current directory).
1. Copy contents of policy-library to new repo `cp -RT ../terraform-example-foundation/policy-library/ ./policy-library` (modify accordingly based on your current directory).
1. Copy the Jenkinsfile script `cp ../terraform-example-foundation/build/Jenkinsfile .` to the root of your new repository (modify accordingly based on your current directory).
1. Update the variables located in the `environment {}` section of the `Jenkinsfile` with values from your environment:
    ```
    _TF_SA_EMAIL
    _STATE_BUCKET_NAME
    _PROJECT_ID (the cicd project id)
    ```
1. Copy terraform wrapper script `cp ../terraform-example-foundation/build/tf-wrapper.sh . ` to the root of your new repository (modify accordingly based on your current directory).
1. Ensure wrapper script can be executed `chmod 755 ./tf-wrapper.sh`.
1. Check if your organization already has a Access Context Manager Policy `gcloud access-context-manager policies list --organization YOUR_ORGANIZATION_ID --format="value(name)"`.
1. Rename `./envs/shared/terraform.example.tfvars` to `./envs/shared/terraform.tfvars` and update the file with values from your environment and bootstrap  (you can re-run `terraform output` in the 0-bootstrap directory to find these values). Make sure that `default_region` is set to a valid [BigQuery dataset region](https://cloud.google.com/bigquery/docs/locations). Also if the previous step showed a numeric value, make sure to un-comment the variable `create_access_context_manager_access_policy = false`. See the shared folder [README.md](./envs/shared/README.md) for additional information on the values in the `terraform.tfvars` file.
1. Commit changes with `git add .` and `git commit -m 'Your message'`
1. Push your plan branch `git push --set-upstream origin plan`. The branch `plan` is not a special one. Any branch which name is different from `development`, `non-production` or `production` will trigger a terraform plan.
    - Assuming you configured an automatic trigger in your Jenkins Master (see [Jenkins sub-module README](../0-bootstrap/modules/jenkins-agent)), this will trigger a plan. You can also trigger a Jenkins job manually. Given the many options to do this in Jenkins, it is out of the scope of this document see [Jenkins website](http://www.jenkins.io) for more details.
    1. Review the plan output in your Master's web UI.
1. Merge changes to production branch with `git checkout -b production` and `git push origin production`.
    1. Review the apply output in your Master's web UI (You might want to use the option to "Scan Multibranch Pipeline Now" in your Jenkins Master UI).

1. You can now move to the instructions in the step [2-environments](../2-environments/README.md).

### Run terraform locally

1. Change into 1-org folder.
1. Run `cp ../build/tf-wrapper.sh .`
1. Run `chmod 755 ./tf-wrapper.sh`
1. Change into 1-org/envs/shared/ folder.
1. Rename terraform.example.tfvars to terraform.tfvars and update the file with values from your environment and bootstrap.
1. Update backend.tf with your bucket from bootstrap. You can run
```for i in `find -name 'backend.tf'`; do sed -i 's/UPDATE_ME/<YOUR-BUCKET-NAME>/' $i; done```.
You can run `terraform output gcs_bucket_tfstate` in the 0-bootstap folder to obtain the bucket name.

We will now deploy our environment (production) using this script.
When using Cloud Build or Jenkins as your CI/CD tool each environment corresponds to a branch is the repository for 1-org step and only the corresponding environment is applied.

To use the `validate` option of the `tf-wrapper.sh` script, the latest version of `terraform-validator` must be [installed](https://github.com/forseti-security/policy-library/blob/master/docs/user_guide.md#how-to-use-terraform-validator) in your system and in you `PATH`.

1. Run `./tf-wrapper.sh init production`.
1. Run `./tf-wrapper.sh plan production` and review output.
1. Run `./tf-wrapper.sh validate production $(pwd)/../policy-library <YOUR_CLOUD_BUILD_PROJECT_ID>` and check for violations.
1. Run `./tf-wrapper.sh apply production`.

If you received any errors or made any changes to the Terraform config or `terraform.tfvars` you must re-run `./tf-wrapper.sh plan production` before run `./tf-wrapper.sh apply production`.
