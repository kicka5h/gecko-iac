// Package k8s provides the Gecko Kubernetes provider.
// Manages Kubernetes resources across any conformant cluster (k3s, k0s, Talos, etc.)
package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/core"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

// Provider implements core.Provider for Kubernetes clusters
type Provider struct {
	kubeconfig string
	namespace  string
	context_   string
	client     dynamic.Interface
}

// gvrTable maps gecko resource types to Kubernetes GroupVersionResource
var gvrTable = map[core.ResourceType]schema.GroupVersionResource{
	"k8s:namespace":               {Group: "", Version: "v1", Resource: "namespaces"},
	"k8s:deployment":              {Group: "apps", Version: "v1", Resource: "deployments"},
	"k8s:statefulset":             {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"k8s:daemonset":               {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"k8s:service":                 {Group: "", Version: "v1", Resource: "services"},
	"k8s:configmap":               {Group: "", Version: "v1", Resource: "configmaps"},
	"k8s:secret":                  {Group: "", Version: "v1", Resource: "secrets"},
	"k8s:persistentvolumeclaim":   {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	"k8s:persistentvolume":        {Group: "", Version: "v1", Resource: "persistentvolumes"},
	"k8s:storageclass":            {Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses"},
	"k8s:serviceaccount":          {Group: "", Version: "v1", Resource: "serviceaccounts"},
	"k8s:ingress":                 {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"k8s:networkpolicy":           {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
	"k8s:clusterrole":             {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
	"k8s:clusterrolebinding":      {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
	"k8s:role":                    {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
	"k8s:rolebinding":             {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
	"k8s:horizontalpodautoscaler": {Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
	"k8s:poddisruptionbudget":     {Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"},
	"k8s:job":                     {Group: "batch", Version: "v1", Resource: "jobs"},
	"k8s:cronjob":                 {Group: "batch", Version: "v1", Resource: "cronjobs"},
}

// clusterScopedTypes are resources that are not namespaced
var clusterScopedTypes = map[core.ResourceType]bool{
	"k8s:namespace":          true,
	"k8s:persistentvolume":   true,
	"k8s:storageclass":       true,
	"k8s:clusterrole":        true,
	"k8s:clusterrolebinding": true,
}

// NewProvider creates a new Kubernetes provider with optional config overrides
func NewProvider(config map[string]interface{}) *Provider {
	p := &Provider{
		kubeconfig: "~/.kube/config",
		namespace:  "default",
	}
	if config != nil {
		if v, ok := config["kubeconfig"].(string); ok {
			p.kubeconfig = v
		}
		if v, ok := config["namespace"].(string); ok {
			p.namespace = v
		}
		if v, ok := config["context"].(string); ok {
			p.context_ = v
		}
	}
	return p
}

func (p *Provider) Name() string    { return "k8s" }
func (p *Provider) Version() string { return "0.1.0" }

func (p *Provider) SupportedTypes() []core.ResourceType {
	types := make([]core.ResourceType, 0, len(gvrTable))
	for t := range gvrTable {
		types = append(types, t)
	}
	return types
}

// Configure loads the kubeconfig and builds the dynamic client
func (p *Provider) Configure(ctx context.Context, config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["kubeconfig"].(string); ok {
			p.kubeconfig = v
		}
		if v, ok := config["context"].(string); ok {
			p.context_ = v
		}
	}

	kubeconfigPath := p.kubeconfig

	// Expand ~ to home directory
	if strings.HasPrefix(kubeconfigPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		kubeconfigPath = filepath.Join(home, kubeconfigPath[2:])
	}

	// Fall back to KUBECONFIG env or default path
	if kubeconfigPath == "" {
		if env := os.Getenv("KUBECONFIG"); env != "" {
			kubeconfigPath = env
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("cannot determine home directory: %w", err)
			}
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfigPath

	overrides := &clientcmd.ConfigOverrides{}
	if p.context_ != "" {
		overrides.CurrentContext = p.context_
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	p.client = dynClient
	return nil
}

// Validate checks that resource args are valid for this provider
func (p *Provider) Validate(ctx context.Context, args core.ResourceArgs) error {
	if args.Name == "" {
		return fmt.Errorf("k8s resource name cannot be empty")
	}
	if _, ok := gvrTable[args.Type]; !ok {
		return fmt.Errorf("unsupported resource type: %s", args.Type)
	}
	return nil
}

// resourceClient returns the appropriate dynamic resource client for the given type/namespace
func (p *Provider) resourceClient(resourceType core.ResourceType, ns string) (dynamic.ResourceInterface, error) {
	gvr, ok := gvrTable[resourceType]
	if !ok {
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
	if p.client == nil {
		return nil, fmt.Errorf("provider not configured: call Configure first")
	}
	if clusterScopedTypes[resourceType] {
		return p.client.Resource(gvr), nil
	}
	if ns == "" {
		ns = p.namespace
	}
	return p.client.Resource(gvr).Namespace(ns), nil
}

// Create provisions a new Kubernetes resource
func (p *Provider) Create(ctx context.Context, args core.ResourceArgs) (*core.ResourceState, error) {
	manifest := buildManifest(args.Type, args.Name, args.Inputs)
	obj := &unstructured.Unstructured{Object: manifest}

	ns := getStringInput(args.Inputs, "namespace", p.namespace)
	rc, err := p.resourceClient(args.Type, ns)
	if err != nil {
		return nil, err
	}

	created, err := rc.Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create %s %s: %w", args.Type, args.Name, err)
	}

	externalID := externalIDFromObject(created, args.Type)
	return &core.ResourceState{
		ID:         core.ResourceID(fmt.Sprintf("%s::%s", args.Type, args.Name)),
		Type:       args.Type,
		Name:       args.Name,
		Status:     core.StatusRunning,
		Inputs:     args.Inputs,
		Outputs:    deriveOutputs(args.Type, args.Name, created),
		Tags:       args.Tags,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		ProviderID: "k8s",
		ExternalID: externalID,
	}, nil
}

// Read retrieves the current state of a Kubernetes resource
func (p *Provider) Read(ctx context.Context, id core.ResourceID, externalID string) (*core.ResourceState, error) {
	// Parse type from ResourceID ("k8s:deployment::nginx" → "k8s:deployment")
	parts := strings.SplitN(string(id), "::", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid resource ID: %s", id)
	}
	resourceType := core.ResourceType(parts[0])
	name := parts[1]

	ns, resName := parseExternalID(externalID, name)

	rc, err := p.resourceClient(resourceType, ns)
	if err != nil {
		return nil, err
	}

	obj, err := rc.Get(ctx, resName, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get %s %s: %w", resourceType, resName, err)
	}

	inputs := inputsFromObject(obj)
	return &core.ResourceState{
		ID:         id,
		Type:       resourceType,
		Name:       resName,
		Status:     core.StatusRunning,
		Inputs:     inputs,
		Outputs:    deriveOutputs(resourceType, resName, obj),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		ProviderID: "k8s",
		ExternalID: externalID,
	}, nil
}

// Update applies changes to an existing Kubernetes resource
func (p *Provider) Update(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.ResourceState, error) {
	ns := getStringInput(desired.Inputs, "namespace", p.namespace)
	rc, err := p.resourceClient(desired.Type, ns)
	if err != nil {
		return nil, err
	}

	_, resName := parseExternalID(current.ExternalID, desired.Name)

	// GET the current object to obtain resourceVersion
	existing, err := rc.Get(ctx, resName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get existing resource before update: %w", err)
	}

	newManifest := buildManifest(desired.Type, desired.Name, desired.Inputs)
	newObj := &unstructured.Unstructured{Object: newManifest}
	// Preserve resourceVersion for optimistic concurrency
	newObj.SetResourceVersion(existing.GetResourceVersion())

	updated, err := rc.Update(ctx, newObj, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to update %s %s: %w", desired.Type, desired.Name, err)
	}

	return &core.ResourceState{
		ID:         current.ID,
		Type:       desired.Type,
		Name:       desired.Name,
		Status:     core.StatusRunning,
		Inputs:     desired.Inputs,
		Outputs:    deriveOutputs(desired.Type, desired.Name, updated),
		Tags:       desired.Tags,
		CreatedAt:  current.CreatedAt,
		UpdatedAt:  time.Now(),
		ProviderID: "k8s",
		ExternalID: current.ExternalID,
	}, nil
}

// Delete removes a Kubernetes resource
func (p *Provider) Delete(ctx context.Context, state *core.ResourceState) error {
	ns := getStringInput(state.Inputs, "namespace", p.namespace)
	rc, err := p.resourceClient(state.Type, ns)
	if err != nil {
		return err
	}

	_, resName := parseExternalID(state.ExternalID, state.Name)

	gracePeriod := int64(0)
	propagation := metav1.DeletePropagationBackground
	deleteOpts := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
		PropagationPolicy:  &propagation,
	}

	err = rc.Delete(ctx, resName, deleteOpts)
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete %s %s: %w", state.Type, resName, err)
	}
	return nil
}

// Import reads an existing resource into Gecko state
func (p *Provider) Import(ctx context.Context, resourceType core.ResourceType, externalID string) (*core.ResourceState, error) {
	ns, resName := parseExternalID(externalID, "")
	if resName == "" {
		return nil, fmt.Errorf("invalid externalID for import: %s (expected namespace/name)", externalID)
	}

	rc, err := p.resourceClient(resourceType, ns)
	if err != nil {
		return nil, err
	}

	obj, err := rc.Get(ctx, resName, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, fmt.Errorf("resource %s/%s not found in cluster", resourceType, externalID)
		}
		return nil, fmt.Errorf("failed to import %s %s: %w", resourceType, externalID, err)
	}

	inputs := inputsFromObject(obj)
	id := core.ResourceID(fmt.Sprintf("%s::%s", resourceType, resName))
	return &core.ResourceState{
		ID:         id,
		Type:       resourceType,
		Name:       resName,
		Status:     core.StatusRunning,
		Inputs:     inputs,
		Outputs:    deriveOutputs(resourceType, resName, obj),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		ProviderID: "k8s",
		ExternalID: externalID,
	}, nil
}

// Diff computes what would change between current state and desired args
func (p *Provider) Diff(ctx context.Context, current *core.ResourceState, desired core.ResourceArgs) (*core.Diff, error) {
	if current == nil {
		return &core.Diff{
			ResourceID: core.ResourceID(fmt.Sprintf("%s::%s", desired.Type, desired.Name)),
			Kind:       core.ChangeAdd,
		}, nil
	}

	var changes []core.FieldChange

	switch desired.Type {
	case "k8s:deployment":
		changes = append(changes, compareField("image", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("replicas", current.Inputs, desired.Inputs)...)

	case "k8s:service":
		changes = append(changes, compareField("port", current.Inputs, desired.Inputs)...)
		changes = append(changes, compareField("type", current.Inputs, desired.Inputs)...)

	case "k8s:configmap":
		// Compare data keys
		currentData, _ := current.Inputs["data"].(map[string]interface{})
		desiredData, _ := desired.Inputs["data"].(map[string]interface{})
		for k, dv := range desiredData {
			cv, exists := currentData[k]
			if !exists {
				changes = append(changes, core.FieldChange{Field: "data." + k, Kind: core.ChangeAdd, NewValue: dv})
			} else if fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", dv) {
				changes = append(changes, core.FieldChange{Field: "data." + k, Kind: core.ChangeUpdate, OldValue: cv, NewValue: dv})
			}
		}
		for k, cv := range currentData {
			if _, exists := desiredData[k]; !exists {
				changes = append(changes, core.FieldChange{Field: "data." + k, Kind: core.ChangeDelete, OldValue: cv})
			}
		}

	default:
		// Generic: compare all inputs
		for k, dv := range desired.Inputs {
			cv, exists := current.Inputs[k]
			if !exists {
				changes = append(changes, core.FieldChange{Field: k, Kind: core.ChangeAdd, NewValue: dv})
			} else if fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", dv) {
				changes = append(changes, core.FieldChange{Field: k, Kind: core.ChangeUpdate, OldValue: cv, NewValue: dv})
			}
		}
		for k, cv := range current.Inputs {
			if _, exists := desired.Inputs[k]; !exists {
				changes = append(changes, core.FieldChange{Field: k, Kind: core.ChangeDelete, OldValue: cv})
			}
		}
	}

	kind := core.ChangeNoOp
	if len(changes) > 0 {
		kind = core.ChangeUpdate
		// Immutable fields require replace
		for _, c := range changes {
			if c.Field == "namespace" {
				return &core.Diff{
					ResourceID:      current.ID,
					Kind:            core.ChangeReplace,
					Changes:         changes,
					RequiresReplace: true,
				}, nil
			}
		}
	}

	return &core.Diff{
		ResourceID: current.ID,
		Kind:       kind,
		Changes:    changes,
	}, nil
}

// ─── Manifest Builders ────────────────────────────────────────────────────────

// buildManifest converts simplified Gecko inputs to a full unstructured k8s object
func buildManifest(resourceType core.ResourceType, name string, inputs core.Inputs) map[string]interface{} {
	switch resourceType {
	case "k8s:namespace":
		return buildNamespace(name, inputs)
	case "k8s:deployment":
		return buildDeployment(name, inputs)
	case "k8s:service":
		return buildService(name, inputs)
	case "k8s:configmap":
		return buildConfigMap(name, inputs)
	case "k8s:secret":
		return buildSecret(name, inputs)
	case "k8s:persistentvolumeclaim":
		return buildPVC(name, inputs)
	default:
		return buildGeneric(resourceType, name, inputs)
	}
}

func buildNamespace(name string, inputs core.Inputs) map[string]interface{} {
	meta := map[string]interface{}{"name": name}
	if labels, ok := inputs["labels"].(map[string]interface{}); ok {
		meta["labels"] = labels
	}
	if annotations, ok := inputs["annotations"].(map[string]interface{}); ok {
		meta["annotations"] = annotations
	}
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   meta,
	}
}

func buildDeployment(name string, inputs core.Inputs) map[string]interface{} {
	ns := getStringInput(inputs, "namespace", "default")
	image := getStringInput(inputs, "image", "")
	replicas := int64(1)
	if r, ok := inputs["replicas"]; ok {
		switch v := r.(type) {
		case int:
			replicas = int64(v)
		case int64:
			replicas = v
		case float64:
			replicas = int64(v)
		}
	}
	imagePullPolicy := getStringInput(inputs, "image_pull_policy", "IfNotPresent")
	serviceAccount := getStringInput(inputs, "service_account", "")

	labels := map[string]interface{}{"app": name}
	if l, ok := inputs["labels"].(map[string]interface{}); ok {
		for k, v := range l {
			labels[k] = v
		}
	}

	container := map[string]interface{}{
		"name":            name,
		"image":           image,
		"imagePullPolicy": imagePullPolicy,
	}

	// Ports
	if ports, ok := inputs["ports"].([]interface{}); ok {
		containerPorts := make([]interface{}, 0, len(ports))
		for _, p := range ports {
			containerPorts = append(containerPorts, map[string]interface{}{"containerPort": p})
		}
		container["ports"] = containerPorts
	}

	// Env vars
	if env, ok := inputs["env"].(map[string]interface{}); ok {
		envVars := make([]interface{}, 0, len(env))
		for k, v := range env {
			envVars = append(envVars, map[string]interface{}{"name": k, "value": fmt.Sprintf("%v", v)})
		}
		container["env"] = envVars
	}

	// Command and args
	if cmd, ok := inputs["command"].([]interface{}); ok {
		container["command"] = cmd
	}
	if args, ok := inputs["args"].([]interface{}); ok {
		container["args"] = args
	}

	// Resources
	if res, ok := inputs["resources"].(map[string]interface{}); ok {
		container["resources"] = res
	}

	podSpec := map[string]interface{}{
		"containers": []interface{}{container},
	}
	if serviceAccount != "" {
		podSpec["serviceAccountName"] = serviceAccount
	}

	meta := map[string]interface{}{
		"name":      name,
		"namespace": ns,
		"labels":    labels,
	}

	return map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   meta,
		"spec": map[string]interface{}{
			"replicas": replicas,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"app": name},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{"labels": labels},
				"spec":     podSpec,
			},
		},
	}
}

func buildService(name string, inputs core.Inputs) map[string]interface{} {
	ns := getStringInput(inputs, "namespace", "default")
	svcType := getStringInput(inputs, "type", "ClusterIP")

	port := int64(80)
	if p, ok := inputs["port"]; ok {
		switch v := p.(type) {
		case int:
			port = int64(v)
		case int64:
			port = v
		case float64:
			port = int64(v)
		}
	}
	targetPort := port
	if tp, ok := inputs["target_port"]; ok {
		switch v := tp.(type) {
		case int:
			targetPort = int64(v)
		case int64:
			targetPort = v
		case float64:
			targetPort = int64(v)
		}
	}

	selector := map[string]interface{}{"app": name}
	if s, ok := inputs["selector"].(map[string]interface{}); ok {
		selector = s
	}

	labels := map[string]interface{}{"app": name}
	if l, ok := inputs["labels"].(map[string]interface{}); ok {
		for k, v := range l {
			labels[k] = v
		}
	}

	portSpec := map[string]interface{}{
		"port":       port,
		"targetPort": targetPort,
		"protocol":   "TCP",
	}
	if svcType == "NodePort" {
		if np, ok := inputs["node_port"]; ok {
			switch v := np.(type) {
			case int:
				portSpec["nodePort"] = int64(v)
			case int64:
				portSpec["nodePort"] = v
			case float64:
				portSpec["nodePort"] = int64(v)
			}
		}
	}

	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": ns,
			"labels":    labels,
		},
		"spec": map[string]interface{}{
			"type":     svcType,
			"selector": selector,
			"ports":    []interface{}{portSpec},
		},
	}
}

func buildConfigMap(name string, inputs core.Inputs) map[string]interface{} {
	ns := getStringInput(inputs, "namespace", "default")
	data := map[string]interface{}{}
	if d, ok := inputs["data"].(map[string]interface{}); ok {
		data = d
	}
	meta := map[string]interface{}{
		"name":      name,
		"namespace": ns,
	}
	if labels, ok := inputs["labels"].(map[string]interface{}); ok {
		meta["labels"] = labels
	}
	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   meta,
		"data":       data,
	}
}

func buildSecret(name string, inputs core.Inputs) map[string]interface{} {
	ns := getStringInput(inputs, "namespace", "default")
	secretType := getStringInput(inputs, "type", "Opaque")

	meta := map[string]interface{}{
		"name":      name,
		"namespace": ns,
	}
	if labels, ok := inputs["labels"].(map[string]interface{}); ok {
		meta["labels"] = labels
	}

	manifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   meta,
		"type":       secretType,
	}
	if data, ok := inputs["data"].(map[string]interface{}); ok {
		manifest["data"] = data
	}
	if stringData, ok := inputs["string_data"].(map[string]interface{}); ok {
		manifest["stringData"] = stringData
	}
	return manifest
}

func buildPVC(name string, inputs core.Inputs) map[string]interface{} {
	ns := getStringInput(inputs, "namespace", "default")
	storage := getStringInput(inputs, "storage", "1Gi")

	accessModes := []interface{}{"ReadWriteOnce"}
	if modes, ok := inputs["access_modes"].([]interface{}); ok {
		accessModes = modes
	}

	spec := map[string]interface{}{
		"accessModes": accessModes,
		"resources": map[string]interface{}{
			"requests": map[string]interface{}{
				"storage": storage,
			},
		},
	}
	if sc := getStringInput(inputs, "storage_class", ""); sc != "" {
		spec["storageClassName"] = sc
	}

	meta := map[string]interface{}{
		"name":      name,
		"namespace": ns,
	}
	if labels, ok := inputs["labels"].(map[string]interface{}); ok {
		meta["labels"] = labels
	}

	return map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "PersistentVolumeClaim",
		"metadata":   meta,
		"spec":       spec,
	}
}

