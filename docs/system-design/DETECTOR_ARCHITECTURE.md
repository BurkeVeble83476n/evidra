# Evidra — Adapter & Detector Architecture Redesign

**Status:** Architecture decision — implement before v1.0
**Date:** March 2026
**Motivation:** Enable community contribution, cover AWS/GCP/Azure (80% of market), scale to 100+ detectors without monolith

---

## 1. Current Architecture (what's wrong)

```
internal/risk/detectors.go      ← ALL detectors in ONE file (300+ lines)
internal/canon/k8s.go           ← ALL K8s canonicalization in one file
internal/canon/terraform.go     ← ALL Terraform canonicalization in one file
```

Problems:

**Detectors are hardcoded.** Adding one detector = edit `DefaultDetectors()`, add struct to `detectors.go`, rebuild. Community contributor must understand the whole file.

**No provider separation.** AWS S3 detector sits next to K8s privileged detector sits next to Terraform IAM detector. At 7 detectors this is fine. At 50 it's chaos.

**Terraform adapter treats all providers the same.** AWS, GCP, Azure terraform plans all go through one code path. But detection logic is completely different per provider: `aws_s3_bucket` needs public access block check, `google_storage_bucket` needs uniform_bucket_level_access check, `azurerm_storage_account` needs network_rules check. Same concept (public storage), three different detection implementations.

**No way to add "just an AWS RDS detector."** You must: understand the detector interface, find the right place in detectors.go, add to DefaultDetectors(), write test in the same file. Barrier too high for community.

---

## 2. Target Architecture

```
internal/
  canon/                              # Adapters — STABLE, rarely change
    adapter.go                        # Interface + registry
    k8s.go                            # K8s YAML → CanonicalAction
    terraform.go                      # TF plan JSON → CanonicalAction
    docker.go                         # docker-compose YAML → CanonicalAction
    generic.go                        # Fallback
    
  risk/                               # Risk engine — orchestration
    engine.go                         # RunAll, registry, severity lookup
    matrix.go                         # Risk level from operation × scope
    
  detectors/                          # Detectors — ONE FILE PER PATTERN
    registry.go                       # Global registry, Register() function
    
    k8s/                              # Kubernetes YAML detectors
      privileged.go                   # k8s.privileged_container
      hostpath.go                     # k8s.hostpath_mount
      host_namespace.go               # k8s.host_namespace_escape
      docker_socket.go                # k8s.docker_socket
      run_as_root.go                  # k8s.run_as_root
      capabilities.go                 # k8s.dangerous_capabilities
      cluster_admin.go                # k8s.cluster_admin_binding
      writable_rootfs.go              # k8s.writable_rootfs
      helpers.go                      # Shared K8s YAML parsing
      
    terraform/                        # Terraform plan detectors (by provider)
      helpers.go                      # Shared TF plan parsing
      aws/                            # AWS over Terraform
        iam_wildcard.go               # aws.iam_wildcard_policy
        s3_public.go                  # aws.s3_public_access
        security_group.go             # aws.security_group_open
        rds_public.go                 # aws.rds_public
        ebs_unencrypted.go            # aws.ebs_unencrypted
      gcp/                            # GCP over Terraform
        iam_wildcard.go               # gcp.iam_wildcard
        storage_public.go             # gcp.storage_public
        compute_default_sa.go         # gcp.compute_default_service_account
      azure/                          # Azure over Terraform
        nsg_open.go                   # azure.nsg_open
        storage_public.go             # azure.storage_public
        sql_public.go                 # azure.sql_public
      
    docker/                           # Docker/Compose detectors
      privileged.go                   # docker.privileged
      socket_mount.go                 # docker.socket_mount
      host_network.go                 # docker.host_network
      
    ops/                              # Operational detectors (tool-agnostic)
      mass_delete.go                  # ops.mass_delete
      namespace_delete.go             # ops.namespace_delete
      kube_system.go                  # ops.kube_system
```

**One detector = one file.** Community contributor adds ONE Go file to the right directory. That's it.

---

## 3. Detector Registry (self-registering)

