package collect_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/redhat-developer/service-binding-operator/api/v1alpha1"
	"github.com/redhat-developer/service-binding-operator/pkg/binding"
	bindingmocks "github.com/redhat-developer/service-binding-operator/pkg/binding/mocks"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"

	"github.com/redhat-developer/service-binding-operator/pkg/reconcile/pipeline"
	"github.com/redhat-developer/service-binding-operator/pkg/reconcile/pipeline/handler/collect"
	"github.com/redhat-developer/service-binding-operator/pkg/reconcile/pipeline/mocks"
)

var (
	mockCtrl    *gomock.Controller
	ctx         *mocks.MockContext
	shouldRetry = func(handler pipeline.Handler, reason string, err error) {
		It("should indicate retry and set collection ready status to false", func() {
			ctx.EXPECT().RetryProcessing(err)
			ctx.EXPECT().SetCondition(v1alpha1.Conditions().NotCollectionReady().Reason(reason).Msg(err.Error()).Build())
			handler.Handle(ctx)
		})
	}
)

var _ = Describe("Preflight check", func() {
	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		ctx = mocks.NewMockContext(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("on error reading services", func() {
		var (
			errMsg = "foo"
			err    = errors.New(errMsg)
		)

		BeforeEach(func() {
			ctx.EXPECT().Services().Return([]pipeline.Service{}, err)
		})
		shouldRetry(pipeline.HandlerFunc(collect.PreFlight), collect.ErrorReadingServicesReason, err)
	})
})

var _ = Describe("Collect Binding Definitions", func() {

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		ctx = mocks.NewMockContext(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("request retry processing and set collection ready status to false", func() {

		var (
			errMsg = "foo"
			err    = errors.New(errMsg)
		)

		Context("on error reading CRD for at least one service", func() {

			BeforeEach(func() {
				service1 := mocks.NewMockService(mockCtrl)
				crd := mocks.NewMockCRD(mockCtrl)
				service1.EXPECT().CustomResourceDefinition().Return(crd, nil)
				service1.EXPECT().Resource().Return(&unstructured.Unstructured{})
				crd.EXPECT().Resource().Return(&unstructured.Unstructured{})
				crd.EXPECT().Descriptor().Return(nil, nil)

				service2 := mocks.NewMockService(mockCtrl)
				service2.EXPECT().CustomResourceDefinition().Return(nil, err)
				ctx.EXPECT().Services().Return([]pipeline.Service{service1, service2}, nil)
			})

			shouldRetry(pipeline.HandlerFunc(collect.BindingDefinitions), collect.ErrorReadingCRD, err)
		})

		Context("on error reading descriptor from at least one service", func() {
			BeforeEach(func() {
				service1 := mocks.NewMockService(mockCtrl)
				crd := mocks.NewMockCRD(mockCtrl)
				service1.EXPECT().CustomResourceDefinition().Return(crd, nil)
				service1.EXPECT().Resource().Return(&unstructured.Unstructured{})
				crd.EXPECT().Resource().Return(&unstructured.Unstructured{})
				crd.EXPECT().Descriptor().Return(nil, nil)

				service2 := mocks.NewMockService(mockCtrl)
				crd2 := mocks.NewMockCRD(mockCtrl)
				service2.EXPECT().CustomResourceDefinition().Return(crd2, nil)
				crd2.EXPECT().Descriptor().Return(nil, err)

				ctx.EXPECT().Services().Return([]pipeline.Service{service1, service2}, nil)
			})

			shouldRetry(pipeline.HandlerFunc(collect.BindingDefinitions), collect.ErrorReadingDescriptorReason, err)
		})

	})
	Describe("successful processing", func() {

		var (
			services []pipeline.Service

			defService = func() (*mocks.MockService, *unstructured.Unstructured) {
				service := mocks.NewMockService(mockCtrl)
				serviceContent := &unstructured.Unstructured{}
				service.EXPECT().Resource().Return(serviceContent)
				services = append(services, service)
				return service, serviceContent
			}
		)

		BeforeEach(func() {
			services = []pipeline.Service{}
			ctx.EXPECT().Services().DoAndReturn(func() ([]pipeline.Service, error) { return services, nil })
		})

		Context("single custom service", func() {
			var (
				service        *mocks.MockService
				serviceContent *unstructured.Unstructured
				crd            *mocks.MockCRD
				crdContent     *unstructured.Unstructured
			)
			BeforeEach(func() {
				service, serviceContent = defService()

				crd = mocks.NewMockCRD(mockCtrl)
				crdContent = &unstructured.Unstructured{}
				crd.EXPECT().Resource().Return(crdContent)

				service.EXPECT().CustomResourceDefinition().Return(crd, nil)

			})

			It("should extract binding definitions from service annotations", func() {

				serviceContent.SetAnnotations(map[string]string{
					"foo":                                   "bar",
					collect.ProvisionedServiceAnnotationKey: "true",
					"service.binding/foo":                   "path={.status.foo},objectType=Secret,sourceValue=username",
					"service.binding/foo2":                  "path={.status.foo2},objectType=Secret,sourceValue=username",
				})
				crd.EXPECT().Descriptor().Return(&olmv1alpha1.CRDDescription{}, nil)
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"status", "foo"}))
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"status", "foo2"}))
				collect.BindingDefinitions(ctx)
			})

			It("should extract binding definitions definitions from CRD annotations", func() {
				crdContent.SetAnnotations(map[string]string{
					"foo":                  "bar",
					"service.binding/foo":  "path={.status.foo},objectType=Secret,sourceValue=username",
					"service.binding/foo2": "path={.status.foo2},objectType=Secret,sourceValue=username",
				})
				crd.EXPECT().Descriptor().Return(&olmv1alpha1.CRDDescription{}, nil)
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"status", "foo"}))
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"status", "foo2"}))
				collect.BindingDefinitions(ctx)
			})

			It("should extract binding definitions from status descriptors", func() {
				crd.EXPECT().Descriptor().Return(&olmv1alpha1.CRDDescription{
					StatusDescriptors: []olmv1alpha1.StatusDescriptor{
						{
							Path:         "foo",
							XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:Secret", "service.binding:username:sourceValue=username"},
						},
						{
							Path:         "bar",
							XDescriptors: []string{"bar"},
						},
						{
							Path:         "foo2",
							XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:Secret", "service.binding:username2:sourceValue=username"},
						},
					},
				}, nil)
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"status", "foo"}))
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"status", "foo2"}))
				collect.BindingDefinitions(ctx)
			})

			It("should extract binding definitions from spec descriptors", func() {
				crd.EXPECT().Descriptor().Return(&olmv1alpha1.CRDDescription{
					SpecDescriptors: []olmv1alpha1.SpecDescriptor{
						{
							Path:         "foo",
							XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:Secret", "service.binding:username:sourceValue=username"},
						},
						{
							Path:         "bar",
							XDescriptors: []string{"bar"},
						},
						{
							Path:         "foo2",
							XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:Secret", "service.binding:username2:sourceValue=username"},
						},
					},
				}, nil)
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"spec", "foo"}))
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"spec", "foo2"}))
				collect.BindingDefinitions(ctx)
			})

			It("should extract binding definitions both from spec and status descriptors", func() {
				crd.EXPECT().Descriptor().Return(&olmv1alpha1.CRDDescription{
					SpecDescriptors: []olmv1alpha1.SpecDescriptor{
						{
							Path:         "foo",
							XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:Secret", "service.binding:username:sourceValue=username"},
						},
					},
					StatusDescriptors: []olmv1alpha1.StatusDescriptor{
						{
							Path:         "foo",
							XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:Secret", "service.binding:username2:sourceValue=username"},
						},
					},
				}, nil)
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"status", "foo"}))
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"spec", "foo"}))
				collect.BindingDefinitions(ctx)
			})

			It("binding definitions on CRD take precedence over those from descriptors", func() {
				crd.EXPECT().Descriptor().Return(&olmv1alpha1.CRDDescription{
					SpecDescriptors: []olmv1alpha1.SpecDescriptor{
						{
							Path:         "foo",
							XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:Secret", "service.binding:username:sourceValue=username"},
						},
					},
					StatusDescriptors: []olmv1alpha1.StatusDescriptor{
						{
							Path:         "foo",
							XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:Secret", "service.binding:username2:sourceValue=username"},
						},
					},
				}, nil)
				crdContent.SetAnnotations(map[string]string{
					"service.binding/username2": "path={.spec.foo2},objectType=Secret,sourceValue=username",
				})
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"spec", "foo"}))
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"spec", "foo2"}))
				collect.BindingDefinitions(ctx)
			})

			It("binding definitions on service take precedence over those from descriptors", func() {
				crd.EXPECT().Descriptor().Return(&olmv1alpha1.CRDDescription{
					SpecDescriptors: []olmv1alpha1.SpecDescriptor{
						{
							Path:         "foo",
							XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:Secret", "service.binding:username:sourceValue=username"},
						},
					},
					StatusDescriptors: []olmv1alpha1.StatusDescriptor{
						{
							Path:         "foo",
							XDescriptors: []string{"urn:alm:descriptor:io.kubernetes:Secret", "service.binding:username2:sourceValue=username"},
						},
					},
				}, nil)
				serviceContent.SetAnnotations(map[string]string{
					"service.binding/username2": "path={.spec.foo2},objectType=Secret,sourceValue=username",
				})
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"spec", "foo"}))
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"spec", "foo2"}))
				collect.BindingDefinitions(ctx)
			})

			It("binding definitions on service take precedence over those from CRD", func() {
				crd.EXPECT().Descriptor().Return(&olmv1alpha1.CRDDescription{}, nil)
				crdContent.SetAnnotations(map[string]string{
					"service.binding/foo":  "path={.spec.foo},objectType=Secret,sourceValue=username",
					"service.binding/foo2": "path={.spec.foo2},objectType=Secret,sourceValue=username",
				})
				serviceContent.SetAnnotations(map[string]string{
					"service.binding/foo2": "path={.status.foo2},objectType=Secret,sourceValue=username",
					"service.binding/foo3": "path={.spec.foo3},objectType=Secret,sourceValue=username",
				})
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"spec", "foo"}))
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"spec", "foo3"}))
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"status", "foo2"}))
				collect.BindingDefinitions(ctx)
			})

			Context("non OLM environment", func() {
				It("should extract binding definitions both from service and CRD annotations", func() {
					crd.EXPECT().Descriptor().Return(nil, nil)
					crdContent.SetAnnotations(map[string]string{
						"service.binding/foo": "path={.spec.foo},objectType=Secret,sourceValue=username",
					})
					serviceContent.SetAnnotations(map[string]string{
						"service.binding/foo2": "path={.status.foo2},objectType=Secret,sourceValue=username",
					})
					service.EXPECT().AddBindingDef(bindingDefPath([]string{"spec", "foo"}))
					service.EXPECT().AddBindingDef(bindingDefPath([]string{"status", "foo2"}))
					collect.BindingDefinitions(ctx)
				})
			})

		})

		Context("plain k8s resource as service", func() {

			It("should extract binding definitions from annotations", func() {
				service, content := defService()
				service.EXPECT().CustomResourceDefinition().Return(nil, nil)
				content.SetAnnotations(map[string]string{
					"service.binding/foo2": "path={.status.foo2},objectType=Secret,sourceValue=username",
				})
				service.EXPECT().AddBindingDef(bindingDefPath([]string{"status", "foo2"}))
				collect.BindingDefinitions(ctx)
			})
		})

		Context("multiple services", func() {
			It("should extract binding definitions for both service", func() {
				for i := 0; i < 2; i++ {
					service, content := defService()
					service.EXPECT().CustomResourceDefinition().Return(nil, nil)
					content.SetAnnotations(map[string]string{
						"service.binding/foo2": "path={.status.foo2},objectType=Secret,sourceValue=username",
					})
					service.EXPECT().AddBindingDef(bindingDefPath([]string{"status", "foo2"}))
				}

				collect.BindingDefinitions(ctx)
			})
		})
	})

})

