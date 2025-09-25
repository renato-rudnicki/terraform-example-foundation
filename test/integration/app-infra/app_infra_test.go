// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package appinfra

import (
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/cloud-foundation-toolkit/infra/blueprint-test/pkg/gcloud"
	"github.com/GoogleCloudPlatform/cloud-foundation-toolkit/infra/blueprint-test/pkg/tft"
	"github.com/GoogleCloudPlatform/cloud-foundation-toolkit/infra/blueprint-test/pkg/utils"
	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/stretchr/testify/assert"
)

func TestAppInfra(t *testing.T) {

	bootstrap := tft.NewTFBlueprintTest(t,
		tft.WithTFDir("../../../0-bootstrap"),
	)
	projects_backend_bucket := bootstrap.GetStringOutput("projects_gcs_bucket_tfstate")
	vars := map[string]interface{}{
		"remote_state_bucket": projects_backend_bucket,
	}

	shared := tft.NewTFBlueprintTest(t,
		tft.WithTFDir("../../../4-projects/business_unit_1/shared"),
	)

	// Configure impersonation for test execution
	terraformSA := terraform.OutputMap(t, shared.GetTFOptions(), "terraform_service_accounts")["bu1-example-app"]
	backend_bucket := terraform.OutputMap(t, shared.GetTFOptions(), "state_buckets")["bu1-example-app"]
	utils.SetEnv(t, "GOOGLE_IMPERSONATE_SERVICE_ACCOUNT", terraformSA)
	backendConfig := map[string]interface{}{
		"bucket": backend_bucket,
	}

	for _, envName := range []string{
		"development",
		"nonproduction",
		"production",
	} {
		envName := envName
		t.Run(envName, func(t *testing.T) {
			t.Parallel()

			projects := tft.NewTFBlueprintTest(t,
				tft.WithTFDir(fmt.Sprintf("../../../4-projects/business_unit_1/%s", envName)),
			)

			appInfra := tft.NewTFBlueprintTest(t,
				tft.WithTFDir(fmt.Sprintf("../../../5-app-infra/business_unit_1/%s", envName)),
				tft.WithBackendConfig(backendConfig),
				tft.WithPolicyLibraryPath("/workspace/policy-library", projects.GetStringOutput("shared_vpc_project")),
				tft.WithVars(vars),
			)

			appInfra.DefineVerify(
				func(assert *assert.Assertions) {
					projectID := appInfra.GetStringOutput("project_id")
					workloadPoolProvider := appInfra.GetStringOutput("workload_pool_provider_id")
					workloadIdentityPool := appInfra.GetStringOutput("workload_identity_pool_id")
					instanceName := terraform.OutputList(t, appInfra.GetTFOptions(), "instances_names")[0]
					instanceZone := terraform.OutputList(t, appInfra.GetTFOptions(), "instances_zones")[0]
					machineType := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s/machineTypes/f1-micro", projectID, instanceZone)

					gcOps := gcloud.WithCommonArgs([]string{"--project", projectID, "--zone", instanceZone, "--format", "json"})
					instance := gcloud.Run(t, fmt.Sprintf("compute instances describe %s", instanceName), gcOps)
					assert.Equal(machineType, instance.Get("machineType").String(), "should have machine_type f1-micro")

					confidentialProjectID := appInfra.GetStringOutput("confidential_space_project_id")
					confidentialInstanceName := terraform.OutputList(t, appInfra.GetTFOptions(), "confidential_instances_names")[0]
					confidentialInstanceZone := terraform.OutputList(t, appInfra.GetTFOptions(), "confidential_instances_zones")[0]
					confidentialProjectNumber := appInfra.GetStringOutput("confidential_space_project_number")

					gcPoolOps := gcloud.WithCommonArgs([]string{"--project", confidentialProjectID, "--format", "json"})
					poolDetails := gcloud.Run(t, fmt.Sprintf("iam workload-identity-pools describe %s --location=global", workloadIdentityPool), gcPoolOps)
					name := poolDetails.Get("name").String()
					expectedName := fmt.Sprintf("projects/%s/locations/global/workloadIdentityPools/%s", confidentialProjectNumber, workloadIdentityPool)
					assert.Equal(expectedName, name, "Workload Identity Pool full name should match")

					gcPoolProviderOps := gcloud.WithCommonArgs([]string{fmt.Sprintf("--workload-identity-pool=%s", workloadIdentityPool), "--location=global", "--project", confidentialProjectID, "--format", "json"})
					workloadIdentityPoolProviderID := gcloud.Run(t, fmt.Sprintf("iam workload-identity-pools providers describe %s", workloadPoolProvider), gcPoolProviderOps)
					assert.Equal(workloadPoolProvider, workloadIdentityPoolProviderID.Get("displayName").String(), fmt.Sprintf("workload identity pool provider should have name equals to %s", workloadPoolProvider))

					gcInstanceOps := gcloud.WithCommonArgs([]string{"--project", confidentialProjectID, "--zone", confidentialInstanceZone, "--format", "json"})
					computeInstanceList := gcloud.Run(t, fmt.Sprintf("compute instances describe %s", confidentialInstanceName), gcInstanceOps)
					assert.NotEmpty(computeInstanceList, "Expected instance details to be present")
					computeInstance := computeInstanceList
					assert.Equal(confidentialInstanceName, computeInstance.Get("name").String(), "Confidential instance name must match expected")
					confidentialInstanceConfig := computeInstance.Get("confidentialInstanceConfig")
					assert.True(confidentialInstanceConfig.Get("enableConfidentialCompute").Bool(), "Confidential Compute should be enabled")
					assert.Equal("SEV", confidentialInstanceConfig.Get("confidentialInstanceType").String())
					assert.Equal("MIGRATE", computeInstance.Get("scheduling").Get("onHostMaintenance").String())
					serviceAccounts := computeInstance.Get("serviceAccounts").Array()
					assert.Len(serviceAccounts, 1)
					assert.Equal(fmt.Sprintf("confidential-space-workload-sa@%s.iam.gserviceaccount.com", confidentialProjectID), serviceAccounts[0].Get("email").String())
				})

			appInfra.Test()
		})

	}
}
