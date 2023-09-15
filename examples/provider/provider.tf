terraform {
  required_providers {
    kuma = {
      source = "registry.terraform.io/lahabana/kuma"
    }
  }
}

variable "kuma_token" {
  type = string
}

provider "kuma" {
  # example configuration here
  # endpoint = "http://localhost:5681"
  endpoint = "https://us.api.konghq.com/v0/mesh/control-planes/5637a856-a9b1-40db-b046-d5b16cccb1e2/api"
  token = var.kuma_token
}

resource "kuma_raw_resource" "example" {
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
      action: Deny
  - targetRef:
      kind: MeshService
      name: foo
    default:
      action: Deny
YAML
))
}

resource "kuma_raw_resource" "other_example" {
  json_body = jsonencode(yamldecode(<<YAML
type: MeshTrafficPermission
name: bar
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

resource "kuma_raw_resource" "tracing" {
  json_body = jsonencode({
    type="TrafficTrace"
    mesh="default"
    name="trace-all-traffic"
    selectors=[{
      match={"kuma.io/service": "*"}
    }]
    conf={
        backend="jaeger-collector"
    }
})
}
