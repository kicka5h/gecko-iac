//go:build integration

package foss

// Integration tests exercising the real Proxmox API end-to-end through the
// provider. They only run with -tags integration and skip unless
// PROXMOX_TEST_ENDPOINT is set:
//
//	PROXMOX_TEST_ENDPOINT=https://pve.example:8006 \
//	PROXMOX_TEST_TOKEN_ID='root@pam!gecko' \
//	PROXMOX_TEST_TOKEN_SECRET=... \
//	PROXMOX_TEST_NODE=pve \
//	PROXMOX_TEST_INSECURE=true \
//	go test -tags integration -v -run TestIntegration ./providers/foss/
//
// Snapshot and VM-scoped firewall tests additionally need PROXMOX_TEST_VMID
// pointing at an existing, snapshottable guest.
//
// All resources created here are prefixed "geckoit" and are deleted by the
// tests themselves (with t.Cleanup as a safety net).

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/gecko-iac/gecko/internal/core"
)

func integrationProvider(t *testing.T) *ProxmoxProvider {
	t.Helper()
	endpoint := os.Getenv("PROXMOX_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("PROXMOX_TEST_ENDPOINT not set; skipping integration test")
	}
	node := os.Getenv("PROXMOX_TEST_NODE")
	if node == "" {
		node = "pve"
	}
	insecure, _ := strconv.ParseBool(os.Getenv("PROXMOX_TEST_INSECURE"))
	return NewProxmoxProvider(map[string]interface{}{
		"endpoint":     endpoint,
		"token_id":     os.Getenv("PROXMOX_TEST_TOKEN_ID"),
		"token_secret": os.Getenv("PROXMOX_TEST_TOKEN_SECRET"),
		"node":         node,
		"insecure":     insecure,
	})
}

// cleanupState deletes state on test exit unless the test already did.
func cleanupState(t *testing.T, p *ProxmoxProvider, state *core.ResourceState) {
	t.Helper()
	t.Cleanup(func() {
		if err := p.Delete(context.Background(), state); err != nil {
			t.Logf("cleanup of %s %q failed: %v", state.Type, state.ExternalID, err)
		}
	})
}

func TestIntegrationPoolLifecycle(t *testing.T) {
	p := integrationProvider(t)
	ctx := context.Background()

	created, err := p.Create(ctx, core.ResourceArgs{
		Type: "proxmox:pool", Name: "geckoit-pool",
		Inputs: core.Inputs{"comment": "gecko integration test"},
	})
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	cleanupState(t, p, created)

	read, err := p.Read(ctx, created.ID, created.ExternalID)
	if err != nil {
		t.Fatalf("read pool: %v", err)
	}
	if read == nil {
		t.Fatal("pool not found after create")
	}
	if read.Inputs["comment"] != "gecko integration test" {
		t.Errorf("want comment round-tripped, got %v", read.Inputs["comment"])
	}

	updated, err := p.Update(ctx, created, core.ResourceArgs{
		Type: "proxmox:pool", Name: "geckoit-pool",
		Inputs: core.Inputs{"comment": "updated"},
	})
	if err != nil {
		t.Fatalf("update pool: %v", err)
	}
	if updated.Inputs["comment"] != "updated" {
		t.Errorf("want updated comment, got %v", updated.Inputs["comment"])
	}

	if err := p.Delete(ctx, created); err != nil {
		t.Fatalf("delete pool: %v", err)
	}
	gone, err := p.Read(ctx, created.ID, created.ExternalID)
	if err != nil {
		t.Fatalf("read deleted pool: %v", err)
	}
	if gone != nil {
		t.Errorf("pool still present after delete: %+v", gone)
	}
	// Second delete must be a no-op.
	if err := p.Delete(ctx, created); err != nil {
		t.Errorf("delete of already-deleted pool: %v", err)
	}
}

