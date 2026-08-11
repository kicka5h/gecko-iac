package cross

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// tfState represents the top-level Terraform state file structure (version 4).
type tfState struct {
	Version          int          `json:"version"`
	TerraformVersion string       `json:"terraform_version"`
	Resources        []tfResource `json:"resources"`
}

type tfResource struct {
	Module    string       `json:"module"`
	Mode      string       `json:"mode"` // "managed" or "data"
	Type      string       `json:"type"`
	Name      string       `json:"name"`
	Instances []tfInstance `json:"instances"`
}

type tfInstance struct {
	Attributes map[string]interface{} `json:"attributes"`
}

// ParseTerraformState reads a Terraform state file (v4) and returns a CrossResult.
func ParseTerraformState(path string) (*CrossResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read state file: %w", err)
	}

	var state tfState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("invalid Terraform state JSON: %w", err)
	}
	if state.Version < 4 {
		return nil, fmt.Errorf("unsupported Terraform state version %d (need >= 4)", state.Version)
	}

	result := &CrossResult{
		SourceFile:  path,
		Kind:        SourceTerraform,
		ToolVersion: state.TerraformVersion,
		ImportedAt:  time.Now(),
	}

	for _, r := range state.Resources {
		if r.Mode != "managed" {
			continue // skip data sources
		}
		for i, inst := range r.Instances {
			name := r.Name
			if len(r.Instances) > 1 {
				name = fmt.Sprintf("%s-%d", r.Name, i)
			}

			attrs := flattenTFAttrs(inst.Attributes)
			externalID, _ := attrs["id"].(string)
			delete(attrs, "id")
			delete(attrs, "timeouts")

			res := &ImportedResource{
				SourceKind: SourceTerraform,
				SourceType: r.Type,
				GeckoType:  MapTerraformType(r.Type),
				Name:       sanitizeName(name),
				ExternalID: externalID,
				Inputs:     attrs,
				Module:     r.Module,
			}

			normalizeTFInputs(r.Type, res)
			result.Resources = append(result.Resources, res)
		}
	}

	return result, nil
}

// flattenTFAttrs flattens Terraform's block-as-array convention.
// Single-element arrays of objects are unwrapped to maps; empty values are pruned.
func flattenTFAttrs(attrs map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(attrs))
	for k, v := range attrs {
		if flat := flattenTFValue(v); flat != nil {
			out[k] = flat
		}
	}
	return out
}

