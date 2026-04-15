package domain

import (
	"errors"
	"fmt"
	"slices"

	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
)

const (
	ResourceTypePackage = "package"
	ResourceTypeFile    = "file"
	ResourceTypeService = "service"
)

const (
	PackageStateInstalled = "installed"
	PackageStateAbsent    = "absent"
	FileStatePresent      = "present"
	FileStateAbsent       = "absent"
	ServiceStateRunning   = "running"
	ServiceStateStopped   = "stopped"
)

// Manifest is the domain representation of a declarative host configuration.
type Manifest struct {
	APIVersion string
	Kind       string
	Name       string
	Labels     map[string]string
	Target     TargetSpec
	Resources  []Resource
}

// TargetSpec defines the hosts selected by a manifest.
type TargetSpec struct {
	Hosts          []string
	SelectorLabels map[string]string
}

// Resource is the common behavior shared by all resource types.
type Resource interface {
	GetID() string
	GetType() string
	GetRequires() []string
	GetNotifies() []string
	ToAPI() *scmv1.ManifestResource
}

// PackageResource ensures a package is present or absent.
type PackageResource struct {
	ID       string
	Name     string
	State    string
	Requires []string
	Notifies []string
}

func (r PackageResource) GetID() string         { return r.ID }
func (r PackageResource) GetType() string       { return ResourceTypePackage }
func (r PackageResource) GetRequires() []string { return r.Requires }
func (r PackageResource) GetNotifies() []string { return r.Notifies }
func (r PackageResource) ToAPI() *scmv1.ManifestResource {
	return &scmv1.ManifestResource{
		ID:       r.ID,
		Type:     ResourceTypePackage,
		Name:     r.Name,
		State:    r.State,
		Requires: slices.Clone(r.Requires),
		Notifies: slices.Clone(r.Notifies),
	}
}

// FileResource ensures file contents and metadata are converged.
type FileResource struct {
	ID       string
	Path     string
	Content  string
	Mode     string
	Owner    string
	Group    string
	State    string
	Requires []string
	Notifies []string
}

func (r FileResource) GetID() string         { return r.ID }
func (r FileResource) GetType() string       { return ResourceTypeFile }
func (r FileResource) GetRequires() []string { return r.Requires }
func (r FileResource) GetNotifies() []string { return r.Notifies }
func (r FileResource) ToAPI() *scmv1.ManifestResource {
	return &scmv1.ManifestResource{
		ID:       r.ID,
		Type:     ResourceTypeFile,
		Path:     r.Path,
		Content:  r.Content,
		Mode:     r.Mode,
		Owner:    r.Owner,
		Group:    r.Group,
		State:    r.State,
		Requires: slices.Clone(r.Requires),
		Notifies: slices.Clone(r.Notifies),
	}
}

// ServiceResource ensures a system service is running or stopped.
type ServiceResource struct {
	ID       string
	Name     string
	State    string
	Enabled  *bool
	Requires []string
	Notifies []string
}

func (r ServiceResource) GetID() string         { return r.ID }
func (r ServiceResource) GetType() string       { return ResourceTypeService }
func (r ServiceResource) GetRequires() []string { return r.Requires }
func (r ServiceResource) GetNotifies() []string { return r.Notifies }
func (r ServiceResource) ToAPI() *scmv1.ManifestResource {
	enabled := false
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return &scmv1.ManifestResource{
		ID:       r.ID,
		Type:     ResourceTypeService,
		Name:     r.Name,
		State:    r.State,
		Enabled:  enabled,
		Requires: slices.Clone(r.Requires),
		Notifies: slices.Clone(r.Notifies),
	}
}

// CompiledManifest is the validated and ordered form used by the control plane
// and the agent runtime.
type CompiledManifest struct {
	Manifest
	OrderedResources []Resource
}

