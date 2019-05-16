// +build !ignore_autogenerated

/*
Copyright 2019 TAKAISHI Ryo.

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
// Code generated by main. DO NOT EDIT.

package v1beta1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIP) DeepCopyInto(out *FloatingIP) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIP.
func (in *FloatingIP) DeepCopy() *FloatingIP {
	if in == nil {
		return nil
	}
	out := new(FloatingIP)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FloatingIP) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIPAssociate) DeepCopyInto(out *FloatingIPAssociate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIPAssociate.
func (in *FloatingIPAssociate) DeepCopy() *FloatingIPAssociate {
	if in == nil {
		return nil
	}
	out := new(FloatingIPAssociate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FloatingIPAssociate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIPAssociateList) DeepCopyInto(out *FloatingIPAssociateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FloatingIPAssociate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIPAssociateList.
func (in *FloatingIPAssociateList) DeepCopy() *FloatingIPAssociateList {
	if in == nil {
		return nil
	}
	out := new(FloatingIPAssociateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FloatingIPAssociateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIPAssociateSpec) DeepCopyInto(out *FloatingIPAssociateSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIPAssociateSpec.
func (in *FloatingIPAssociateSpec) DeepCopy() *FloatingIPAssociateSpec {
	if in == nil {
		return nil
	}
	out := new(FloatingIPAssociateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIPAssociateStatus) DeepCopyInto(out *FloatingIPAssociateStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIPAssociateStatus.
func (in *FloatingIPAssociateStatus) DeepCopy() *FloatingIPAssociateStatus {
	if in == nil {
		return nil
	}
	out := new(FloatingIPAssociateStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIPList) DeepCopyInto(out *FloatingIPList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FloatingIP, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIPList.
func (in *FloatingIPList) DeepCopy() *FloatingIPList {
	if in == nil {
		return nil
	}
	out := new(FloatingIPList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FloatingIPList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIPSet) DeepCopyInto(out *FloatingIPSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIPSet.
func (in *FloatingIPSet) DeepCopy() *FloatingIPSet {
	if in == nil {
		return nil
	}
	out := new(FloatingIPSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FloatingIPSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIPSetList) DeepCopyInto(out *FloatingIPSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FloatingIPSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIPSetList.
func (in *FloatingIPSetList) DeepCopy() *FloatingIPSetList {
	if in == nil {
		return nil
	}
	out := new(FloatingIPSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FloatingIPSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIPSetSpec) DeepCopyInto(out *FloatingIPSetSpec) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIPSetSpec.
func (in *FloatingIPSetSpec) DeepCopy() *FloatingIPSetSpec {
	if in == nil {
		return nil
	}
	out := new(FloatingIPSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIPSetStatus) DeepCopyInto(out *FloatingIPSetStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIPSetStatus.
func (in *FloatingIPSetStatus) DeepCopy() *FloatingIPSetStatus {
	if in == nil {
		return nil
	}
	out := new(FloatingIPSetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIPSpec) DeepCopyInto(out *FloatingIPSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIPSpec.
func (in *FloatingIPSpec) DeepCopy() *FloatingIPSpec {
	if in == nil {
		return nil
	}
	out := new(FloatingIPSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FloatingIPStatus) DeepCopyInto(out *FloatingIPStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FloatingIPStatus.
func (in *FloatingIPStatus) DeepCopy() *FloatingIPStatus {
	if in == nil {
		return nil
	}
	out := new(FloatingIPStatus)
	in.DeepCopyInto(out)
	return out
}
