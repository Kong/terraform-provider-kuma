terraform {
  required_providers {
    kuma = {
      source = "registry.terraform.io/kong/kuma"
    }
  }
}

variable "kuma_token" {
  type = string
}

provider "kuma" {
  # example configuration here
  endpoint = "http://localhost:5681"
  # For Konnect you could use something like
  # endpoint = "https://us.api.konghq.com/v0/mesh/control-planes/<cpId>/api"
  # Set the variable using `TF_VAR_kuma_token`
  # token    = var.kuma_token
}

resource "kuma_raw_resource" "example" {
  raw_json = jsonencode(yamldecode(<<YAML
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
  raw_json = jsonencode(yamldecode(<<YAML
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
  raw_json = jsonencode({
    type = "TrafficTrace"
    mesh = "default"
    name = "trace-all-traffic"
    selectors = [{
      match = { "kuma.io/service" : "*" }
    }]
    conf = {
      backend = "jaeger-collector"
    }
  })
}
