package foss

import (
	"context"
	"fmt"
	"net/http"
)

// realFlyAPI implements flyAPI using the live Fly.io Machines REST API.
type realFlyAPI struct {
	client  *http.Client
	baseURL string
	token   string
}

func newRealFlyAPI(client *http.Client, baseURL, token string) *realFlyAPI {
	return &realFlyAPI{client: client, baseURL: baseURL, token: token}
}

// ── App ───────────────────────────────────────────────────────────────────────

func (r *realFlyAPI) CreateApp(ctx context.Context, name, org string) (*AppInfo, error) {
	return nil, fmt.Errorf("fly: CreateApp not yet implemented")
}

func (r *realFlyAPI) GetApp(ctx context.Context, name string) (*AppInfo, error) {
	return nil, fmt.Errorf("fly: GetApp not yet implemented")
}

func (r *realFlyAPI) DeleteApp(ctx context.Context, name string) error {
	return fmt.Errorf("fly: DeleteApp not yet implemented")
}

// ── Machine ───────────────────────────────────────────────────────────────────

func (r *realFlyAPI) CreateMachine(ctx context.Context, appName string, config map[string]interface{}) (*MachineInfo, error) {
	return nil, fmt.Errorf("fly: CreateMachine not yet implemented")
}

func (r *realFlyAPI) GetMachine(ctx context.Context, appName, machineID string) (*MachineInfo, error) {
	return nil, fmt.Errorf("fly: GetMachine not yet implemented")
}

func (r *realFlyAPI) UpdateMachine(ctx context.Context, appName, machineID string, config map[string]interface{}) (*MachineInfo, error) {
	return nil, fmt.Errorf("fly: UpdateMachine not yet implemented")
}

func (r *realFlyAPI) DeleteMachine(ctx context.Context, appName, machineID string) error {
	return fmt.Errorf("fly: DeleteMachine not yet implemented")
}

// ── Volume ────────────────────────────────────────────────────────────────────

func (r *realFlyAPI) CreateVolume(ctx context.Context, appName string, config map[string]interface{}) (*FlyVolumeInfo, error) {
	return nil, fmt.Errorf("fly: CreateVolume not yet implemented")
}

func (r *realFlyAPI) GetVolume(ctx context.Context, appName, volumeID string) (*FlyVolumeInfo, error) {
	return nil, fmt.Errorf("fly: GetVolume not yet implemented")
}

func (r *realFlyAPI) DeleteVolume(ctx context.Context, appName, volumeID string) error {
	return fmt.Errorf("fly: DeleteVolume not yet implemented")
}

// ── Secrets ───────────────────────────────────────────────────────────────────

func (r *realFlyAPI) SetSecrets(ctx context.Context, appName string, secrets map[string]string) error {
	return fmt.Errorf("fly: SetSecrets not yet implemented")
}

func (r *realFlyAPI) UnsetSecret(ctx context.Context, appName, key string) error {
	return fmt.Errorf("fly: UnsetSecret not yet implemented")
}

func (r *realFlyAPI) ListSecrets(ctx context.Context, appName string) ([]SecretInfo, error) {
	return nil, fmt.Errorf("fly: ListSecrets not yet implemented")
}