```go
// internal/detectors/registry.go

package detectors

import (
    "samebits.com/evidra-benchmark/internal/canon"
    "sync"
)

// Stability indicates the maturity of a detector.
type Stability string

const (
    Stable       Stability = "stable"       // Tag semantics frozen, safe for benchmarks
    Experimental Stability = "experimental" // May change in next release
    Deprecated   Stability = "deprecated"   // Will be removed, use replacement
)

// VocabularyLevel classifies the type of risk event a detector produces.
type VocabularyLevel string

const (
    ResourceRisk  VocabularyLevel = "resource"  // Misconfig in artifact: aws.s3_public_access
    OperationRisk VocabularyLevel = "operation" // Risky operational pattern: ops.mass_delete
    // BehaviorSignal is NOT a detector output — it comes from the signals engine.
    // Listed here for documentation: retry_loop, artifact_drift, protocol_violation
)

// Detector inspects an operation for risk patterns.
type Detector interface {
    // Tag returns the canonical risk tag this detector produces.
    // Must be dot-namespace format: "k8s.privileged_container"
    Tag() string

    // BaseSeverity returns the severity when this detector fires, before
    // context elevation. Final severity is computed by the risk matrix
    // using base severity + operation_class + scope_class.
    BaseSeverity() string // "low", "medium", "high", "critical"

    // Detect returns true if the risk pattern is present.
    Detect(action canon.CanonicalAction, raw []byte) bool

    // Metadata returns full detector metadata for registry export.
    Metadata() TagMetadata
}

// TagMetadata describes a registered detector for export, documentation, and LLM prompt generation.
type TagMetadata struct {
    Tag           string          `json:"tag" yaml:"tag"`
    BaseSeverity  string          `json:"base_severity" yaml:"base_severity"`
    Stability     Stability       `json:"stability" yaml:"stability"`
    Level         VocabularyLevel `json:"level" yaml:"level"`
    Domain        string          `json:"domain" yaml:"domain"`         // k8s, aws, gcp, azure, docker, ops
    SourceKind    string          `json:"source_kind" yaml:"source_kind"` // terraform_plan, k8s_yaml, compose_yaml, any
    Summary       string          `json:"summary" yaml:"summary"`
}

var (
    mu       sync.RWMutex
    registry []Detector
)

// Register adds a detector to the global registry.
// Called from init() in each detector file.
func Register(d Detector) {
    mu.Lock()
    defer mu.Unlock()
    registry = append(registry, d)
}

// All returns all registered detectors.
func All() []Detector {
    mu.RLock()
    defer mu.RUnlock()
    return append([]Detector{}, registry...)
}

// RunAll runs all registered detectors against an operation.
// Returns list of tags that fired.
func RunAll(action canon.CanonicalAction, raw []byte) []string {
    var tags []string
    for _, d := range All() {
        if d.Detect(action, raw) {
            tags = append(tags, d.Tag())
        }
    }
    return tags
}

// MaxBaseSeverity returns the highest base severity among fired tags.
// Note: this is BASE severity — final severity requires risk matrix elevation.
func MaxBaseSeverity(tags []string) string {
    order := map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}
    max := "low"
    for _, d := range All() {
        for _, tag := range tags {
            if d.Tag() == tag {
                if order[d.BaseSeverity()] > order[max] {
                    max = d.BaseSeverity()
                }
            }
        }
    }
    return max
}

// AllMetadata returns metadata for all registered detectors (for export).
func AllMetadata() []TagMetadata {
    var info []TagMetadata
    for _, d := range All() {
        info = append(info, d.Metadata())
    }
    return info
}

// StableOnly returns metadata for stable detectors only (for benchmarks).
func StableOnly() []TagMetadata {
    var info []TagMetadata
    for _, d := range All() {
        m := d.Metadata()
        if m.Stability == Stable {
            info = append(info, m)
        }
    }
    return info
}
```

### Three Vocabulary Levels (architecture decision)

Evidra produces three distinct types of risk events. They must not be confused:

