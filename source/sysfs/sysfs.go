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
		if strings.HasPrefix(attr, "/sys") {
			attr = attr[4:]
		}
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
	if(len(name) > 55){
		//truncate the key by stripping off direcory names from the start
		start := len(name)-55

		offset := strings.Index(name[start:], ".") + 1

		//klog.InfoS("truncating key", "key", name, "start", start, "offset", offset, "name", name[start+offset:], "trunc", name[start:])
		name = name[start+offset:]
	}
	return name
}

func convertToLabel(str string) string {

	if str == "" {
		return str
	}
	// strip characters that cant make labels, then trucate to 62 chars (max label len)
	startWithRe := regexp.MustCompile(`^[^-A-Za-z0-9]+`)
	endsWithRe := regexp.MustCompile(`[^-A-Za-z0-9]+$`)
	inStringRe := regexp.MustCompile(`[^-A-Za-z0-9_.]+`)

	value := startWithRe.ReplaceAllString(str, "")
	value = inStringRe.ReplaceAllString(value, "_")
	if len(value) > 62 {
		value = value[:62]
	}
	value = endsWithRe.ReplaceAllString(value, "")

	return value
}


func readSingleParameter(attrPath string) (string, error){

	fileInfo, err := os.Stat(attrPath)
	if err != nil {
		return "", fmt.Errorf("failed to read parameter %s: %v", attrPath, err)
	}

	if fileInfo.IsDir() {
		// is a directory, so create an empty label
		return "", nil
	}

	// it exists and its a file
	data, err := os.ReadFile(attrPath)
	if err != nil {
		// if we get "Permission Denied" create an empty label to show it does exist
		if os.IsPermission(err){
			return "", nil
		}
		return "", fmt.Errorf("failed to read parameter %s: %v", attrPath, err)
	}

	// its a file and we've read the contents for the label value, so need to sanitize it
	return convertToLabel(string(data)), nil

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
