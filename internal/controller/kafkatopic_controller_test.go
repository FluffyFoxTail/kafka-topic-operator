/*
Copyright 2025.

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

package controller

import (
	"context"

	clustrer_admin "github.com/FluffyFoxTail/kafka-topic-operator/internal/controller/clustrer-admin"
	"github.com/IBM/sarama"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kafkav1 "github.com/FluffyFoxTail/kafka-topic-operator/api/v1"
)

type MockAdmin struct {
	mock.Mock
}

func (m *MockAdmin) CreateTopic(topic string, detail *sarama.TopicDetail, validateOnly bool) error {
	args := m.Called(topic, detail, validateOnly)
	return args.Error(0)
}

func (m *MockAdmin) ListTopics() (map[string]sarama.TopicDetail, error) {
	args := m.Called()
	topics, _ := args.Get(0).(map[string]sarama.TopicDetail)
	return topics, nil
}

func (m *MockAdmin) Close() error {
	return m.Called().Error(0)
}

type MockAdminFactory struct {
	mock.Mock
	Admin *MockAdmin
}

func (m *MockAdminFactory) NewClient(addrs []string, config *sarama.Config) (clustrer_admin.ClusterAdmin, error) {
	m.Called(addrs, config)
	return m.Admin, nil
}

var _ = Describe("KafkaTopic Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		kafkatopic := &kafkav1.KafkaTopic{}

		BeforeEach(func() {
			By("creating the Kafka Service needed for reconciliation")
			svc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kafka",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     9092,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			}
			err := k8sClient.Create(ctx, svc)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating the endpointSlice needed for reconciliation")
			endpointSlice := &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kafka-abc",
					Namespace: "default",
					Labels: map[string]string{
						"kubernetes.io/service-name": "test-kafka",
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.1"},
						Conditions: discoveryv1.EndpointConditions{
							Ready: ptr.To(true),
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name: ptr.To("kafka"),
						Port: ptr.To(int32(9092)),
					},
				},
			}
			Expect(k8sClient.Create(ctx, endpointSlice)).Should(Succeed())

			By("creating the custom resource for the Kind KafkaTopic")
			err = k8sClient.Get(ctx, typeNamespacedName, kafkatopic)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &kafkav1.KafkaTopic{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: kafkav1.KafkaTopicSpec{
						Replicas:              1,
						RequeueDelaySeconds:   15,
						KafkaServiceName:      "test-kafka",
						KafkaServiceNamespace: "default",
						KafkaPort:             9092,
						Topics: []kafkav1.KafkaTopicDescription{{
							Name:              "test-topic",
							Partitions:        3,
							ReplicationFactor: 1,
						}},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &kafkav1.KafkaTopic{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance KafkaTopic")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			svc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kafka",
					Namespace: "default",
				},
			}
			_ = k8sClient.Delete(ctx, svc)
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			mockAdmin := new(MockAdmin)
			mockAdmin.On("CreateTopic", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			mockAdmin.On("ListTopics").Return(make(map[string]sarama.TopicDetail))
			mockAdmin.On("Close").Return(nil)

			mockAdminFactory := new(MockAdminFactory)
			mockAdminFactory.Admin = mockAdmin
			mockAdminFactory.On("NewClient", mock.Anything, mock.Anything).Return(mockAdmin, nil)

			controllerReconciler := &KafkaTopicReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				Recorder:     record.NewFakeRecorder(10),
				AdminFactory: mockAdminFactory,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