| Level | Source | Examples | Lifecycle |
|-------|--------|---------|-----------|
| **Resource risk** | Detectors | `aws.s3_public_access`, `k8s.privileged_container` | Fires at prescribe time on artifact content |
| **Operation risk** | Detectors | `ops.mass_delete`, `ops.kube_system` | Fires at prescribe time on canonical action |
| **Behavior signal** | Signal engine | `retry_loop`, `protocol_violation`, `artifact_drift` | Fires at scorecard time from evidence chain patterns |

Detectors produce resource and operation risk tags. The **signals engine** produces behavior signals. They are different layers:

```
artifact → adapter → canonical action → DETECTORS → risk tags (resource + operation)
                                              ↓
evidence chain (prescribe/report sequence) → SIGNALS ENGINE → behavior signals
                                              ↓
risk tags + behavior signals → SCORECARD → reliability score
```

Detectors create the **typed event vocabulary** that the signals engine interprets. `k8s.privileged_container` is not just a finding — it's a typed evidence event that feeds retry_loop detection ("agent prescribed privileged container 3 times") and blast_radius estimation.

### Base Severity vs Final Severity

`BaseSeverity()` is a property of the pattern. `docker.socket_mount` is always "critical" as a base — mounting the Docker socket is inherently dangerous.

Final severity is context-dependent. Same `docker.socket_mount` in a local dev compose file → might stay "critical". In a production agent runtime → stays "critical". But the risk matrix can modulate based on `scope_class`:

```
final_severity = risk_matrix(base_severity, operation_class, scope_class)
```

Detectors define base severity. Policy defines final severity. This is already how `internal/risk/matrix.go` works — the redesign just makes the naming explicit.
```

---

## 4. Example Detector (one file, complete)

```go
// internal/detectors/terraform/aws/s3_public.go

package aws

import (
    "samebits.com/evidra-benchmark/internal/canon"
    "samebits.com/evidra-benchmark/internal/detectors"
    "samebits.com/evidra-benchmark/internal/detectors/terraform"
)

func init() {
    detectors.Register(&S3PublicAccess{})
}

// S3PublicAccess detects S3 buckets without complete public access blocks.
type S3PublicAccess struct{}

func (d *S3PublicAccess) Tag() string          { return "aws.s3_public_access" }
func (d *S3PublicAccess) BaseSeverity() string { return "high" }

func (d *S3PublicAccess) Metadata() detectors.TagMetadata {
    return detectors.TagMetadata{
        Tag:          "aws.s3_public_access",
        BaseSeverity: "high",
        Stability:    detectors.Stable,
        Level:        detectors.ResourceRisk,
        Domain:       "aws",
        SourceKind:   "terraform_plan",
        Summary:      "S3 bucket without complete public access block",
    }
}

func (d *S3PublicAccess) Detect(action canon.CanonicalAction, raw []byte) bool {
    plan := terraform.ParsePlan(raw)
    if plan == nil {
        return false
    }

    if !terraform.HasResource(plan, "aws_s3_bucket") {
        return false
    }

    for _, rc := range terraform.ResourcesByType(plan, "aws_s3_bucket_public_access_block") {
        if rc.Change != nil && isCompletePublicAccessBlock(rc.Change.After) {
            return false
        }
    }

    return true
}
```

**That's the entire file.** `init()` self-registers. No edits to registry.go. No edits to DefaultDetectors(). Import the package and it works.

---

## 5. Auto-Import via Package Convention

Go's `init()` only runs if the package is imported. Use a single import file:

```go
// internal/detectors/all/all.go
// Import this package to register all built-in detectors.

package all

import (
    _ "samebits.com/evidra-benchmark/internal/detectors/docker"
    _ "samebits.com/evidra-benchmark/internal/detectors/k8s"
    _ "samebits.com/evidra-benchmark/internal/detectors/ops"
    _ "samebits.com/evidra-benchmark/internal/detectors/terraform/aws"
    _ "samebits.com/evidra-benchmark/internal/detectors/terraform/azure"
    _ "samebits.com/evidra-benchmark/internal/detectors/terraform/gcp"
)
```

In `cmd/evidra/main.go`:

```go
import (
    _ "samebits.com/evidra-benchmark/internal/detectors/all"
)
```

Community adds a new provider directory (e.g., `internal/detectors/oci/`), adds one import line to `all.go`, done. Everything else automatic.

---

## 6. Shared Helpers Per Domain

Each provider directory has a `helpers.go` with shared parsing:

### K8s Helpers (already exist, just move)

```go
// internal/detectors/k8s/helpers.go
package k8s

