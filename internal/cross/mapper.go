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
	// ── Proxmox (telmate/proxmox) ────────────────────────────────────────────────
	"proxmox_vm_qemu":  "proxmox:vm",
	"proxmox_lxc":      "proxmox:lxc",
	"proxmox_storage":  "proxmox:storage",
	"proxmox_network":  "proxmox:network",

	// ── Fly.io (fly-apps/terraform-provider-fly) ────────────────────────────────
	"fly_app":     "fly:app",
	"fly_machine": "fly:machine",
	"fly_volume":  "fly:volume",

	// ── OpenStack (terraform-provider-openstack) ────────────────────────────────
	"openstack_compute_instance_v2":    "openstack:instance",
	"openstack_networking_network_v2":  "openstack:network",
	"openstack_networking_subnet_v2":   "openstack:subnet",
	"openstack_networking_secgroup_v2": "openstack:security_group",
	"openstack_blockstorage_volume_v3": "openstack:volume",

	// ── OpenNebula (terraform-provider-opennebula) ──────────────────────────────
	"opennebula_virtual_machine": "opennebula:vm",
	"opennebula_virtual_network": "opennebula:vnet",
	"opennebula_image":           "opennebula:image",
	"opennebula_template":        "opennebula:template",

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
	// ── OpenStack ────────────────────────────────────────────────────────────────
	"openstack:compute/instance:Instance":     "openstack:instance",
	"openstack:networking/network:Network":     "openstack:network",
	"openstack:networking/subnet:Subnet":       "openstack:subnet",
	"openstack:networking/secGroup:SecGroup":   "openstack:security_group",
	"openstack:blockstorage/volumeV3:VolumeV3": "openstack:volume",
}
