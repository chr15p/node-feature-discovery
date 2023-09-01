/*
Copyright 2018-2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sysfs 

import (
	"fmt"
	"strings"
	"os"
	"regexp"
	"path/filepath"

	"k8s.io/klog/v2"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"sigs.k8s.io/node-feature-discovery/source"
	"sigs.k8s.io/node-feature-discovery/pkg/utils/hostpath"
)

// Name of this feature source
const (
	Name = "sysfs"
	sysfsFeature = "attribute"
)

// Config holds the configuration parameters of this source.
type Config struct {
	SysfsWhitelist []string `json:"sysfsWhitelist,omitempty"`
}

// newDefaultConfig returns a new config with pre-populated defaults
func newDefaultConfig() *Config {
	return &Config{
		SysfsWhitelist: []string{""},
	}
}

// sysfsSource implements the FeatureSource, LabelSource and ConfigurableSource interfaces.
type sysfsSource struct {
	config   *Config
	features *nfdv1alpha1.Features
}

// Singleton source instance
var (
	src                           = sysfsSource{config: newDefaultConfig()}
	_   source.FeatureSource      = &src
	_   source.LabelSource        = &src
	_   source.ConfigurableSource = &src
)


// Name returns the name of the feature source
func (s *sysfsSource) Name() string { return Name }

// NewConfig method of the LabelSource interface
func (s *sysfsSource) NewConfig() source.Config { return newDefaultConfig() }

// GetConfig method of the LabelSource interface
func (s *sysfsSource) GetConfig() source.Config { return s.config }

// SetConfig method of the LabelSource interface
func (s *sysfsSource) SetConfig(conf source.Config) {
	switch v := conf.(type) {
	case *Config:
		s.config = v
	default:
		panic(fmt.Sprintf("invalid config type: %T", conf))
	}
}

// Priority method of the LabelSource interface
func (s *sysfsSource) Priority() int { return 0 }

// GetLabels method of the LabelSource interface
func (s *sysfsSource) GetLabels() (source.FeatureLabels, error) {
	labels := source.FeatureLabels{}
	features := s.GetFeatures()

	for key, value := range  features.Attributes[sysfsFeature].Elements {
		labels[key] = value
	}

	return labels, nil
}

// Discover method of the FeatureSource interface
func (s *sysfsSource) Discover() error {
	s.features = nfdv1alpha1.NewFeatures()
	// Get node name
	s.features.Attributes[sysfsFeature] = nfdv1alpha1.NewAttributeFeatures(nil)


	for _, attr := range s.config.SysfsWhitelist {
		// if provide with a relative path make it absolute
		if ! filepath.IsAbs(attr) {
			attr = filepath.Join("/", attr)	
		}

		attr = filepath.Clean(attr)
		sysfsBasePath := hostpath.SysfsDir.Path(attr)

		//klog.InfoS("reading attr", "attr", attr, "sysfsBasePath", sysfsBasePath)
		paramVal, err := readSingleParameter(sysfsBasePath)
		if err != nil {
			klog.InfoS("reading parameter failed", "parameter", attr, "error", err)
			continue
		}
		name := buildAttributeName(attr)
		//klog.InfoS("read attr", "name", name, "value", paramVal)

		s.features.Attributes[sysfsFeature].Elements[name] = paramVal
	}

	return nil
}

func buildAttributeName(attr string) string {
	name := strings.Replace(attr, "/", ".", -1)[1:]

	//if its too long strip off the excess chars from the start
	if(len(name)> 64){
		name = name[len(name)-64:]
	}
	return name
}

func readSingleParameter(attrPath string) (string, error){
	data, err := os.ReadFile(attrPath)
	if err != nil {
		return "", fmt.Errorf("failed to read parameter %s: %v", attrPath, err)
	}

	// strip characters that cant make labels, then trucate to 62 chars (max label len)
	// so we actually get 62 chars (truncateing then stripping can leave a lot less)
	startWithRe := regexp.MustCompile(`^[^-A-Za-z0-9_.]+`)
	inStringRe := regexp.MustCompile(`[^-A-Za-z0-9_.]+`)

	value := startWithRe.ReplaceAllString(string(data), "")
	value = inStringRe.ReplaceAllString(value, "_")
	if len(value) > 62 {
		return value[:62], nil
	}
	return value, nil
}

// GetFeatures method of the FeatureSource Interface
func (s *sysfsSource) GetFeatures() *nfdv1alpha1.Features {
	if s.features == nil {
		s.features = nfdv1alpha1.NewFeatures()
	}
	return s.features
}

func init() {
	source.Register(&src)
}
