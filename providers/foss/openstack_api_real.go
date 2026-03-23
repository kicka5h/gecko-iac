package foss

import (
	"context"
	"fmt"
)

// realOpenStackAPI implements openstackAPI with stub methods.
// Once the OpenStack SDK is integrated, this will delegate to gophercloud or similar.
type realOpenStackAPI struct {
	authURL    string
	username   string
	password   string
	tenantName string
	region     string
	domainName string
}

func newRealOpenStackAPI(authURL, username, password, tenantName, region, domainName string) *realOpenStackAPI {
	return &realOpenStackAPI{
		authURL:    authURL,
		username:   username,
		password:   password,
		tenantName: tenantName,
		region:     region,
		domainName: domainName,
	}
}

// ── Server ────────────────────────────────────────────────────────────────────

func (r *realOpenStackAPI) CreateServer(ctx context.Context, opts map[string]interface{}) (*ServerInfo, error) {
	return nil, fmt.Errorf("openstack: CreateServer not yet implemented")
}

func (r *realOpenStackAPI) GetServer(ctx context.Context, id string) (*ServerInfo, error) {
	return nil, fmt.Errorf("openstack: GetServer not yet implemented")
}

func (r *realOpenStackAPI) UpdateServer(ctx context.Context, id string, opts map[string]interface{}) (*ServerInfo, error) {
	return nil, fmt.Errorf("openstack: UpdateServer not yet implemented")
}

func (r *realOpenStackAPI) DeleteServer(ctx context.Context, id string) error {
	return fmt.Errorf("openstack: DeleteServer not yet implemented")
}

// ── Network ───────────────────────────────────────────────────────────────────

func (r *realOpenStackAPI) CreateNetwork(ctx context.Context, opts map[string]interface{}) (*OSNetworkInfo, error) {
	return nil, fmt.Errorf("openstack: CreateNetwork not yet implemented")
}

func (r *realOpenStackAPI) GetNetwork(ctx context.Context, id string) (*OSNetworkInfo, error) {
	return nil, fmt.Errorf("openstack: GetNetwork not yet implemented")
}

func (r *realOpenStackAPI) DeleteNetwork(ctx context.Context, id string) error {
	return fmt.Errorf("openstack: DeleteNetwork not yet implemented")
}

// ── Subnet ────────────────────────────────────────────────────────────────────

func (r *realOpenStackAPI) CreateSubnet(ctx context.Context, opts map[string]interface{}) (*SubnetInfo, error) {
	return nil, fmt.Errorf("openstack: CreateSubnet not yet implemented")
}

func (r *realOpenStackAPI) GetSubnet(ctx context.Context, id string) (*SubnetInfo, error) {
	return nil, fmt.Errorf("openstack: GetSubnet not yet implemented")
}

func (r *realOpenStackAPI) DeleteSubnet(ctx context.Context, id string) error {
	return fmt.Errorf("openstack: DeleteSubnet not yet implemented")
}

// ── Security Group ────────────────────────────────────────────────────────────

func (r *realOpenStackAPI) CreateSecurityGroup(ctx context.Context, opts map[string]interface{}) (*SecurityGroupInfo, error) {
	return nil, fmt.Errorf("openstack: CreateSecurityGroup not yet implemented")
}

func (r *realOpenStackAPI) GetSecurityGroup(ctx context.Context, id string) (*SecurityGroupInfo, error) {
	return nil, fmt.Errorf("openstack: GetSecurityGroup not yet implemented")
}

func (r *realOpenStackAPI) DeleteSecurityGroup(ctx context.Context, id string) error {
	return fmt.Errorf("openstack: DeleteSecurityGroup not yet implemented")
}

// ── Volume ────────────────────────────────────────────────────────────────────

func (r *realOpenStackAPI) CreateVolume(ctx context.Context, opts map[string]interface{}) (*OSVolumeInfo, error) {
	return nil, fmt.Errorf("openstack: CreateVolume not yet implemented")
}

func (r *realOpenStackAPI) GetVolume(ctx context.Context, id string) (*OSVolumeInfo, error) {
	return nil, fmt.Errorf("openstack: GetVolume not yet implemented")
}

func (r *realOpenStackAPI) DeleteVolume(ctx context.Context, id string) error {
	return fmt.Errorf("openstack: DeleteVolume not yet implemented")
}