var _ = Describe("Collect Binding Data", func() {

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		ctx = mocks.NewMockContext(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("request retry processing and set collection ready status to false", func() {
		var (
			errMsg = "foo"
			err    = errors.New(errMsg)
		)

		Context("on error collecting data", func() {
			var (
				service *mocks.MockService
			)
			BeforeEach(func() {
				service = mocks.NewMockService(mockCtrl)
				serviceContent := &unstructured.Unstructured{}
				service.EXPECT().Resource().Return(serviceContent)

				bd := bindingmocks.NewMockDefinition(mockCtrl)
				bd.EXPECT().Apply(serviceContent).Return(nil, err)

				service.EXPECT().BindingDefs().Return([]binding.Definition{bd})

				ctx.EXPECT().Services().Return([]pipeline.Service{service}, nil)
			})
			shouldRetry(pipeline.HandlerFunc(collect.BindingItems), collect.ErrorReadingBindingReason, err)
		})

		Context("on returning unexpected data", func() {
			var (
				service *mocks.MockService
			)
			BeforeEach(func() {
				service = mocks.NewMockService(mockCtrl)
				serviceContent := &unstructured.Unstructured{}
				service.EXPECT().Resource().Return(serviceContent)

				bd := bindingmocks.NewMockDefinition(mockCtrl)
				bv := bindingmocks.NewMockValue(mockCtrl)
				bv.EXPECT().Get().Return("we should not return strings")
				bd.EXPECT().Apply(serviceContent).Return(bv, nil)

				service.EXPECT().BindingDefs().Return([]binding.Definition{bd})

				ctx.EXPECT().Services().Return([]pipeline.Service{service}, nil)
			})
			shouldRetry(pipeline.HandlerFunc(collect.BindingItems), "DataNotMap", collect.DataNotMap)
		})
	})

	Describe("successful processing", func() {
		var (
			services []pipeline.Service

			defService = func() (*mocks.MockService, *unstructured.Unstructured) {
				service := mocks.NewMockService(mockCtrl)
				serviceContent := &unstructured.Unstructured{}
				service.EXPECT().Resource().Return(serviceContent)
				services = append(services, service)
				return service, serviceContent
			}
		)

		BeforeEach(func() {
			services = []pipeline.Service{}
			ctx.EXPECT().Services().DoAndReturn(func() ([]pipeline.Service, error) { return services, nil })
		})

		Context("two services with binding definitions", func() {
			BeforeEach(func() {
				serviceMap := map[string]map[string]interface{}{
					"service1": {
						"bd1": map[string]interface{}{
							"foo": "bar",
						},
					},
					"service2": {
						"bd2": map[string]interface{}{
							"foo2": "bar2",
							"foo3": "bar3",
						},
					},
				}
				for _, bindingsVal := range serviceMap {
					service, res := defService()
					var bindings []binding.Definition
					for _, val := range bindingsVal {
						bd := bindingmocks.NewMockDefinition(mockCtrl)
						bv := bindingmocks.NewMockValue(mockCtrl)
						bv.EXPECT().Get().Return(val)
						bd.EXPECT().Apply(res).Return(bv, nil)
						bindings = append(bindings, bd)
						for k, v := range val.(map[string]interface{}) {
							ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: k, Value: v, Source: service})
						}
					}
					service.EXPECT().BindingDefs().Return(bindings)
				}
			})
			It("should collect all data", func() {
				collect.BindingItems(ctx)
			})
		})
		It("should expand map values", func() {
			service, res := defService()
			val := map[string]interface{}{
				"foo": map[string]interface{}{
					"bar":  "bla",
					"bar2": "bla2",
				},
			}
			var bindings []binding.Definition
			bd := bindingmocks.NewMockDefinition(mockCtrl)
			bv := bindingmocks.NewMockValue(mockCtrl)
			bv.EXPECT().Get().Return(val)
			bd.EXPECT().Apply(res).Return(bv, nil)
			bindings = append(bindings, bd)
			ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo_bar", Value: "bla", Source: service})
			ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo_bar2", Value: "bla2", Source: service})
			service.EXPECT().BindingDefs().Return(bindings)
			collect.BindingItems(ctx)
		})
		It("should expand slice values", func() {
			service, res := defService()
			val := map[string]interface{}{
				"foo": map[string]interface{}{
					"bar": []string{"bla", "bla2"},
				},
			}
			var bindings []binding.Definition
			bd := bindingmocks.NewMockDefinition(mockCtrl)
			bv := bindingmocks.NewMockValue(mockCtrl)
			bv.EXPECT().Get().Return(val)
			bd.EXPECT().Apply(res).Return(bv, nil)
			bindings = append(bindings, bd)
			ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo_bar_0", Value: "bla", Source: service})
			ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo_bar_1", Value: "bla2", Source: service})
			service.EXPECT().BindingDefs().Return(bindings)
			collect.BindingItems(ctx)
		})
	})

})

