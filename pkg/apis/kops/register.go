/*
Copyright 2019 The Kubernetes Authors.

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

package kops

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

//var GroupFactoryRegistry = make(announced.APIGroupFactoryRegistry)
//
//var Registry = registered.NewOrDie(os.Getenv("KOPS_API_VERSIONS"))
//
//var Scheme = runtime.NewScheme()
//
//var Codecs = serializer.NewCodecFactory(Scheme)
//
//// ParameterCodec handles versioning of objects that are converted to query parameters.
//var ParameterCodec = runtime.NewParameterCodec(Scheme)

// GroupName is the group name use in this package
const GroupName = "kops.k8s.io"

// SchemeGroupVersion is the group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}

//// Kind takes an unqualified kind and returns a Group qualified GroupKind
//func Kind(kind string) schema.GroupKind {
//	return SchemeGroupVersion.WithKind(kind).GroupKind()
//}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&Cluster{},
		&ClusterList{},
		&InstanceGroup{},
		&InstanceGroupList{},
		&Keyset{},
		&KeysetList{},
		&SSHCredential{},
		&SSHCredentialList{},
		&Host{},
		&HostList{},
	)
	// metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

func (obj *Cluster) GetObjectKind() schema.ObjectKind {
	return &obj.TypeMeta
}

func (obj *InstanceGroup) GetObjectKind() schema.ObjectKind {
	return &obj.TypeMeta
}

func (obj *Keyset) GetObjectKind() schema.ObjectKind {
	return &obj.TypeMeta
}

func (obj *SSHCredential) GetObjectKind() schema.ObjectKind {
	return &obj.TypeMeta
}