// parseK8sYAML, getPodSpec, getAllContainers, getBool
// — moved from internal/risk/detectors.go
```

### Terraform Helpers (shared by all cloud providers)

Already defined in section 8: `internal/detectors/terraform/helpers.go`. AWS, GCP, Azure detectors all import the same `terraform` package.

### Short Example: AWS RDS (15 lines)

```go
// internal/detectors/terraform/aws/rds_public.go
package aws

import (
    "samebits.com/evidra-benchmark/internal/canon"
    "samebits.com/evidra-benchmark/internal/detectors"
    "samebits.com/evidra-benchmark/internal/detectors/terraform"
)

func init() { detectors.Register(&RDSPublic{}) }

type RDSPublic struct{}

func (d *RDSPublic) Tag() string          { return "aws.rds_public" }
func (d *RDSPublic) BaseSeverity() string { return "high" }
func (d *RDSPublic) Metadata() detectors.TagMetadata {
    return detectors.TagMetadata{
        Tag: "aws.rds_public", BaseSeverity: "high", Stability: detectors.Stable,
        Level: detectors.ResourceRisk, Domain: "aws", SourceKind: "terraform_plan",
        Summary: "RDS instance with publicly_accessible=true",
    }
}

func (d *RDSPublic) Detect(_ canon.CanonicalAction, raw []byte) bool {
    plan := terraform.ParsePlan(raw)
    if plan == nil {
        return false
    }
    for _, rc := range terraform.ResourcesByType(plan, "aws_db_instance") {
        if terraform.AfterBool(rc, "publicly_accessible") {
            return true
        }
    }
    return false
}
```

Community contributor needs to know: Terraform resource type, field to check, tag to emit.

---

## 7. GCP & Azure Detectors (same pattern)

```go
// internal/detectors/terraform/gcp/storage_public.go
package gcp

import (
    "samebits.com/evidra-benchmark/internal/canon"
    "samebits.com/evidra-benchmark/internal/detectors"
    "samebits.com/evidra-benchmark/internal/detectors/terraform"
)

func init() { detectors.Register(&StoragePublic{}) }

type StoragePublic struct{}

func (d *StoragePublic) Tag() string          { return "gcp.storage_public" }
func (d *StoragePublic) BaseSeverity() string { return "high" }
func (d *StoragePublic) Metadata() detectors.TagMetadata {
    return detectors.TagMetadata{
        Tag: "gcp.storage_public", BaseSeverity: "high", Stability: detectors.Stable,
        Level: detectors.ResourceRisk, Domain: "gcp", SourceKind: "terraform_plan",
        Summary: "GCS bucket without uniform_bucket_level_access",
    }
}

func (d *StoragePublic) Detect(_ canon.CanonicalAction, raw []byte) bool {
    plan := terraform.ParsePlan(raw)
    if plan == nil {
        return false
    }
    for _, rc := range terraform.ResourcesByType(plan, "google_storage_bucket") {
        if !terraform.AfterBool(rc, "uniform_bucket_level_access") {
            return true
        }
    }
    return false
}
```

```go
// internal/detectors/terraform/azure/nsg_open.go
package azure

import (
    "samebits.com/evidra-benchmark/internal/canon"
    "samebits.com/evidra-benchmark/internal/detectors"
    "samebits.com/evidra-benchmark/internal/detectors/terraform"
)

func init() { detectors.Register(&NSGOpen{}) }

type NSGOpen struct{}

func (d *NSGOpen) Tag() string          { return "azure.nsg_open" }
func (d *NSGOpen) BaseSeverity() string { return "high" }
func (d *NSGOpen) Metadata() detectors.TagMetadata {
    return detectors.TagMetadata{
        Tag: "azure.nsg_open", BaseSeverity: "high", Stability: detectors.Stable,
        Level: detectors.ResourceRisk, Domain: "azure", SourceKind: "terraform_plan",
        Summary: "NSG rule with source_address_prefix=* on sensitive ports",
    }
}

