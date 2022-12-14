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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeHosts implements HostInterface
type FakeHosts struct {
	Fake *FakeHostV1alpha1
}

var hostsResource = schema.GroupVersionResource{Group: "host.upm.io", Version: "v1alpha1", Resource: "hosts"}

var hostsKind = schema.GroupVersionKind{Group: "host.upm.io", Version: "v1alpha1", Kind: "Host"}

// Get takes name of the host, and returns the corresponding host object, and an error if there is any.
func (c *FakeHosts) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.Host, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(hostsResource, name), &v1alpha1.Host{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Host), err
}

// List takes label and field selectors, and returns the list of Hosts that match those selectors.
func (c *FakeHosts) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.HostList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(hostsResource, hostsKind, opts), &v1alpha1.HostList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.HostList{ListMeta: obj.(*v1alpha1.HostList).ListMeta}
	for _, item := range obj.(*v1alpha1.HostList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested hosts.
func (c *FakeHosts) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(hostsResource, opts))
}

// Create takes the representation of a host and creates it.  Returns the server's representation of the host, and an error, if there is any.
func (c *FakeHosts) Create(ctx context.Context, host *v1alpha1.Host, opts v1.CreateOptions) (result *v1alpha1.Host, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(hostsResource, host), &v1alpha1.Host{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Host), err
}

// Update takes the representation of a host and updates it. Returns the server's representation of the host, and an error, if there is any.
func (c *FakeHosts) Update(ctx context.Context, host *v1alpha1.Host, opts v1.UpdateOptions) (result *v1alpha1.Host, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(hostsResource, host), &v1alpha1.Host{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Host), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeHosts) UpdateStatus(ctx context.Context, host *v1alpha1.Host, opts v1.UpdateOptions) (*v1alpha1.Host, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(hostsResource, "status", host), &v1alpha1.Host{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Host), err
}

// Delete takes name of the host and deletes it. Returns an error if one occurs.
func (c *FakeHosts) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(hostsResource, name), &v1alpha1.Host{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeHosts) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(hostsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.HostList{})
	return err
}

// Patch applies the patch and returns the patched host.
func (c *FakeHosts) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Host, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(hostsResource, name, pt, data, subresources...), &v1alpha1.Host{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Host), err
}