func flattenTFValue(v interface{}) interface{} {
	switch val := v.(type) {
	case []interface{}:
		if len(val) == 0 {
			return nil
		}
		// Single-element block arrays are unwrapped (the Terraform block-as-array pattern)
		if len(val) == 1 {
			if m, ok := val[0].(map[string]interface{}); ok {
				flat := flattenTFAttrs(m)
				if len(flat) == 0 {
					return nil
				}
				return flat
			}
		}
		// Multi-element: flatten each element individually
		result := make([]interface{}, 0, len(val))
		for _, el := range val {
			if flat := flattenTFValue(el); flat != nil {
				result = append(result, flat)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result

	case map[string]interface{}:
		flat := flattenTFAttrs(val)
		if len(flat) == 0 {
			return nil
		}
		return flat

	case string:
		if val == "" {
			return nil
		}
		return val

	case float64, bool:
		return val

	case nil:
		return nil
	}
	return v
}

// normalizeTFInputs applies type-specific extraction to pull the most meaningful
// fields out of a flattened Terraform attributes map.
func normalizeTFInputs(tfType string, res *ImportedResource) {
	attrs := res.Inputs

	switch {
	case tfType == "gitea_repository":
		clean := make(map[string]interface{})
		for _, k := range []string{"name", "owner", "description", "private", "auto_init", "default_branch"} {
			if v, ok := attrs[k]; ok {
				clean[k] = v
			}
		}
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	case tfType == "minio_s3_bucket":
		clean := make(map[string]interface{})
		for _, k := range []string{"bucket", "acl", "force_destroy"} {
			if v, ok := attrs[k]; ok {
				clean[k] = v
			}
		}
		if b, ok := attrs["bucket"].(string); ok {
			res.Name = sanitizeName(b)
		}
		res.Inputs = clean

	case tfType == "postgresql_database" || tfType == "postgresql_role":
		clean := make(map[string]interface{})
		for _, k := range []string{"name", "owner", "encoding", "lc_collate", "lc_ctype", "login", "superuser"} {
			if v, ok := attrs[k]; ok {
				clean[k] = v
			}
		}
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	// ── Proxmox (telmate/proxmox) ────────────────────────────────────────────

	case tfType == "proxmox_vm_qemu":
		clean := pickInputs(attrs,
			"name", "vmid", "memory", "cores", "sockets", "cpu", "ostype",
			"onboot", "agent", "tags", "scsihw", "boot", "bios", "vga",
			"ciuser", "cipassword", "ipconfig0", "sshkeys", "nameserver", "searchdomain")
		renameInput(attrs, clean, "target_node", "node")
		renameInput(attrs, clean, "desc", "description")
		for i, net := range tfBlocks(attrs["network"]) {
			if i > 3 {
				break
			}
			if s := tfProxmoxNet(net); s != "" {
				clean[fmt.Sprintf("net%d", i)] = s
			}
		}
		for i, d := range tfBlocks(attrs["disk"]) {
			if i > 3 {
				break
			}
			spec := tfProxmoxDisk(d)
			if spec == "" {
				continue
			}
			slot := fmt.Sprintf("scsi%d", i)
			if bus := tfString(d, "type"); bus != "" {
				slot = fmt.Sprintf("%s%d", bus, tfInt(d, "slot"))
			}
			clean[slot] = spec
		}
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	case tfType == "proxmox_lxc":
		clean := pickInputs(attrs,
			"hostname", "memory", "swap", "cores", "ostemplate", "unprivileged",
			"onboot", "ssh_public_keys", "nameserver", "searchdomain", "tags", "description")
		renameInput(attrs, clean, "target_node", "node")
		if rf := tfBlocks(attrs["rootfs"]); len(rf) > 0 {
			if spec := tfProxmoxDisk(rf[0]); spec != "" {
				clean["rootfs"] = spec
			}
		}
		for i, net := range tfBlocks(attrs["network"]) {
			if i > 3 {
				break
			}
			if s := tfProxmoxLXCNet(net); s != "" {
				clean[fmt.Sprintf("net%d", i)] = s
			}
		}
		if hostname, ok := attrs["hostname"].(string); ok {
			res.Name = sanitizeName(hostname)
		}
		res.Inputs = clean

	// ── Fly.io ───────────────────────────────────────────────────────────────

	case tfType == "fly_app":
		clean := pickInputs(attrs, "name", "org")
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	case tfType == "fly_machine":
		clean := pickInputs(attrs, "name", "app", "region", "image", "cpus", "env", "services")
		renameInput(attrs, clean, "memorymb", "memory")
		renameInput(attrs, clean, "cputype", "cpu_type")
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	case tfType == "fly_volume":
		clean := pickInputs(attrs, "name", "app", "region", "size")
		renameInput(attrs, clean, "sizegb", "size")
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	// ── OpenStack ────────────────────────────────────────────────────────────

	case tfType == "openstack_compute_instance_v2":
		clean := pickInputs(attrs,
			"name", "flavor_id", "flavor_name", "image_id", "image_name",
			"key_pair", "security_groups", "user_data", "availability_zone")
		if networks := tfBlocks(attrs["network"]); len(networks) > 0 {
			if uuid := tfString(networks[0], "uuid"); uuid != "" {
				clean["network_id"] = uuid
			}
			if n := tfString(networks[0], "name"); n != "" {
				clean["network"] = n
			}
		}
		if bds := tfBlocks(attrs["block_device"]); len(bds) > 0 {
			if uuid := tfString(bds[0], "uuid"); uuid != "" {
				clean["image_id"] = uuid
			}
			if size := tfInt(bds[0], "volume_size"); size > 0 {
				clean["volume_size"] = size
			}
		}
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	case tfType == "openstack_networking_network_v2":
		clean := pickInputs(attrs, "name", "admin_state_up", "shared", "external")
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	case tfType == "openstack_networking_subnet_v2":
		clean := pickInputs(attrs, "name", "cidr", "ip_version", "gateway_ip", "enable_dhcp", "network_id")
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	case tfType == "openstack_networking_secgroup_v2":
		clean := pickInputs(attrs, "name", "description")
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	case tfType == "openstack_blockstorage_volume_v3":
		clean := pickInputs(attrs, "name", "size", "description", "volume_type")
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	// ── OpenNebula ───────────────────────────────────────────────────────────

	case tfType == "opennebula_virtual_machine":
		clean := pickInputs(attrs, "name", "cpu", "vcpu", "memory", "template_id", "group")
		if disks := tfBlocks(attrs["disk"]); len(disks) > 0 {
			if _, ok := disks[0]["image_id"]; ok {
				clean["image_id"] = tfInt(disks[0], "image_id")
			}
			if size := tfInt(disks[0], "size"); size > 0 {
				clean["disk_size"] = size
			}
		}
		if nics := tfBlocks(attrs["nic"]); len(nics) > 0 {
			if _, ok := nics[0]["network_id"]; ok {
				clean["network_id"] = tfInt(nics[0], "network_id")
			}
		}
		if ctxBlock, ok := attrs["context"].(map[string]interface{}); ok {
			clean["context"] = ctxBlock
		}
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	case tfType == "opennebula_virtual_network":
		clean := pickInputs(attrs, "name", "bridge", "physical_device", "vlan_id", "permissions")
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean

	case tfType == "opennebula_image":
		clean := pickInputs(attrs, "name", "datastore_id", "path", "persistent", "type")
		if name, ok := attrs["name"].(string); ok {
			res.Name = sanitizeName(name)
		}
		res.Inputs = clean
	}
}

// ── Normalization helpers ─────────────────────────────────────────────────────

// pickInputs copies the listed keys from attrs into a fresh input map.
func pickInputs(attrs map[string]interface{}, keys ...string) map[string]interface{} {
	clean := make(map[string]interface{})
	for _, k := range keys {
		if v, ok := attrs[k]; ok {
			clean[k] = v
		}
	}
	return clean
}

// renameInput copies attrs[from] into clean[to] when present.
func renameInput(attrs, clean map[string]interface{}, from, to string) {
	if v, ok := attrs[from]; ok {
		clean[to] = v
	}
}

// tfBlocks returns nested block(s) as a slice of maps, handling both the
// single-block (map) and repeated-block (slice) shapes flattenTFAttrs produces.
func tfBlocks(v interface{}) []map[string]interface{} {
	switch b := v.(type) {
	case map[string]interface{}:
		return []map[string]interface{}{b}
	case []interface{}:
		var out []map[string]interface{}
		for _, el := range b {
			if m, ok := el.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func tfString(m map[string]interface{}, k string) string {
	v, _ := m[k].(string)
	return v
}

func tfInt(m map[string]interface{}, k string) int {
	switch v := m[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// tfProxmoxNet renders a telmate network block as a PVE net string,
// e.g. "virtio,bridge=vmbr0,tag=10".
func tfProxmoxNet(m map[string]interface{}) string {
	model := tfString(m, "model")
	if model == "" {
		model = "virtio"
	}
	s := model
	if b := tfString(m, "bridge"); b != "" {
		s += ",bridge=" + b
	}
	if tag := tfInt(m, "tag"); tag > 0 {
		s += fmt.Sprintf(",tag=%d", tag)
	}
	if fw, ok := m["firewall"].(bool); ok && fw {
		s += ",firewall=1"
	}
	return s
}

// tfProxmoxLXCNet renders a telmate LXC network block as a PVE net string,
// e.g. "name=eth0,bridge=vmbr0,ip=dhcp".
func tfProxmoxLXCNet(m map[string]interface{}) string {
	var parts []string
	if n := tfString(m, "name"); n != "" {
		parts = append(parts, "name="+n)
	}
	if b := tfString(m, "bridge"); b != "" {
		parts = append(parts, "bridge="+b)
	}
	if ip := tfString(m, "ip"); ip != "" {
		parts = append(parts, "ip="+ip)
	}
	if gw := tfString(m, "gw"); gw != "" {
		parts = append(parts, "gw="+gw)
	}
	if tag := tfInt(m, "tag"); tag > 0 {
		parts = append(parts, fmt.Sprintf("tag=%d", tag))
	}
	return strings.Join(parts, ",")
}

// tfProxmoxDisk renders a telmate disk/rootfs block as a PVE volume spec,
// e.g. "local-lvm:20".
func tfProxmoxDisk(m map[string]interface{}) string {
	storage := tfString(m, "storage")
	if storage == "" {
		return ""
	}
	size := strings.TrimRight(tfString(m, "size"), "Gg")
	if size == "" {
		return storage
	}
	return storage + ":" + size
}

// sanitizeName converts an arbitrary string into a valid Scute resource name.
func sanitizeName(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('-')
		}
	}
	return strings.Trim(sb.String(), "-_")
}