func (d *NSGOpen) Detect(_ canon.CanonicalAction, raw []byte) bool {
    plan := terraform.ParsePlan(raw)
    if plan == nil {
        return false
    }
    for _, rc := range terraform.ResourcesByType(plan, "azurerm_network_security_rule") {
        cidr, ok := terraform.AfterValue(rc, "source_address_prefix")
        if ok && cidr == "*" {
            return true
        }
    }
    return false
}
```

Every cloud detector: same shape, same imports, same pattern. ~30 lines with full metadata.

---

## 8. Shared TF Helpers (one package, all providers share)

```go
// internal/detectors/terraform/helpers.go
package terraform

import (
    "encoding/json"
    tfjson "github.com/hashicorp/terraform-json"
)

func ParsePlan(raw []byte) *tfjson.Plan { ... }
func ResourcesByType(plan *tfjson.Plan, t string) []*tfjson.ResourceChange { ... }
func HasResource(plan *tfjson.Plan, t string) bool { ... }
func AfterValue(rc *tfjson.ResourceChange, key string) (interface{}, bool) { ... }
func AfterBool(rc *tfjson.ResourceChange, key string) bool { ... }
func AfterString(rc *tfjson.ResourceChange, key string) string { ... }
```

AWS, GCP, Azure detector packages all import `terraform` helpers. K8s detectors import `k8s/helpers`. Docker detectors have their own `docker/helpers`. No duplication.

This nesting (`terraform/aws/`, `terraform/gcp/`) is intentional: when CloudFormation or Pulumi support arrives, the structure becomes `cloudformation/aws/` and `pulumi/aws/` — same provider domain, different evidence source, no conflict.

---

## 9. Migration Plan (from current to new)

### Step 1: Create directory structure

```bash
mkdir -p internal/detectors/{k8s,terraform/{aws,gcp,azure},docker,ops,all}
```

### Step 2: Move K8s helpers

```
internal/risk/detectors.go → internal/detectors/k8s/helpers.go
  parseK8sYAML()
  getPodSpec()
  getAllContainers()
  getBool()
```

### Step 3: Split detectors into individual files

```
DetectPrivileged     → internal/detectors/k8s/privileged.go
DetectHostNamespace  → internal/detectors/k8s/host_namespace.go
DetectHostPath       → internal/detectors/k8s/hostpath.go
DetectMassDestroy    → internal/detectors/ops/mass_delete.go
DetectWildcardIAM    → internal/detectors/terraform/aws/iam_wildcard.go
DetectTerraformIAMWildcard → internal/detectors/terraform/aws/iam_wildcard_any.go
DetectS3PublicAccess → internal/detectors/terraform/aws/s3_public.go
```

### Step 4: Create terraform/helpers.go from existing TF parsing code

```
parsePlan()                    → internal/detectors/terraform/helpers.go
extractIAMStatements()         → internal/detectors/terraform/aws/iam_helpers.go
isCompletePublicAccessBlock()  → internal/detectors/terraform/aws/s3_helpers.go
```

### Step 5: Update imports

```go
// internal/risk/engine.go (replaces old RunAll)
package risk

import "samebits.com/evidra-benchmark/internal/detectors"

func RunAll(action canon.CanonicalAction, raw []byte) []string {
    return detectors.RunAll(action, raw)
}
```

### Step 6: Create all.go import file

### Step 7: Delete old `internal/risk/detectors.go`

### Step 8: Run all tests — everything must pass

**Zero behavioral change.** Same tags, same severity, same interface. Only internal organization changes.

---

## 10. Community Contribution Flow

### Adding a Detector (contributor guide)

```markdown
# How to Add a Detector

1. Pick the right directory:
   - K8s YAML patterns → `internal/detectors/k8s/`
   - AWS Terraform patterns → `internal/detectors/terraform/aws/`
   - GCP Terraform patterns → `internal/detectors/terraform/gcp/`
   - Azure Terraform patterns → `internal/detectors/terraform/azure/`
   - Docker/Compose → `internal/detectors/docker/`
   - Operational (tool-agnostic) → `internal/detectors/ops/`