var _ = Describe("Collect From Owned Resources", func() {
	var (
		mockCtrl *gomock.Controller
		ctx      *mocks.MockContext
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		ctx = mocks.NewMockContext(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("successful processing", func() {
		var (
			services []pipeline.Service

			defService = func() (*mocks.MockService, *unstructured.Unstructured) {
				service := mocks.NewMockService(mockCtrl)
				serviceContent := &unstructured.Unstructured{}
				services = append(services, service)
				return service, serviceContent
			}
		)

		BeforeEach(func() {
			services = []pipeline.Service{}
			ctx.EXPECT().Services().DoAndReturn(func() ([]pipeline.Service, error) { return services, nil })
		})

		Context("two services", func() {

			It("should collect bindings from owned secrets", func() {

				service1, _ := defService()
				secret1 := &unstructured.Unstructured{Object: map[string]interface{}{
					"data": map[string]interface{}{
						"foo1": base64.StdEncoding.EncodeToString([]byte("val1")),
						"foo2": base64.StdEncoding.EncodeToString([]byte("val2")),
					},
				}}
				secret1.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"})
				service2, _ := defService()
				secret2 := &unstructured.Unstructured{Object: map[string]interface{}{
					"data": map[string]interface{}{
						"foo3": base64.StdEncoding.EncodeToString([]byte("val3")),
						"foo4": base64.StdEncoding.EncodeToString([]byte("val4")),
					},
				}}
				secret2.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"})

				secret3 := &unstructured.Unstructured{Object: map[string]interface{}{
					"data": map[string]interface{}{
						"foo5": base64.StdEncoding.EncodeToString([]byte("val5")),
						"foo6": base64.StdEncoding.EncodeToString([]byte("val6")),
					},
				}}
				secret3.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"})

				service1.EXPECT().OwnedResources().Return([]*unstructured.Unstructured{secret1}, nil)

				service2.EXPECT().OwnedResources().Return([]*unstructured.Unstructured{secret2, secret3}, nil)

				ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo1", Value: "val1", Source: service1})
				ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo2", Value: "val2", Source: service1})

				ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo3", Value: "val3", Source: service2})
				ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo4", Value: "val4", Source: service2})

				ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo5", Value: "val5", Source: service2})
				ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo6", Value: "val6", Source: service2})

				collect.OwnedResources(ctx)
			})

			It("should collect bindings from owned configmaps", func() {

				service1, _ := defService()
				configMap1 := &unstructured.Unstructured{Object: map[string]interface{}{
					"data": map[string]interface{}{
						"foo1": "val1",
						"foo2": "val2",
					},
				}}
				configMap1.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
				service2, _ := defService()
				configMap2 := &unstructured.Unstructured{Object: map[string]interface{}{
					"data": map[string]interface{}{
						"foo3": "val3",
						"foo4": "val4",
					},
				}}
				configMap2.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})

				service1.EXPECT().OwnedResources().Return([]*unstructured.Unstructured{configMap1}, nil)

				service2.EXPECT().OwnedResources().Return([]*unstructured.Unstructured{configMap2}, nil)

				ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo1", Value: "val1", Source: service1})
				ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo2", Value: "val2", Source: service1})

				ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo3", Value: "val3", Source: service2})
				ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "foo4", Value: "val4", Source: service2})

				collect.OwnedResources(ctx)
			})

			It("should collect bindings from owned secrets", func() {

				service1, _ := defService()
				svr := &unstructured.Unstructured{Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"clusterIP": "val1",
					},
				}}
				svr.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"})

				service1.EXPECT().OwnedResources().Return([]*unstructured.Unstructured{svr}, nil)

				ctx.EXPECT().AddBindingItem(&pipeline.BindingItem{Name: "clusterIP", Value: "val1", Source: service1})

				collect.OwnedResources(ctx)
			})
		})
	})

})