// ToAPI returns the stable transport representation of a compiled manifest.
func (m CompiledManifest) ToAPI() *scmv1.Manifest {
	resources := make([]*scmv1.ManifestResource, 0, len(m.Resources))
	for _, resource := range m.Resources {
		resources = append(resources, resource.ToAPI())
	}
	return &scmv1.Manifest{
		APIVersion: m.APIVersion,
		Kind:       m.Kind,
		Name:       m.Name,
		Labels:     cloneMap(m.Labels),
		Target: &scmv1.TargetSpec{
			Hosts: slices.Clone(m.Target.Hosts),
			Selector: &scmv1.HostSelector{
				MatchLabels: cloneMap(m.Target.SelectorLabels),
			},
		},
		Resources: resources,
	}
}

func cloneMap[K comparable, V any](input map[K]V) map[K]V {
	if len(input) == 0 {
		return nil
	}
	out := make(map[K]V, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

// Validate checks the manifest shape and builds a deterministic resource order.
func (m Manifest) Validate() (CompiledManifest, error) {
	if m.APIVersion != "scm/v1" {
		return CompiledManifest{}, fmt.Errorf("unsupported apiVersion %q", m.APIVersion)
	}
	if m.Kind != "Manifest" {
		return CompiledManifest{}, fmt.Errorf("unsupported kind %q", m.Kind)
	}
	if m.Name == "" {
		return CompiledManifest{}, errors.New("manifest metadata.name is required")
	}
	if len(m.Target.Hosts) == 0 && len(m.Target.SelectorLabels) == 0 {
		return CompiledManifest{}, errors.New("manifest target requires hosts, selector labels, or both")
	}
	if len(m.Resources) == 0 {
		return CompiledManifest{}, errors.New("manifest must define at least one resource")
	}

	byID := make(map[string]Resource, len(m.Resources))
	for _, resource := range m.Resources {
		if resource.GetID() == "" {
			return CompiledManifest{}, errors.New("resource id is required")
		}
		if _, exists := byID[resource.GetID()]; exists {
			return CompiledManifest{}, fmt.Errorf("resource id %q is duplicated", resource.GetID())
		}
		if err := validateResource(resource); err != nil {
			return CompiledManifest{}, fmt.Errorf("resource %q: %w", resource.GetID(), err)
		}
		byID[resource.GetID()] = resource
	}

	for _, resource := range m.Resources {
		for _, dep := range resource.GetRequires() {
			if _, ok := byID[dep]; !ok {
				return CompiledManifest{}, fmt.Errorf("resource %q requires unknown resource %q", resource.GetID(), dep)
			}
		}
		for _, dep := range resource.GetNotifies() {
			target, ok := byID[dep]
			if !ok {
				return CompiledManifest{}, fmt.Errorf("resource %q notifies unknown resource %q", resource.GetID(), dep)
			}
			if target.GetType() != ResourceTypeService {
				return CompiledManifest{}, fmt.Errorf("resource %q notifies non-service resource %q", resource.GetID(), dep)
			}
		}
	}

	ordered, err := topoSort(m.Resources)
	if err != nil {
		return CompiledManifest{}, err
	}

	return CompiledManifest{
		Manifest:         m,
		OrderedResources: ordered,
	}, nil
}

func validateResource(resource Resource) error {
	switch typed := resource.(type) {
	case PackageResource:
		if typed.Name == "" {
			return errors.New("package name is required")
		}
		if typed.State != PackageStateInstalled && typed.State != PackageStateAbsent {
			return fmt.Errorf("invalid package state %q", typed.State)
		}
	case FileResource:
		if typed.Path == "" {
			return errors.New("file path is required")
		}
		if typed.State != FileStatePresent && typed.State != FileStateAbsent {
			return fmt.Errorf("invalid file state %q", typed.State)
		}
		if typed.State == FileStatePresent && typed.Content == "" {
			return errors.New("file content is required for present files")
		}
	case ServiceResource:
		if typed.Name == "" {
			return errors.New("service name is required")
		}
		if typed.State != ServiceStateRunning && typed.State != ServiceStateStopped {
			return fmt.Errorf("invalid service state %q", typed.State)
		}
	default:
		return fmt.Errorf("unsupported resource type %T", resource)
	}
	return nil
}