func TestIntegrationRoleAndUserAndACLLifecycle(t *testing.T) {
	p := integrationProvider(t)
	ctx := context.Background()

	role, err := p.Create(ctx, core.ResourceArgs{
		Type: "proxmox:role", Name: "geckoit-role",
		Inputs: core.Inputs{"privs": "VM.Audit"},
	})
	if err != nil {
		t.Fatalf("create role: %v", err)
	}
	cleanupState(t, p, role)

	readRole, err := p.Read(ctx, role.ID, role.ExternalID)
	if err != nil {
		t.Fatalf("read role: %v", err)
	}
	if readRole == nil || readRole.Inputs["privs"] != "VM.Audit" {
		t.Fatalf("role not round-tripped: %+v", readRole)
	}

	updatedRole, err := p.Update(ctx, role, core.ResourceArgs{
		Type: "proxmox:role", Name: "geckoit-role",
		Inputs: core.Inputs{"privs": "VM.Audit,VM.PowerMgmt"},
	})
	if err != nil {
		t.Fatalf("update role: %v", err)
	}
	if updatedRole.Inputs["privs"] != "VM.Audit,VM.PowerMgmt" {
		t.Errorf("want updated privs, got %v", updatedRole.Inputs["privs"])
	}

	user, err := p.Create(ctx, core.ResourceArgs{
		Type: "proxmox:user", Name: "geckoit",
		Inputs: core.Inputs{"userid": "geckoit@pve", "enable": true, "comment": "gecko integration test"},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	cleanupState(t, p, user)

	readUser, err := p.Read(ctx, user.ID, user.ExternalID)
	if err != nil {
		t.Fatalf("read user: %v", err)
	}
	if readUser == nil || readUser.Inputs["enable"] != true {
		t.Fatalf("user not round-tripped: %+v", readUser)
	}

	acl, err := p.Create(ctx, core.ResourceArgs{
		Type: "proxmox:acl", Name: "geckoit-acl",
		Inputs: core.Inputs{"path": "/pool/geckoit-nonexistent", "roles": "geckoit-role", "users": "geckoit@pve", "propagate": true},
	})
	if err != nil {
		t.Fatalf("create acl: %v", err)
	}
	cleanupState(t, p, acl)

	readACL, err := p.Read(ctx, acl.ID, acl.ExternalID)
	if err != nil {
		t.Fatalf("read acl: %v", err)
	}
	if readACL == nil || readACL.Inputs["users"] != "geckoit@pve" {
		t.Fatalf("acl not round-tripped: %+v", readACL)
	}

	// Tear down in dependency order and verify each is gone.
	for _, state := range []*core.ResourceState{acl, user, role} {
		if err := p.Delete(ctx, state); err != nil {
			t.Fatalf("delete %s: %v", state.Type, err)
		}
		gone, err := p.Read(ctx, state.ID, state.ExternalID)
		if err != nil {
			t.Fatalf("read deleted %s: %v", state.Type, err)
		}
		if gone != nil {
			t.Errorf("%s still present after delete", state.Type)
		}
	}
}

func TestIntegrationHAGroupLifecycle(t *testing.T) {
	p := integrationProvider(t)
	ctx := context.Background()

	group, err := p.Create(ctx, core.ResourceArgs{
		Type: "proxmox:ha_group", Name: "geckoit-ha",
		Inputs: core.Inputs{"nodes": p.node, "comment": "gecko integration test"},
	})
	if err != nil {
		t.Fatalf("create ha group: %v", err)
	}
	cleanupState(t, p, group)

	read, err := p.Read(ctx, group.ID, group.ExternalID)
	if err != nil {
		t.Fatalf("read ha group: %v", err)
	}
	if read == nil || read.Inputs["nodes"] != p.node {
		t.Fatalf("ha group not round-tripped: %+v", read)
	}

	if err := p.Delete(ctx, group); err != nil {
		t.Fatalf("delete ha group: %v", err)
	}
}

func TestIntegrationBackupJobLifecycle(t *testing.T) {
	p := integrationProvider(t)
	ctx := context.Background()

	storage := os.Getenv("PROXMOX_TEST_BACKUP_STORAGE")
	if storage == "" {
		storage = "local"
	}

	// Disabled job: registered with the scheduler but never runs.
	job, err := p.Create(ctx, core.ResourceArgs{
		Type: "proxmox:backup", Name: "geckoit-backup",
		Inputs: core.Inputs{"vmid": "all", "storage": storage, "mode": "snapshot", "schedule": "sat 03:00", "enabled": false},
	})
	if err != nil {
		t.Fatalf("create backup job: %v", err)
	}
	cleanupState(t, p, job)
	if job.ExternalID == "" {
		t.Fatal("backup job created without an ID")
	}

	read, err := p.Read(ctx, job.ID, job.ExternalID)
	if err != nil {
		t.Fatalf("read backup job: %v", err)
	}
	if read == nil || read.Inputs["storage"] != storage {
		t.Fatalf("backup job not round-tripped: %+v", read)
	}
	if read.Inputs["enabled"] != false {
		t.Errorf("want job disabled, got %v", read.Inputs["enabled"])
	}

	if err := p.Delete(ctx, job); err != nil {
		t.Fatalf("delete backup job: %v", err)
	}
}

func TestIntegrationFirewallRuleLifecycle(t *testing.T) {
	p := integrationProvider(t)
	ctx := context.Background()

	// Disabled cluster-level rule: inert even if the cluster firewall is on.
	rule, err := p.Create(ctx, core.ResourceArgs{
		Type: "proxmox:firewall_rule", Name: "geckoit-rule",
		Inputs: core.Inputs{"type": "in", "action": "ACCEPT", "proto": "tcp", "dport": "22222", "enable": false, "comment": "gecko integration test"},
	})
	if err != nil {
		t.Fatalf("create firewall rule: %v", err)
	}
	cleanupState(t, p, rule)

	read, err := p.Read(ctx, rule.ID, rule.ExternalID)
	if err != nil {
		t.Fatalf("read firewall rule: %v", err)
	}
	if read == nil || read.Inputs["dport"] != "22222" {
		t.Fatalf("firewall rule not round-tripped: %+v", read)
	}

	if err := p.Delete(ctx, rule); err != nil {
		t.Fatalf("delete firewall rule: %v", err)
	}
}

func TestIntegrationSnapshotLifecycle(t *testing.T) {
	p := integrationProvider(t)
	ctx := context.Background()

	vmidStr := os.Getenv("PROXMOX_TEST_VMID")
	if vmidStr == "" {
		t.Skip("PROXMOX_TEST_VMID not set; skipping snapshot test")
	}
	vmid, err := strconv.Atoi(vmidStr)
	if err != nil {
		t.Fatalf("invalid PROXMOX_TEST_VMID %q: %v", vmidStr, err)
	}

	snap, err := p.Create(ctx, core.ResourceArgs{
		Type: "proxmox:snapshot", Name: "geckoit-snap",
		Inputs: core.Inputs{"vmid": vmid, "description": "gecko integration test"},
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	cleanupState(t, p, snap)
	if want := fmt.Sprintf("%d/geckoit-snap", vmid); snap.ExternalID != want {
		t.Errorf("want externalID %q, got %q", want, snap.ExternalID)
	}

	read, err := p.Read(ctx, snap.ID, snap.ExternalID)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if read == nil || read.Inputs["description"] != "gecko integration test" {
		t.Fatalf("snapshot not round-tripped: %+v", read)
	}

	if err := p.Delete(ctx, snap); err != nil {
		t.Fatalf("delete snapshot: %v", err)
	}
}

func TestIntegrationClusterOptionsRead(t *testing.T) {
	p := integrationProvider(t)

	// Read-only: never mutate real cluster options from tests.
	state, err := p.Read(context.Background(), "proxmox:cluster_options::cluster", "cluster")
	if err != nil {
		t.Fatalf("read cluster options: %v", err)
	}
	if state == nil {
		t.Fatal("cluster options read returned nil")
	}
	t.Logf("cluster options: %+v", state.Inputs)
}