2. Create one file: `{directory}/{pattern}.go`

3. Template:

   package {provider}
   
   import (
       "samebits.com/evidra-benchmark/internal/canon"
       "samebits.com/evidra-benchmark/internal/detectors"
       // for Terraform detectors, also:
       "samebits.com/evidra-benchmark/internal/detectors/terraform"
   )
   
   func init() { detectors.Register(&YourDetector{}) }
   
   type YourDetector struct{}
   
   func (d *YourDetector) Tag() string          { return "{domain}.{pattern}" }
   func (d *YourDetector) BaseSeverity() string { return "high" }
   func (d *YourDetector) Metadata() detectors.TagMetadata {
       return detectors.TagMetadata{
           Tag: "{domain}.{pattern}", BaseSeverity: "high",
           Stability: detectors.Experimental,  // new detectors start as experimental
           Level: detectors.ResourceRisk, Domain: "{domain}",
           SourceKind: "terraform_plan",  // or "k8s_yaml", "compose_yaml"
           Summary: "Short description of what this detects",
       }
   }
   func (d *YourDetector) Detect(action canon.CanonicalAction, raw []byte) bool {
       // Your detection logic here
       return false
   }

4. Add test: `{directory}/{pattern}_test.go` (positive + negative fixture)

5. Add import to `internal/detectors/all/all.go` (only if new provider directory)

6. Run: `make test` (contract_test.go validates metadata automatically)

7. Open PR
```

**Barrier to contribute: one Go file + one test file.** New detectors start as `Experimental` stability. Promoted to `Stable` after validation on benchmark dataset.

---

## 11. Cloud Provider Coverage Roadmap

### AWS (biggest market share)

| Detector | Resource Type | Field Check | Severity | Effort |
|----------|-------------|-------------|----------|--------|
| `aws.iam_wildcard_policy` | aws_iam_policy | Action:* AND Resource:* | critical | Done |
| `aws.s3_public_access` | aws_s3_bucket | missing public access block | high | Done |
| `aws.security_group_open` | aws_security_group | ingress 0.0.0.0/0 on 22,3389,3306,5432 | high | 30 min |
| `aws.rds_public` | aws_db_instance | publicly_accessible=true | high | 15 min |
| `aws.ebs_unencrypted` | aws_ebs_volume | encrypted=false/absent | medium | 15 min |
| `aws.cloudtrail_disabled` | aws_cloudtrail | is_multi_region_trail=false | medium | 15 min |
| `aws.s3_no_encryption` | aws_s3_bucket | no server_side_encryption | medium | 20 min |
| `aws.elasticache_unencrypted` | aws_elasticache | transit_encryption_enabled=false | medium | 15 min |
| `aws.lambda_public` | aws_lambda_permission | principal=* | high | 20 min |
| `aws.eks_public_endpoint` | aws_eks_cluster | endpoint_public_access=true | high | 15 min |

10 AWS detectors, ~3 hours total. With `terraform` helpers, each is 15-20 lines.

### GCP (second largest)

| Detector | Resource Type | Field Check | Severity |
|----------|-------------|-------------|----------|
| `gcp.storage_public` | google_storage_bucket | uniform_bucket_level_access | high |
| `gcp.iam_wildcard` | google_project_iam_binding | members contains allUsers | critical |
| `gcp.compute_default_sa` | google_compute_instance | default service account | medium |
| `gcp.sql_public` | google_sql_database_instance | public IP enabled | high |
| `gcp.gke_legacy_auth` | google_container_cluster | legacy ABAC enabled | high |

5 GCP detectors, ~2 hours.

### Azure (third largest)

| Detector | Resource Type | Field Check | Severity |
|----------|-------------|-------------|----------|
| `azure.nsg_open` | azurerm_network_security_rule | source * on sensitive ports | high |
| `azure.storage_public` | azurerm_storage_account | allow_blob_public_access | high |
| `azure.sql_public` | azurerm_sql_server | public_network_access_enabled | high |
| `azure.aks_rbac_disabled` | azurerm_kubernetes_cluster | role_based_access_control absent | high |
| `azure.disk_unencrypted` | azurerm_managed_disk | encryption absent | medium |

5 Azure detectors, ~2 hours.

### Total: 20 new cloud detectors in ~1 week

7 existing + 13 from PARALLEL_EXECUTION_PLAN + 20 cloud = **40 detectors** at launch.

Coverage:
- K8s workloads: 8 detectors
- AWS: 12 detectors (including 2 existing)
- GCP: 5 detectors
- Azure: 5 detectors
- Docker: 3 detectors
- Ops: 3 detectors
- Terraform (cross-provider): legacy IAM detector
- **Total: 37-40 detectors**

This is serious coverage. Not Checkov-level (800+ rules) but meaningful for v1.0. And the architecture makes it trivial to add more.

---

## 12. Adapter Changes (minimal)

Adapters barely change. The key insight: **Terraform adapter already parses all providers.** `terraform show -json` produces the same plan format for AWS, GCP, and Azure. The provider-specific logic lives in detectors, not adapters.

Only one adapter addition needed: Docker.

```
Adapters (v1.0):
  K8sAdapter         — handles kubectl, oc, helm  (existing)
  TerraformAdapter   — handles terraform           (existing, covers all providers)
  DockerAdapter      — handles docker, compose      (new)
  GenericAdapter     — handles everything else       (existing)
