package cross

import (
	"testing"
)

func normalize(tfType string, attrs map[string]interface{}) *ImportedResource {
	res := &ImportedResource{
		SourceKind: SourceTerraform,
		SourceType: tfType,
		GeckoType:  MapTerraformType(tfType),
		Name:       "raw-name",
		Inputs:     attrs,
	}
	normalizeTFInputs(tfType, res)
	return res
}

func TestNormalizeTF_ProxmoxVM(t *testing.T) {
	res := normalize("proxmox_vm_qemu", map[string]interface{}{
		"name":        "web-01",
		"target_node": "pve",
		"vmid":        float64(100),
		"memory":      float64(2048),
		"cores":       float64(2),
		"desc":        "web server",
		"agent":       float64(1),
		"ciuser":      "admin",
		"ipconfig0":   "ip=10.0.0.5/24,gw=10.0.0.1",
		"network": map[string]interface{}{ // single flattened block
			"model":    "virtio",
			"bridge":   "vmbr0",
			"tag":      float64(10),
			"firewall": true,
		},
		"disk": map[string]interface{}{
			"type":    "scsi",
			"slot":    float64(0),
			"storage": "local-lvm",
			"size":    "20G",
		},
		"unwanted_attr": "should be dropped",
	})

	if res.Name != "web-01" {
		t.Errorf("want name web-01, got %q", res.Name)
	}
	if res.Inputs["node"] != "pve" {
		t.Errorf("want target_node renamed to node, got %v", res.Inputs["node"])
	}
	if res.Inputs["description"] != "web server" {
		t.Errorf("want desc renamed to description, got %v", res.Inputs["description"])
	}
	if res.Inputs["net0"] != "virtio,bridge=vmbr0,tag=10,firewall=1" {
		t.Errorf("net0 not composed: %v", res.Inputs["net0"])
	}
	if res.Inputs["scsi0"] != "local-lvm:20" {
		t.Errorf("scsi0 not composed: %v", res.Inputs["scsi0"])
	}
	if _, ok := res.Inputs["unwanted_attr"]; ok {
		t.Error("unlisted attributes should be dropped")
	}
}

func TestNormalizeTF_ProxmoxVM_MultipleNetworks(t *testing.T) {
	res := normalize("proxmox_vm_qemu", map[string]interface{}{
		"name": "router",
		"network": []interface{}{
			map[string]interface{}{"model": "virtio", "bridge": "vmbr0"},
			map[string]interface{}{"model": "e1000", "bridge": "vmbr1"},
		},
	})
	if res.Inputs["net0"] != "virtio,bridge=vmbr0" {
		t.Errorf("net0: %v", res.Inputs["net0"])
	}
	if res.Inputs["net1"] != "e1000,bridge=vmbr1" {
		t.Errorf("net1: %v", res.Inputs["net1"])
	}
}

func TestNormalizeTF_ProxmoxLXC(t *testing.T) {
	res := normalize("proxmox_lxc", map[string]interface{}{
		"hostname":    "cache-01",
		"target_node": "pve",
		"memory":      float64(512),
		"swap":        float64(512),
		"ostemplate":  "local:vztmpl/alpine-3.19.tar.xz",
		"rootfs": map[string]interface{}{
			"storage": "local-lvm",
			"size":    "8G",
		},
		"network": map[string]interface{}{
			"name":   "eth0",
			"bridge": "vmbr0",
			"ip":     "dhcp",
		},
	})

	if res.Name != "cache-01" {
		t.Errorf("want name from hostname, got %q", res.Name)
	}
	if res.Inputs["rootfs"] != "local-lvm:8" {
		t.Errorf("rootfs: %v", res.Inputs["rootfs"])
	}
	if res.Inputs["net0"] != "name=eth0,bridge=vmbr0,ip=dhcp" {
		t.Errorf("net0: %v", res.Inputs["net0"])
	}
}

func TestNormalizeTF_FlyMachine(t *testing.T) {
	res := normalize("fly_machine", map[string]interface{}{
		"name":     "worker",
		"app":      "my-app",
		"region":   "syd",
		"image":    "flyio/worker:v2",
		"cpus":     float64(2),
		"memorymb": float64(512),
		"env":      map[string]interface{}{"MODE": "prod"},
	})

	if res.Name != "worker" {
		t.Errorf("name: %q", res.Name)
	}
	if res.Inputs["memory"] != float64(512) {
		t.Errorf("want memorymb renamed to memory, got %v", res.Inputs["memory"])
	}
	if res.Inputs["image"] != "flyio/worker:v2" || res.Inputs["region"] != "syd" {
		t.Errorf("core fields missing: %+v", res.Inputs)
	}
}

func TestNormalizeTF_OpenStackInstance(t *testing.T) {
	res := normalize("openstack_compute_instance_v2", map[string]interface{}{
		"name":        "app-server",
		"flavor_name": "m1.small",
		"image_name":  "ubuntu-22.04",
		"key_pair":    "deploy-key",
		"network": []interface{}{
			map[string]interface{}{"name": "private", "uuid": "abc-123"},
			map[string]interface{}{"name": "public"},
		},
	})

	if res.Name != "app-server" {
		t.Errorf("name: %q", res.Name)
	}
	if res.Inputs["network_id"] != "abc-123" {
		t.Errorf("want first network uuid as network_id, got %v", res.Inputs["network_id"])
	}
	if res.Inputs["network"] != "private" {
		t.Errorf("network name: %v", res.Inputs["network"])
	}
}

func TestNormalizeTF_OpenNebulaVM(t *testing.T) {
	res := normalize("opennebula_virtual_machine", map[string]interface{}{
		"name":        "one-vm",
		"cpu":         float64(1),
		"memory":      float64(1024),
		"template_id": float64(7),
		"disk": map[string]interface{}{
			"image_id": float64(0), // id 0 is valid in OpenNebula
			"size":     float64(10240),
		},
		"nic": map[string]interface{}{
			"network_id": float64(3),
		},
		"context": map[string]interface{}{"SSH_PUBLIC_KEY": "ssh-ed25519 ..."},
	})

	if res.Inputs["image_id"] != 0 {
		t.Errorf("image_id 0 should survive: %v", res.Inputs["image_id"])
	}
	if res.Inputs["disk_size"] != 10240 {
		t.Errorf("disk_size: %v", res.Inputs["disk_size"])
	}
	if res.Inputs["network_id"] != 3 {
		t.Errorf("network_id: %v", res.Inputs["network_id"])
	}
	if _, ok := res.Inputs["context"]; !ok {
		t.Error("context block should be preserved")
	}
}

func TestNormalizeTF_UnknownType_PassesThrough(t *testing.T) {
	attrs := map[string]interface{}{"anything": "goes", "keep": true}
	res := normalize("some_unmapped_type", attrs)
	if len(res.Inputs) != 2 {
		t.Errorf("unmapped types should keep raw inputs, got %+v", res.Inputs)
	}
	if res.Name != "raw-name" {
		t.Errorf("unmapped types should keep original name, got %q", res.Name)
	}
}