func buildGeneric(resourceType core.ResourceType, name string, inputs core.Inputs) map[string]interface{} {
	gvr, ok := gvrTable[resourceType]
	ns := getStringInput(inputs, "namespace", "default")
	meta := map[string]interface{}{
		"name":      name,
		"namespace": ns,
	}
	if labels, ok := inputs["labels"].(map[string]interface{}); ok {
		meta["labels"] = labels
	}

	manifest := map[string]interface{}{
		"metadata": meta,
	}

	if ok {
		group := gvr.Group
		if group == "" {
			manifest["apiVersion"] = gvr.Version
		} else {
			manifest["apiVersion"] = group + "/" + gvr.Version
		}
		// Derive kind from resource name (e.g. "deployments" → "Deployment")
		manifest["kind"] = resourceToKind(gvr.Resource)
	}

	if spec, ok := inputs["spec"].(map[string]interface{}); ok {
		manifest["spec"] = spec
	}
	return manifest
}

// resourceToKind converts a plural resource name to its Kind (best-effort)
func resourceToKind(resource string) string {
	// Strip trailing 's' and capitalize
	s := resource
	if strings.HasSuffix(s, "ies") {
		s = s[:len(s)-3] + "y"
	} else if strings.HasSuffix(s, "ses") {
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "s") {
		s = s[:len(s)-1]
	}
	if len(s) == 0 {
		return resource
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ─── Output Derivation ────────────────────────────────────────────────────────

func deriveOutputs(resourceType core.ResourceType, name string, obj *unstructured.Unstructured) core.Outputs {
	outputs := core.Outputs{}
	if obj == nil {
		return outputs
	}

	switch resourceType {
	case "k8s:service":
		spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
		if clusterIP, ok := spec["clusterIP"].(string); ok {
			outputs["cluster_ip"] = clusterIP
		}
		if ports, ok := spec["ports"].([]interface{}); ok && len(ports) > 0 {
			if p, ok := ports[0].(map[string]interface{}); ok {
				outputs["port"] = p["port"]
			}
		}

	case "k8s:deployment":
		status, _, _ := unstructured.NestedMap(obj.Object, "status")
		if rr, ok := status["readyReplicas"]; ok {
			outputs["ready_replicas"] = rr
		}
		if ar, ok := status["availableReplicas"]; ok {
			outputs["available_replicas"] = ar
		}

	case "k8s:persistentvolumeclaim":
		status, _, _ := unstructured.NestedMap(obj.Object, "status")
		if phase, ok := status["phase"].(string); ok {
			outputs["status"] = phase
		}
		if vn, ok := status["volumeName"].(string); ok {
			outputs["volume_name"] = vn
		} else {
			outputs["volume_name"] = fmt.Sprintf("pvc-%s", name)
		}
	}

	return outputs
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func externalIDFromObject(obj *unstructured.Unstructured, resourceType core.ResourceType) string {
	if clusterScopedTypes[resourceType] {
		return obj.GetName()
	}
	ns := obj.GetNamespace()
	if ns == "" {
		return obj.GetName()
	}
	return ns + "/" + obj.GetName()
}

func parseExternalID(externalID, fallbackName string) (ns, name string) {
	parts := strings.SplitN(externalID, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// cluster-scoped or bare name
	if externalID != "" {
		return "", externalID
	}
	return "", fallbackName
}

func inputsFromObject(obj *unstructured.Unstructured) core.Inputs {
	// Serialize the full object to get inputs, but only store the spec portion
	inputs := core.Inputs{}
	if spec, ok := obj.Object["spec"].(map[string]interface{}); ok {
		// Convert to JSON-safe map
		data, err := json.Marshal(spec)
		if err == nil {
			var m map[string]interface{}
			if err := json.Unmarshal(data, &m); err == nil {
				inputs["spec"] = m
			}
		}
	}
	if ns := obj.GetNamespace(); ns != "" {
		inputs["namespace"] = ns
	}
	return inputs
}

func compareField(field string, current, desired core.Inputs) []core.FieldChange {
	dv, desiredHas := desired[field]
	cv, currentHas := current[field]

	if desiredHas && !currentHas {
		return []core.FieldChange{{Field: field, Kind: core.ChangeAdd, NewValue: dv}}
	}
	if !desiredHas && currentHas {
		return []core.FieldChange{{Field: field, Kind: core.ChangeDelete, OldValue: cv}}
	}
	if desiredHas && currentHas && fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", dv) {
		return []core.FieldChange{{Field: field, Kind: core.ChangeUpdate, OldValue: cv, NewValue: dv}}
	}
	return nil
}

func getStringInput(inputs core.Inputs, key, def string) string {
	if v, ok := inputs[key].(string); ok {
		return v
	}
	return def
}