```

No changes to K8s or Terraform adapters. They work with the new detector architecture without modification.

---

## 13. Interface Changes (Detector)

Old interface:
```go
type Detector interface {
    Name() string
    Detect(action canon.CanonicalAction, rawArtifact []byte) []string
}
```

New interface:
```go
type Detector interface {
    Tag() string                                            // canonical risk tag
    BaseSeverity() string                                   // base severity (before matrix)
    Detect(action canon.CanonicalAction, raw []byte) bool   // fires or not
    Metadata() TagMetadata                                  // full metadata for export
}
```

Changes:
- `Name()` → `Tag()`: returns full canonical tag, not detector name
- `Detect` returns `bool` not `[]string`: one detector = one tag
- `Severity()` → `BaseSeverity()`: explicit that this is base, not final
- `Metadata()` added: carries domain, source_kind, stability, summary for registry export

**Why `bool` not `[]string`:** A detector that returns multiple tags is doing two jobs. Split it. One-file-one-pattern, enforced by interface.

---

## 14. Detector Test Contract

Every detector must pass a standard test contract:

```go
// internal/detectors/contract_test.go

func TestDetectorContract(t *testing.T) {
    for _, d := range detectors.All() {
        t.Run(d.Tag(), func(t *testing.T) {
            m := d.Metadata()

            // Tag format: namespace.pattern, lowercase, no spaces
            if !regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`).MatchString(m.Tag) {
                t.Errorf("invalid tag format: %q", m.Tag)
            }

            // BaseSeverity must be valid
            validSeverity := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
            if !validSeverity[m.BaseSeverity] {
                t.Errorf("invalid base severity: %q", m.BaseSeverity)
            }

            // Stability must be set
            if m.Stability != detectors.Stable && m.Stability != detectors.Experimental && m.Stability != detectors.Deprecated {
                t.Errorf("invalid stability: %q", m.Stability)
            }

            // Level must be set
            if m.Level != detectors.ResourceRisk && m.Level != detectors.OperationRisk {
                t.Errorf("invalid vocabulary level: %q", m.Level)
            }

            // Summary must be non-empty
            if m.Summary == "" {
                t.Error("summary must not be empty")
            }

            // Tag and metadata must agree
            if d.Tag() != m.Tag {
                t.Errorf("Tag() = %q but Metadata().Tag = %q", d.Tag(), m.Tag)
            }
            if d.BaseSeverity() != m.BaseSeverity {
                t.Errorf("BaseSeverity() = %q but Metadata().BaseSeverity = %q", d.BaseSeverity(), m.BaseSeverity)
            }

            // Idempotent: same input → same result
            dummyAction := canon.CanonicalAction{}
            dummyArtifact := []byte("{}")
            r1 := d.Detect(dummyAction, dummyArtifact)
            r2 := d.Detect(dummyAction, dummyArtifact)
            if r1 != r2 {
                t.Error("Detect is not idempotent on empty input")
            }
        })
    }
}
```

This runs automatically for every registered detector. Adding a detector with broken metadata or non-idempotent behavior fails CI instantly. No manual review needed for structural correctness.

Each detector file also has its own test with positive and negative fixtures:

```go
// internal/detectors/terraform/aws/s3_public_test.go

func TestS3PublicAccess_Fires(t *testing.T) {
    d := &S3PublicAccess{}
    raw := loadFixture("testdata/s3_no_public_block.json")
    if !d.Detect(canon.CanonicalAction{}, raw) {
        t.Error("should detect S3 without public access block")
    }
}

func TestS3PublicAccess_DoesNotFire(t *testing.T) {
    d := &S3PublicAccess{}
    raw := loadFixture("testdata/s3_with_public_block.json")
    if d.Detect(canon.CanonicalAction{}, raw) {
        t.Error("should not fire when public access block is complete")
    }
}
```

---

## 15. Tag Registry Is Auto-Generated from Code

The Go code IS the registry. YAML/JSON is generated for external consumption:

```bash
# Generate tag registry from registered detectors
evidra detectors list --format yaml > tag-registry.yaml
evidra detectors list --format json > tag-registry.json
evidra detectors list --stable-only --format prompt > llm-known-tags.txt
```

```go
// cmd/evidra/detectors_list.go
func listDetectors(format string, stableOnly bool) {
    metas := detectors.AllMetadata()
    if stableOnly {
        metas = detectors.StableOnly()
    }
    switch format {
    case "yaml":
        yaml.NewEncoder(os.Stdout).Encode(metas)
    case "json":
        json.NewEncoder(os.Stdout).Encode(metas)
    case "prompt":
        // LLM-friendly format for system prompt
        for _, m := range metas {
            fmt.Printf("  %s — %s\n", m.Tag, m.Summary)
        }
    }
}
```

No more maintaining registry YAML separately. Add a detector → registry updates automatically. `--stable-only` generates the LLM prompt known tags list, excluding experimental detectors that shouldn't be in the prompt yet.

---

## 16. Migration Checklist

| # | Step | Effort | Blocks |
|---|------|--------|--------|
| 1 | Create `internal/detectors/` directory structure + `terraform/{aws,gcp,azure}/` | 5 min | — |
| 2 | Write `registry.go` with Register/All/RunAll/TagMetadata | 45 min | — |
| 3 | Move K8s helpers to `detectors/k8s/helpers.go` | 15 min | — |
| 4 | Create `detectors/terraform/helpers.go` from existing TF parsing code | 30 min | — |
| 5 | Split 7 existing detectors into individual files with Metadata() | 1.5 hours | Steps 2-4 |
| 6 | Add `init()` self-registration to each | 15 min | Step 5 |
| 7 | Create `all/all.go` import file | 5 min | Step 6 |
| 8 | Write `contract_test.go` for detector test contract | 30 min | Step 7 |
| 9 | Update `internal/risk/` to use new registry | 15 min | Step 7 |
| 10 | Update `internal/lifecycle/service.go` | 10 min | Step 9 |
| 11 | Delete old `internal/risk/detectors.go` | 5 min | Step 10 |
| 12 | Run full test suite — zero behavioral change | 10 min | Step 11 |
| 13 | Add 13 new detectors (from PARALLEL_EXECUTION_PLAN) | 1 day | Step 12 |
| 14 | Add Docker adapter + 3 docker detectors | half day | Step 12 |
| 15 | Add 20 cloud detectors (AWS/GCP/Azure under terraform/) | 1 day | Step 13 |
| 16 | Write CONTRIBUTING.md detector guide | 30 min | Step 15 |

**Total: 3-4 days for complete migration + 40 detectors.**

Steps 1-11 (migration only, no new detectors): **half a day.** Same behavior, better architecture. Then new detectors flow naturally.
