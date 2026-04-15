package infra

import (
	"fmt"

	"github.com/alexisjcarr/scm/internal/manifest/domain"
	"gopkg.in/yaml.v3"
)

type manifestDocument struct {
	APIVersion string             `yaml:"apiVersion"`
	Kind       string             `yaml:"kind"`
	Metadata   metadataDocument   `yaml:"metadata"`
	Target     targetDocument     `yaml:"target"`
	Resources  []resourceDocument `yaml:"resources"`
}

type metadataDocument struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}

type targetDocument struct {
	Hosts    []string         `yaml:"hosts"`
	Selector selectorDocument `yaml:"selector"`
}

type selectorDocument struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

type resourceDocument struct {
	ID       string   `yaml:"id"`
	Type     string   `yaml:"type"`
	Name     string   `yaml:"name"`
	Path     string   `yaml:"path"`
	Content  string   `yaml:"content"`
	Mode     string   `yaml:"mode"`
	Owner    string   `yaml:"owner"`
	Group    string   `yaml:"group"`
	State    string   `yaml:"state"`
	Enabled  *bool    `yaml:"enabled"`
	Requires []string `yaml:"requires"`
	Notifies []string `yaml:"notifies"`
}

// DecodeManifest converts YAML bytes into the manifest domain model.
func DecodeManifest(data []byte) (domain.Manifest, error) {
	var doc manifestDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return domain.Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}

	resources := make([]domain.Resource, 0, len(doc.Resources))
	for _, resource := range doc.Resources {
		switch resource.Type {
		case domain.ResourceTypePackage:
			resources = append(resources, domain.PackageResource{
				ID:       resource.ID,
				Name:     resource.Name,
				State:    resource.State,
				Requires: resource.Requires,
				Notifies: resource.Notifies,
			})
		case domain.ResourceTypeFile:
			resources = append(resources, domain.FileResource{
				ID:       resource.ID,
				Path:     resource.Path,
				Content:  resource.Content,
				Mode:     resource.Mode,
				Owner:    resource.Owner,
				Group:    resource.Group,
				State:    resource.State,
				Requires: resource.Requires,
				Notifies: resource.Notifies,
			})
		case domain.ResourceTypeService:
			resources = append(resources, domain.ServiceResource{
				ID:       resource.ID,
				Name:     resource.Name,
				State:    resource.State,
				Enabled:  resource.Enabled,
				Requires: resource.Requires,
				Notifies: resource.Notifies,
			})
		default:
			return domain.Manifest{}, fmt.Errorf("resource %q: unsupported type %q", resource.ID, resource.Type)
		}
	}

	return domain.Manifest{
		APIVersion: doc.APIVersion,
		Kind:       doc.Kind,
		Name:       doc.Metadata.Name,
		Labels:     doc.Metadata.Labels,
		Target: domain.TargetSpec{
			Hosts:          doc.Target.Hosts,
			SelectorLabels: doc.Target.Selector.MatchLabels,
		},
		Resources: resources,
	}, nil
}
