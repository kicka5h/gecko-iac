package cross

// MapTerraformType converts a Terraform resource type to a Gecko resource type.
// Returns an empty string if no mapping exists.
func MapTerraformType(tfType string) string { return terraformTypeMap[tfType] }

// MapPulumiType converts a Pulumi resource type to a Gecko resource type.
// Returns an empty string if no mapping exists.
func MapPulumiType(pulumiType string) string { return pulumiTypeMap[pulumiType] }

// terraformTypeMap maps Terraform resource type strings to Gecko resource types.
// Covers the FOSS provider ecosystem that Gecko targets.
var terraformTypeMap = map[string]string{
	// ── Kubernetes (hashicorp/kubernetes) ────────────────────────────────────────
	"kubernetes_namespace":                 "k8s:namespace",
	"kubernetes_deployment":                "k8s:deployment",
	"kubernetes_stateful_set":              "k8s:statefulset",
	"kubernetes_daemonset":                 "k8s:daemonset",
	"kubernetes_service":                   "k8s:service",
	"kubernetes_ingress":                   "k8s:ingress",
	"kubernetes_ingress_v1":                "k8s:ingress",
	"kubernetes_config_map":                "k8s:configmap",
	"kubernetes_config_map_v1":             "k8s:configmap",
	"kubernetes_secret":                    "k8s:secret",
	"kubernetes_secret_v1":                 "k8s:secret",
	"kubernetes_persistent_volume_claim":   "k8s:persistentvolumeclaim",
	"kubernetes_persistent_volume":         "k8s:persistentvolume",
	"kubernetes_storage_class":             "k8s:storageclass",
	"kubernetes_service_account":           "k8s:serviceaccount",
	"kubernetes_cluster_role":              "k8s:clusterrole",
	"kubernetes_cluster_role_binding":      "k8s:clusterrolebinding",
	"kubernetes_role":                      "k8s:role",
	"kubernetes_role_binding":              "k8s:rolebinding",
	"kubernetes_horizontal_pod_autoscaler": "k8s:horizontalpodautoscaler",
	"kubernetes_pod_disruption_budget":     "k8s:poddisruptionbudget",
	"kubernetes_network_policy":            "k8s:networkpolicy",
	"kubernetes_job":                       "k8s:job",
	"kubernetes_cron_job":                  "k8s:cronjob",
	"kubernetes_manifest":                  "k8s:customresource",

	// ── Nomad (hashicorp/nomad) ──────────────────────────────────────────────────
	"nomad_job":        "nomad:job",
	"nomad_namespace":  "nomad:namespace",
	"nomad_volume":     "nomad:volume",
	"nomad_acl_policy": "nomad:policy",

	// ── Vault (hashicorp/vault) ──────────────────────────────────────────────────
	"vault_policy":             "vault:policy",
	"vault_auth_backend":       "vault:auth",
	"vault_mount":              "vault:mount",
	"vault_generic_secret":     "vault:secret",
	"vault_kv_secret":          "vault:secret",
	"vault_kv_secret_v2":       "vault:secret",
	"vault_pki_secret_backend": "vault:mount",

	// ── Gitea (terraform-gitea-provider) ─────────────────────────────────────────
	"gitea_repository": "gitea:repo",
	"gitea_org":        "gitea:org",
	"gitea_user":       "gitea:user",
	"gitea_webhook":    "gitea:webhook",

	// ── MinIO (aminueza/minio) ───────────────────────────────────────────────────
	"minio_s3_bucket":  "minio:bucket",
	"minio_iam_policy": "minio:policy",
	"minio_iam_user":   "minio:user",

	// ── PostgreSQL (cyrilgdn/postgresql) ─────────────────────────────────────────
	"postgresql_database":  "postgresql:database",
	"postgresql_role":      "postgresql:role",
	"postgresql_extension": "postgresql:extension",
	"postgresql_schema":    "postgresql:schema",
}

// pulumiTypeMap maps Pulumi resource type strings to Gecko resource types.
var pulumiTypeMap = map[string]string{
	// ── Kubernetes ───────────────────────────────────────────────────────────────
	"kubernetes:core/v1:Namespace":                                "k8s:namespace",
	"kubernetes:apps/v1:Deployment":                               "k8s:deployment",
	"kubernetes:apps/v1:StatefulSet":                              "k8s:statefulset",
	"kubernetes:apps/v1:DaemonSet":                                "k8s:daemonset",
	"kubernetes:core/v1:Service":                                  "k8s:service",
	"kubernetes:networking.k8s.io/v1:Ingress":                     "k8s:ingress",
	"kubernetes:core/v1:ConfigMap":                                "k8s:configmap",
	"kubernetes:core/v1:Secret":                                   "k8s:secret",
	"kubernetes:core/v1:PersistentVolumeClaim":                    "k8s:persistentvolumeclaim",
	"kubernetes:core/v1:PersistentVolume":                         "k8s:persistentvolume",
	"kubernetes:storage.k8s.io/v1:StorageClass":                   "k8s:storageclass",
	"kubernetes:core/v1:ServiceAccount":                           "k8s:serviceaccount",
	"kubernetes:rbac.authorization.k8s.io/v1:ClusterRole":         "k8s:clusterrole",
	"kubernetes:rbac.authorization.k8s.io/v1:ClusterRoleBinding":  "k8s:clusterrolebinding",
	"kubernetes:rbac.authorization.k8s.io/v1:Role":                "k8s:role",
	"kubernetes:rbac.authorization.k8s.io/v1:RoleBinding":         "k8s:rolebinding",
	"kubernetes:autoscaling/v1:HorizontalPodAutoscaler":           "k8s:horizontalpodautoscaler",
	"kubernetes:autoscaling/v2:HorizontalPodAutoscaler":           "k8s:horizontalpodautoscaler",
	"kubernetes:policy/v1:PodDisruptionBudget":                    "k8s:poddisruptionbudget",
	"kubernetes:networking.k8s.io/v1:NetworkPolicy":               "k8s:networkpolicy",
	"kubernetes:batch/v1:Job":                                     "k8s:job",
	"kubernetes:batch/v1:CronJob":                                 "k8s:cronjob",
	"kubernetes:apiextensions.k8s.io/v1:CustomResourceDefinition": "k8s:customresource",

	// ── Vault ────────────────────────────────────────────────────────────────────
	"vault:index/policy:Policy":  "vault:policy",
	"vault:index/mount:Mount":    "vault:mount",
	"vault:kv/secretV2:SecretV2": "vault:secret",
	"vault:kv/secret:Secret":     "vault:secret",

	// ── Nomad ────────────────────────────────────────────────────────────────────
	"nomad:index/job:Job":             "nomad:job",
	"nomad:index/namespace:Namespace": "nomad:namespace",
	"nomad:index/volume:Volume":       "nomad:volume",
}