var _ = Describe("Collect From Provisioned Service", func() {
	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		ctx = mocks.NewMockContext(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("successful processing", func() {
		var (
			services []pipeline.Service

			defService = func() (*mocks.MockService, *unstructured.Unstructured) {
				service := mocks.NewMockService(mockCtrl)
				serviceContent := &unstructured.Unstructured{}
				service.EXPECT().Resource().Return(serviceContent)
				services = append(services, service)
				return service, serviceContent
			}
		)

		BeforeEach(func() {
			services = []pipeline.Service{}
			ctx.EXPECT().Services().DoAndReturn(func() ([]pipeline.Service, error) { return services, nil })
		})

		It("should collect secret names referred in services", func() {
			secretName1 := "foo"
			secretName2 := "bar"
			ns1 := "ns1"
			ns2 := "ns2"
			svc1, content1 := defService()
			content1.Object = map[string]interface{}{
				"status": map[string]interface{}{
					"binding": map[string]interface{}{
						"name": secretName1,
					},
				},
			}
			content1.SetNamespace(ns1)
			secret1 := &unstructured.Unstructured{}
			secret1.SetName(secretName1)

			svc2, content2 := defService()
			content2.Object = map[string]interface{}{
				"status": map[string]interface{}{
					"binding": map[string]interface{}{
						"name": secretName2,
					},
				},
			}
			content2.SetNamespace(ns2)
			secret2 := &unstructured.Unstructured{}
			secret2.SetName(secretName2)

			ctx.EXPECT().ReadSecret(ns1, secretName1).Return(secret1, nil)
			ctx.EXPECT().ReadSecret(ns2, secretName2).Return(secret2, nil)

			ctx.EXPECT().AddBindings(&pipeline.SecretBackedBindings{Service: svc1, Secret: secret1})
			ctx.EXPECT().AddBindings(&pipeline.SecretBackedBindings{Service: svc2, Secret: secret2})

			collect.ProvisionedService(ctx)
		})

		It("do nothing if there is no secret reference and services is not CRD backed", func() {
			service, _ := defService()
			service.EXPECT().CustomResourceDefinition().Return(nil, nil)
			collect.ProvisionedService(ctx)
		})

		It("do nothing if there is no secret reference and services CRD does not indicate provisioned service", func() {
			service, _ := defService()
			service.EXPECT().CustomResourceDefinition().Return(nil, nil)

			collect.ProvisionedService(ctx)
		})

		It("should retry processing if secret reference is not present bu CRD indicates provisioned service", func() {
			service, content := defService()
			content.SetName("foo")
			content.SetNamespace("ns1")
			err := errors.New("CRD of service ns1/foo indicates provisioned service, but no secret name provided under .status.binding.name")
			crd := mocks.NewMockCRD(mockCtrl)
			u := &unstructured.Unstructured{}
			u.SetAnnotations(map[string]string{collect.ProvisionedServiceAnnotationKey: "true"})

			crd.EXPECT().Resource().Return(u)

			service.EXPECT().CustomResourceDefinition().Return(crd, nil)

			ctx.EXPECT().RetryProcessing(err)
			ctx.EXPECT().SetCondition(v1alpha1.Conditions().NotCollectionReady().Reason(collect.ErrorReadingBindingReason).Msg(err.Error()).Build())

			collect.ProvisionedService(ctx)
		})

		It("should retry processing if secret reference is not present bu CRD indicates provisioned service", func() {
			service, _ := defService()

			err := errors.New("foo")

			service.EXPECT().CustomResourceDefinition().Return(nil, err)

			ctx.EXPECT().RetryProcessing(err)
			ctx.EXPECT().SetCondition(v1alpha1.Conditions().NotCollectionReady().Reason(collect.ErrorReadingCRD).Msg(err.Error()).Build())

			collect.ProvisionedService(ctx)
		})
	})

})

var _ = Describe("Integration Collect definitions + items", func() {
	var (
		mockCtrl        *gomock.Controller
		ctx             *mocks.MockContext
		service         *mocks.MockService
		serviceResource *unstructured.Unstructured
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		ctx = mocks.NewMockContext(mockCtrl)
		service = mocks.NewMockService(mockCtrl)

		serviceResource = &unstructured.Unstructured{}

		service.EXPECT().Resource().Return(serviceResource).MinTimes(1)

		service.EXPECT().CustomResourceDefinition().Return(nil, nil)

		ctx.EXPECT().Services().Return([]pipeline.Service{service}, nil).MinTimes(1)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	type testCase struct {
		serviceContent map[string]interface{}
		expectedItems  []*pipeline.BindingItem
		secrets        map[string]*unstructured.Unstructured
		configMaps     map[string]*unstructured.Unstructured
	}

	DescribeTable("retrieve binding data",
		func(tc testCase) {
			serviceResource.SetUnstructuredContent(tc.serviceContent)

			var bindingDefs []binding.Definition
			service.EXPECT().AddBindingDef(gomock.Any()).DoAndReturn(func(bd binding.Definition) {
				bindingDefs = append(bindingDefs, bd)
			}).Times(len(serviceResource.GetAnnotations()))

			service.EXPECT().BindingDefs().DoAndReturn(func() []binding.Definition { return bindingDefs })

			for _, bi := range tc.expectedItems {
				bi.Source = service
				ctx.EXPECT().AddBindingItem(bi)
			}

			for name, content := range tc.secrets {
				ctx.EXPECT().ReadSecret(serviceResource.GetNamespace(), name).Return(content, nil)
			}
			for name, content := range tc.configMaps {
				ctx.EXPECT().ReadConfigMap(serviceResource.GetNamespace(), name).Return(content, nil)
			}
			collect.BindingDefinitions(ctx)
			collect.BindingItems(ctx)
		},
		Entry("from service status part", testCase{
			serviceContent: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"service.binding":      "path={.status.foo}",
						"service.binding/bar2": "path={.status.foo2}",
					},
				},
				"status": map[string]interface{}{
					"foo":  "val1",
					"foo2": "val2",
					"foo3": "val3",
				},
			},
			expectedItems: []*pipeline.BindingItem{
				{
					Name:  "foo",
					Value: "val1",
				},
				{
					Name:  "bar2",
					Value: "val2",
				},
			},
		}),
		Entry("from secret referred in service field", testCase{
			serviceContent: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "n1",
					"annotations": map[string]interface{}{
						"service.binding":     "path={.status.foo},objectType=Secret",
						"service.binding/bar": "path={.status.foo2},objectType=Secret,sourceKey=bar2",
					},
				},
				"status": map[string]interface{}{
					"foo":  "secret1",
					"foo2": "secret2",
					"foo3": "val3",
				},
			},
			expectedItems: []*pipeline.BindingItem{
				{
					Name:  "foo",
					Value: "val1",
				},
				{
					Name:  "bar2",
					Value: "val2",
				},
				{
					Name:  "bar",
					Value: "val3",
				},
			},
			secrets: map[string]*unstructured.Unstructured{
				"secret1": {
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"foo":  base64.StdEncoding.EncodeToString([]byte("val1")),
							"bar2": base64.StdEncoding.EncodeToString([]byte("val2")),
						},
					},
				},
				"secret2": {
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"foo":  base64.StdEncoding.EncodeToString([]byte("val1")),
							"bar2": base64.StdEncoding.EncodeToString([]byte("val3")),
						},
					},
				},
			},
		}),
		Entry("from config map referred in service field", testCase{
			serviceContent: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "n1",
					"annotations": map[string]interface{}{
						"service.binding":     "path={.status.foo},objectType=ConfigMap",
						"service.binding/bar": "path={.status.foo2},objectType=ConfigMap,sourceKey=bar2",
					},
				},
				"status": map[string]interface{}{
					"foo":  "config1",
					"foo2": "config2",
					"foo3": "val3",
				},
			},
			expectedItems: []*pipeline.BindingItem{
				{
					Name:  "foo",
					Value: "val1",
				},
				{
					Name:  "bar2",
					Value: "val2",
				},
				{
					Name:  "bar",
					Value: "val3",
				},
			},
			configMaps: map[string]*unstructured.Unstructured{
				"config1": {
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"foo":  "val1",
							"bar2": "val2",
						},
					},
				},
				"config2": {
					Object: map[string]interface{}{
						"data": map[string]interface{}{
							"foo":  "val1",
							"bar2": "val3",
						},
					},
				},
			},
		}),
	)

})

type bindingDefMatcher struct {
	path []string
}

func (m bindingDefMatcher) Matches(x interface{}) bool {
	bd, ok := x.(binding.Definition)
	if ok {
		return reflect.DeepEqual(bd.GetPath(), m.path)
	}
	return false
}

func (m bindingDefMatcher) String() string {
	return fmt.Sprintf("match %s path", m.path)
}

func bindingDefPath(path []string) gomock.Matcher {
	return &bindingDefMatcher{
		path: path,
	}
}
