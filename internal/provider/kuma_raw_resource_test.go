// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccExampleResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: localProviderConfig + testAccExampleResourceConfig(`
type: MeshTrafficPermission
name: test-1
mesh: default
spec:
  targetRef:
    kind: Mesh
  from:
  - targetRef:
      kind: Mesh
    default:
      action: Allow
  - targetRef:
      kind: MeshService
      name: foo
    default:
      action: Deny
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kuma_raw_resource.test", "name", "test-1"),
					resource.TestCheckResourceAttr("kuma_raw_resource.test", "mesh", "default"),
					resource.TestCheckResourceAttr("kuma_raw_resource.test", "type", "MeshTrafficPermission"),
				),
			},
			// ImportState testing
			{
				ResourceName:                         "kuma_raw_resource.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateId:                        "default/MeshTrafficPermission/test-1",
				ImportStateVerifyIdentifierAttribute: "name",
			},
			// Update and Read testing
			{
				Config: localProviderConfig + testAccExampleResourceConfig(`
type: MeshTrafficPermission
name: test-1
mesh: default
spec:
  targetRef:
    kind: Mesh
  from:
  - targetRef:
      kind: Mesh
    default:
      action: Allow
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("kuma_raw_resource.test", "raw_json", `{"mesh":"default","name":"test-1","spec":{"from":[{"default":{"action":"Allow"},"targetRef":{"kind":"Mesh"}}],"targetRef":{"kind":"Mesh"}},"type":"MeshTrafficPermission"}`),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccExampleResourceConfig(json string) string {
	return fmt.Sprintf(`
resource "kuma_raw_resource" "test" {
  raw_json = jsonencode(yamldecode(<<YAML
%s
YAML
))
}
`, json)
}
