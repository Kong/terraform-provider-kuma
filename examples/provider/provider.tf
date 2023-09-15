terraform {
  required_providers {
    kuma = {
      source = "registry.terraform.io/lahabana/kuma"
    }
  }
}

provider "kuma" {
  # example configuration here
  endpoint = "http://localhost:5681"
}

resource "kuma_resource" "example" {
    json_body = jsonencode(yamldecode(<<YAML
type: MeshTrafficPermission
name: foo
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
YAML
))
}
