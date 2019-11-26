// +build !ignore_autogenerated

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Adam) DeepCopyInto(out *Adam) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Adam.
func (in *Adam) DeepCopy() *Adam {
	if in == nil {
		return nil
	}
	out := new(Adam)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Adam) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AdamList) DeepCopyInto(out *AdamList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Adam, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AdamList.
func (in *AdamList) DeepCopy() *AdamList {
	if in == nil {
		return nil
	}
	out := new(AdamList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AdamList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AdamSpec) DeepCopyInto(out *AdamSpec) {
	*out = *in
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]v1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Resources.DeepCopyInto(&out.Resources)
	if in.InstrumentMatchString != nil {
		in, out := &in.InstrumentMatchString, &out.InstrumentMatchString
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToInstrument != nil {
		in, out := &in.NsToInstrument, &out.NsToInstrument
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToInstrumentExclude != nil {
		in, out := &in.NsToInstrumentExclude, &out.NsToInstrumentExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToMonitor != nil {
		in, out := &in.NsToMonitor, &out.NsToMonitor
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToMonitorExclude != nil {
		in, out := &in.NsToMonitorExclude, &out.NsToMonitorExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodesToMonitor != nil {
		in, out := &in.NodesToMonitor, &out.NodesToMonitor
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodesToMonitorExclude != nil {
		in, out := &in.NodesToMonitorExclude, &out.NodesToMonitorExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.InstrumentRule != nil {
		in, out := &in.InstrumentRule, &out.InstrumentRule
		*out = make([]AgentRequest, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AdamSpec.
func (in *AdamSpec) DeepCopy() *AdamSpec {
	if in == nil {
		return nil
	}
	out := new(AdamSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AdamStatus) DeepCopyInto(out *AdamStatus) {
	*out = *in
	in.LastUpdateTime.DeepCopyInto(&out.LastUpdateTime)
	in.State.DeepCopyInto(&out.State)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AdamStatus.
func (in *AdamStatus) DeepCopy() *AdamStatus {
	if in == nil {
		return nil
	}
	out := new(AdamStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AgentRequest) DeepCopyInto(out *AgentRequest) {
	*out = *in
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.MatchString != nil {
		in, out := &in.MatchString, &out.MatchString
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AgentRequest.
func (in *AgentRequest) DeepCopy() *AgentRequest {
	if in == nil {
		return nil
	}
	out := new(AgentRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AgentStatus) DeepCopyInto(out *AgentStatus) {
	*out = *in
	if in.NsToMonitor != nil {
		in, out := &in.NsToMonitor, &out.NsToMonitor
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToMonitorExclude != nil {
		in, out := &in.NsToMonitorExclude, &out.NsToMonitorExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodesToMonitor != nil {
		in, out := &in.NodesToMonitor, &out.NodesToMonitor
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodesToMonitorExclude != nil {
		in, out := &in.NodesToMonitorExclude, &out.NodesToMonitorExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToInstrument != nil {
		in, out := &in.NsToInstrument, &out.NsToInstrument
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToInstrumentExclude != nil {
		in, out := &in.NsToInstrumentExclude, &out.NsToInstrumentExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.InstrumentRule != nil {
		in, out := &in.InstrumentRule, &out.InstrumentRule
		*out = make([]AgentRequest, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.InstrumentMatchString != nil {
		in, out := &in.InstrumentMatchString, &out.InstrumentMatchString
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AgentStatus.
func (in *AgentStatus) DeepCopy() *AgentStatus {
	if in == nil {
		return nil
	}
	out := new(AgentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppDBag) DeepCopyInto(out *AppDBag) {
	*out = *in
	if in.NsToMonitor != nil {
		in, out := &in.NsToMonitor, &out.NsToMonitor
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToMonitorExclude != nil {
		in, out := &in.NsToMonitorExclude, &out.NsToMonitorExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.DeploysToDashboard != nil {
		in, out := &in.DeploysToDashboard, &out.DeploysToDashboard
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodesToMonitor != nil {
		in, out := &in.NodesToMonitor, &out.NodesToMonitor
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodesToMonitorExclude != nil {
		in, out := &in.NodesToMonitorExclude, &out.NodesToMonitorExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToInstrument != nil {
		in, out := &in.NsToInstrument, &out.NsToInstrument
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToInstrumentExclude != nil {
		in, out := &in.NsToInstrumentExclude, &out.NsToInstrumentExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NSInstrumentRule != nil {
		in, out := &in.NSInstrumentRule, &out.NSInstrumentRule
		*out = make([]AgentRequest, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.InstrumentMatchString != nil {
		in, out := &in.InstrumentMatchString, &out.InstrumentMatchString
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.SchemaUpdateCache != nil {
		in, out := &in.SchemaUpdateCache, &out.SchemaUpdateCache
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppDBag.
func (in *AppDBag) DeepCopy() *AppDBag {
	if in == nil {
		return nil
	}
	out := new(AppDBag)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Clusteragent) DeepCopyInto(out *Clusteragent) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Clusteragent.
func (in *Clusteragent) DeepCopy() *Clusteragent {
	if in == nil {
		return nil
	}
	out := new(Clusteragent)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Clusteragent) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusteragentContainerFilter) DeepCopyInto(out *ClusteragentContainerFilter) {
	*out = *in
	if in.WhitelistedNames != nil {
		in, out := &in.WhitelistedNames, &out.WhitelistedNames
		*out = make(map[string][]string, len(*in))
		for key, val := range *in {
			var outVal []string
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make([]string, len(*in))
				copy(*out, *in)
			}
			(*out)[key] = outVal
		}
	}
	if in.BlacklistedNames != nil {
		in, out := &in.BlacklistedNames, &out.BlacklistedNames
		*out = make(map[string][]string, len(*in))
		for key, val := range *in {
			var outVal []string
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make([]string, len(*in))
				copy(*out, *in)
			}
			(*out)[key] = outVal
		}
	}
	if in.BlacklistedLabels != nil {
		in, out := &in.BlacklistedLabels, &out.BlacklistedLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusteragentContainerFilter.
func (in *ClusteragentContainerFilter) DeepCopy() *ClusteragentContainerFilter {
	if in == nil {
		return nil
	}
	out := new(ClusteragentContainerFilter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusteragentList) DeepCopyInto(out *ClusteragentList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Clusteragent, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusteragentList.
func (in *ClusteragentList) DeepCopy() *ClusteragentList {
	if in == nil {
		return nil
	}
	out := new(ClusteragentList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusteragentList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusteragentSpec) DeepCopyInto(out *ClusteragentSpec) {
	*out = *in
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]v1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Resources.DeepCopyInto(&out.Resources)
	if in.InstrumentMatchString != nil {
		in, out := &in.InstrumentMatchString, &out.InstrumentMatchString
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToInstrument != nil {
		in, out := &in.NsToInstrument, &out.NsToInstrument
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToInstrumentExclude != nil {
		in, out := &in.NsToInstrumentExclude, &out.NsToInstrumentExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToMonitor != nil {
		in, out := &in.NsToMonitor, &out.NsToMonitor
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NsToMonitorExclude != nil {
		in, out := &in.NsToMonitorExclude, &out.NsToMonitorExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodesToMonitor != nil {
		in, out := &in.NodesToMonitor, &out.NodesToMonitor
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.NodesToMonitorExclude != nil {
		in, out := &in.NodesToMonitorExclude, &out.NodesToMonitorExclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.InstrumentRule != nil {
		in, out := &in.InstrumentRule, &out.InstrumentRule
		*out = make([]AgentRequest, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.ContainerFilter.DeepCopyInto(&out.ContainerFilter)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusteragentSpec.
func (in *ClusteragentSpec) DeepCopy() *ClusteragentSpec {
	if in == nil {
		return nil
	}
	out := new(ClusteragentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusteragentStatus) DeepCopyInto(out *ClusteragentStatus) {
	*out = *in
	in.LastUpdateTime.DeepCopyInto(&out.LastUpdateTime)
	in.State.DeepCopyInto(&out.State)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusteragentStatus.
func (in *ClusteragentStatus) DeepCopy() *ClusteragentStatus {
	if in == nil {
		return nil
	}
	out := new(ClusteragentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InfraViz) DeepCopyInto(out *InfraViz) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InfraViz.
func (in *InfraViz) DeepCopy() *InfraViz {
	if in == nil {
		return nil
	}
	out := new(InfraViz)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *InfraViz) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InfraVizList) DeepCopyInto(out *InfraVizList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]InfraViz, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InfraVizList.
func (in *InfraVizList) DeepCopy() *InfraVizList {
	if in == nil {
		return nil
	}
	out := new(InfraVizList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *InfraVizList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InfraVizSpec) DeepCopyInto(out *InfraVizSpec) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]v1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]v1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Resources.DeepCopyInto(&out.Resources)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InfraVizSpec.
func (in *InfraVizSpec) DeepCopy() *InfraVizSpec {
	if in == nil {
		return nil
	}
	out := new(InfraVizSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InfraVizStatus) DeepCopyInto(out *InfraVizStatus) {
	*out = *in
	in.LastUpdateTime.DeepCopyInto(&out.LastUpdateTime)
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InfraVizStatus.
func (in *InfraVizStatus) DeepCopy() *InfraVizStatus {
	if in == nil {
		return nil
	}
	out := new(InfraVizStatus)
	in.DeepCopyInto(out)
	return out
}
