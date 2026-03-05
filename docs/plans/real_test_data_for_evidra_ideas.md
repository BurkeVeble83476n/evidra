> Follow-up discussion draft: [2026-03-05-real-test-data-acquisition-plan.md](./2026-03-05-real-test-data-acquisition-plan.md)

Реальные уязвимые Terraform конфигурации
Checkov test suites

Репозиторий содержит намеренно небезопасные Terraform конфигурации.

https://github.com/bridgecrewio/checkov/tree/main/tests/terraform

Примеры проблем:

S3 bucket public read/write

Security group 0.0.0.0/0

Unencrypted RDS

IAM wildcard permissions

Open SSH

Использование для Evidra:

tests/fixtures/terraform/bad/

s3_public.tf
sg_open_ssh.tf
iam_admin.tf
rds_no_encryption.tf

Затем:

terraform plan
terraform show -json tfplan > plan_bad_s3.json
2. Реальные Kubernetes security failures
Kubescape test workloads

Kubescape содержит набор уязвимых Kubernetes manifests.

https://github.com/kubescape/kubescape/tree/master/examples

Типичные проблемы:

privileged containers

hostPath mount

hostNetwork

cluster-admin RBAC

root containers

Пример проблемного pod:

apiVersion: v1
kind: Pod
metadata:
  name: privileged-pod
spec:
  containers:
  - name: bad
    image: nginx
    securityContext:
      privileged: true

Evidra сигнал:

privileged_container
blast_radius
forbidden_scope
3. OWASP Kubernetes Top-10 примеры

OWASP
OWASP публикует реальные insecure manifests.

https://github.com/OWASP/Top10-Kubernetes

Примеры:

exposed dashboard

service account token abuse

RBAC privilege escalation

4. Real cloud misconfiguration datasets
Netflix

Netflix open-source misconfiguration examples:

https://github.com/Netflix/security_monkey

Содержит реальные AWS misconfigurations:

open S3 buckets

public AMIs

permissive IAM roles

5. Terraform “destructive change” examples

Это особенно важно для вашего сигнала blast_radius.

Пример:

terraform destroy

или план:

resource_changes:
  - delete 120 resources

Можно взять из:

https://github.com/hashicorp/terraform-provider-aws/tree/main/examples

Создать план:

terraform destroy -out destroy.plan
terraform show -json destroy.plan

Fixture:

plan_mass_destroy.json
6. Kubernetes outage examples (реальные инциденты)
etcd wipe scenario

Удаление persistent volumes etcd.

Пример:

kubectl delete pvc --all

или manifest с hostPath:

/var/lib/etcd
kube-system disruption

Пример:

kubectl delete namespace kube-system

Это хороший тест:

forbidden_scope
7. Privilege escalation manifests

RBAC cluster-admin binding:

kind: ClusterRoleBinding
subjects:
- kind: ServiceAccount
  name: default
roleRef:
  kind: ClusterRole
  name: cluster-admin

Источник:

https://github.com/kubernetes/kubernetes/tree/master/plugin/pkg/auth/authorizer/rbac/bootstrappolicy

8. Container breakout scenarios
Falco examples

https://github.com/falcosecurity/falco/tree/master/examples

Типичные проблемы:

hostPID

hostNetwork

root filesystem write

9. Real CI/CD failure scenarios

Можно взять из:

GitHub
https://github.com/actions/starter-workflows

И сделать intentionally bad workflow:

kubectl apply -f prod/
terraform apply -auto-approve

без проверки.

10. Самый мощный dataset (рекомендую)

Соберите 20–30 “bad artifacts”:

Terraform

public S3

open SSH SG

wildcard IAM

mass destroy

cross-account change

Kubernetes

privileged container

hostPath mount

cluster-admin RBAC

kube-system modification

root containers

Scanners

Trivy vulnerabilities

Kubescape policy violations

Checkov findings

Agent behaviors

retry loop

infinite plan/apply

destructive drift

Рекомендуемая структура
benchmark-dataset/

terraform/
  public_s3/
  open_ssh/
  iam_admin/
  mass_destroy/

kubernetes/
  privileged_pod/
  cluster_admin_rbac/
  hostpath_mount/
  kube_system_modify/

scanners/
  trivy_high_vuln/
  kubescape_nsa_fail/
  checkov_high/

agents/
  retry_loop/
  blast_radius/
  forbidden_scope/
Почему это критично для Evidra

Если у нас есть такой dataset, вы можете показать:

agent run
  ↓
bad artifact
  ↓
scanner findings
  ↓
evidra signals
  ↓
scorecard risk

Это превращает Evidra из “инструмента” в benchmark framework для AI-driven infrastructure automation.

Нужно все структурировать по уровню опасности и по тику : security, disaster, misconfiguration,etc
