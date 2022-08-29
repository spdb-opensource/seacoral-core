// +build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha4

import (
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Action) DeepCopyInto(out *Action) {
	*out = *in
	if in.Delete != nil {
		in, out := &in.Delete, &out.Delete
		*out = new(DeleteAction)
		(*in).DeepCopyInto(*out)
	}
	if in.Rebuild != nil {
		in, out := &in.Rebuild, &out.Rebuild
		*out = new(RebuildAction)
		(*in).DeepCopyInto(*out)
	}
	if in.Migrate != nil {
		in, out := &in.Migrate, &out.Migrate
		*out = new(MigrateAction)
		**out = **in
	}
	if in.ReuseRetainVolume != nil {
		in, out := &in.ReuseRetainVolume, &out.ReuseRetainVolume
		*out = new(ReuseRetainVolumeAction)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Action.
func (in *Action) DeepCopy() *Action {
	if in == nil {
		return nil
	}
	out := new(Action)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Condition) DeepCopyInto(out *Condition) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Condition.
func (in *Condition) DeepCopy() *Condition {
	if in == nil {
		return nil
	}
	out := new(Condition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DeleteAction) DeepCopyInto(out *DeleteAction) {
	*out = *in
	if in.PreStop != nil {
		in, out := &in.PreStop, &out.PreStop
		*out = new(v1.ExecAction)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DeleteAction.
func (in *DeleteAction) DeepCopy() *DeleteAction {
	if in == nil {
		return nil
	}
	out := new(DeleteAction)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ErrMsg) DeepCopyInto(out *ErrMsg) {
	*out = *in
	in.Time.DeepCopyInto(&out.Time)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ErrMsg.
func (in *ErrMsg) DeepCopy() *ErrMsg {
	if in == nil {
		return nil
	}
	out := new(ErrMsg)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MigrateAction) DeepCopyInto(out *MigrateAction) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MigrateAction.
func (in *MigrateAction) DeepCopy() *MigrateAction {
	if in == nil {
		return nil
	}
	out := new(MigrateAction)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkingRequest) DeepCopyInto(out *NetworkingRequest) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkingRequest.
func (in *NetworkingRequest) DeepCopy() *NetworkingRequest {
	if in == nil {
		return nil
	}
	out := new(NetworkingRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PVCRequest) DeepCopyInto(out *PVCRequest) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
	if in.AccessModes != nil {
		in, out := &in.AccessModes, &out.AccessModes
		*out = make([]v1.PersistentVolumeAccessMode, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PVCRequest.
func (in *PVCRequest) DeepCopy() *PVCRequest {
	if in == nil {
		return nil
	}
	out := new(PVCRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RebuildAction) DeepCopyInto(out *RebuildAction) {
	*out = *in
	if in.NodeName != nil {
		in, out := &in.NodeName, &out.NodeName
		*out = new(string)
		**out = **in
	}
	if in.RetainVolume != nil {
		in, out := &in.RetainVolume, &out.RetainVolume
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RebuildAction.
func (in *RebuildAction) DeepCopy() *RebuildAction {
	if in == nil {
		return nil
	}
	out := new(RebuildAction)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RebuildVolumeStatus) DeepCopyInto(out *RebuildVolumeStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RebuildVolumeStatus.
func (in *RebuildVolumeStatus) DeepCopy() *RebuildVolumeStatus {
	if in == nil {
		return nil
	}
	out := new(RebuildVolumeStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReuseRetainVolumeAction) DeepCopyInto(out *ReuseRetainVolumeAction) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReuseRetainVolumeAction.
func (in *ReuseRetainVolumeAction) DeepCopy() *ReuseRetainVolumeAction {
	if in == nil {
		return nil
	}
	out := new(ReuseRetainVolumeAction)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Storage) DeepCopyInto(out *Storage) {
	*out = *in
	out.Request = in.Request.DeepCopy()
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Storage.
func (in *Storage) DeepCopy() *Storage {
	if in == nil {
		return nil
	}
	out := new(Storage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Unit) DeepCopyInto(out *Unit) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Unit.
func (in *Unit) DeepCopy() *Unit {
	if in == nil {
		return nil
	}
	out := new(Unit)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Unit) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UnitList) DeepCopyInto(out *UnitList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Unit, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UnitList.
func (in *UnitList) DeepCopy() *UnitList {
	if in == nil {
		return nil
	}
	out := new(UnitList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *UnitList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UnitSpec) DeepCopyInto(out *UnitSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
	out.Networking = in.Networking
	if in.VolumeClaims != nil {
		in, out := &in.VolumeClaims, &out.VolumeClaims
		*out = make([]PVCRequest, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Action.DeepCopyInto(&out.Action)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UnitSpec.
func (in *UnitSpec) DeepCopy() *UnitSpec {
	if in == nil {
		return nil
	}
	out := new(UnitSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UnitStatus) DeepCopyInto(out *UnitStatus) {
	*out = *in
	if in.RebuildStatus != nil {
		in, out := &in.RebuildStatus, &out.RebuildStatus
		*out = new(RebuildVolumeStatus)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]Condition, len(*in))
		copy(*out, *in)
	}
	if in.ErrMsgs != nil {
		in, out := &in.ErrMsgs, &out.ErrMsgs
		*out = make([]ErrMsg, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UnitStatus.
func (in *UnitStatus) DeepCopy() *UnitStatus {
	if in == nil {
		return nil
	}
	out := new(UnitStatus)
	in.DeepCopyInto(out)
	return out
}
