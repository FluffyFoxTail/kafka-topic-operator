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
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	kafkav1 "github.com/FluffyFoxTail/kafka-topic-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const endpointSliceLabelServiceName = "kubernetes.io/service-name"

// KafkaTopicReconciler reconciles a KafkaTopic object
type KafkaTopicReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=kafka.fluffyfoxtail.com,resources=kafkatopics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kafka.fluffyfoxtail.com,resources=kafkatopics/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kafka.fluffyfoxtail.com,resources=kafkatopics/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the KafkaTopic object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *KafkaTopicReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var kafkaTopic kafkav1.KafkaTopic
	if err := r.Get(ctx, req.NamespacedName, &kafkaTopic); err != nil {
		log.Error(err, "unable to fetch kafkaTopic")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	adminClient, err := r.createKafkaAdminClient(ctx, kafkaTopic)
	if err != nil {
		log.Error(err, "failed to create Kafka admin client")
		kafkaTopic.Status.ErrorMessage = err.Error()
		_ = r.Status().Update(ctx, &kafkaTopic)

		r.Recorder.Event(&kafkaTopic, corev1.EventTypeWarning, "KafkaAdminError", "Failed to create Kafka admin client")
		return ctrl.Result{RequeueAfter: r.getDelay(kafkaTopic)}, err
	}
	defer func() {
		if err := adminClient.Close(); err != nil {
			log.Error(err, "failed to close Kafka admin client")

		}
	}()

	for _, topic := range kafkaTopic.Spec.Topics {
		if err = r.createTopicIfNotExists(adminClient, topic); err != nil {
			log.Error(err, "failed to create topic", "topic", topic.Name)
			kafkaTopic.Status.ErrorMessage = err.Error()
			_ = r.Status().Update(ctx, &kafkaTopic)

			r.Recorder.Eventf(&kafkaTopic, corev1.EventTypeWarning, "TopicCreationFailed", "Failed to create topic: %s", topic.Name)
			return ctrl.Result{}, err
		}
		log.Info("topic ensured", "topic", topic.Name)
		r.Recorder.Eventf(&kafkaTopic, corev1.EventTypeNormal, "TopicCreated", "Topic ensured: %s", topic.Name)
	}

	topics, err := adminClient.ListTopics()
	if err != nil {
		log.Error(err, "failed to list topics")
		kafkaTopic.Status.ErrorMessage = err.Error()
		_ = r.Status().Update(ctx, &kafkaTopic)

		r.Recorder.Event(&kafkaTopic, corev1.EventTypeWarning, "ListTopicsFailed", "Failed to list Kafka topics")
		return ctrl.Result{}, err
	}

	kafkaTopic.Status.Topics = []string{}
	for name := range topics {
		kafkaTopic.Status.Topics = append(kafkaTopic.Status.Topics, name)
	}
	kafkaTopic.Status.ErrorMessage = ""

	if err = r.Status().Update(ctx, &kafkaTopic); err != nil {
		log.Error(err, "failed to update KafkaCluster status")
		r.Recorder.Event(&kafkaTopic, corev1.EventTypeWarning, "StatusUpdateFailed", "Failed to update KafkaCluster status")
		return ctrl.Result{}, err
	}

	r.Recorder.Event(&kafkaTopic, corev1.EventTypeNormal, "Reconciled", "KafkaCluster reconciled successfully")
	return ctrl.Result{}, nil
}

func (r *KafkaTopicReconciler) createKafkaAdminClient(ctx context.Context, kt kafkav1.KafkaTopic) (sarama.ClusterAdmin, error) {
	svc := &corev1.Service{}
	svcKey := types.NamespacedName{
		Name:      kt.Spec.KafkaServiceName,
		Namespace: kt.Namespace,
	}

	if err := r.Get(ctx, svcKey, svc); err != nil {
		return nil, fmt.Errorf("failed to get Kafka Service %s: %w", svcKey.String(), err)
	}

	var endpointSlices discoveryv1.EndpointSliceList
	if err := r.List(ctx, &endpointSlices,
		client.InNamespace(kt.Namespace),
		client.MatchingLabels{endpointSliceLabelServiceName: kt.Spec.KafkaServiceName},
	); err != nil || len(endpointSlices.Items) == 0 {
		return nil, fmt.Errorf("no live Kafka brokers found via Kafka Service %s: %w", svcKey.String(), err)
	}

	port := int32(9092)
	if kt.Spec.KafkaPort != 0 {
		port = kt.Spec.KafkaPort
	}

	foundReady := false
outer:
	for _, endpointSlice := range endpointSlices.Items {
		for _, endpoint := range endpointSlice.Endpoints {
			if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
				for _, portInfo := range endpointSlice.Ports {
					if portInfo.Port != nil && *portInfo.Port == port {
						foundReady = true
						break outer
					}
				}
			}
		}
	}
	if !foundReady {
		return nil, fmt.Errorf("no ready Kafka endpoints found on port %d in service %s", port, svcKey.String())
	}

	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0
	return sarama.NewClusterAdmin([]string{fmt.Sprintf("%s.%s.svc.cluster.local:%d", svc.Name, svc.Namespace, port)}, config)
}

func (r *KafkaTopicReconciler) createTopicIfNotExists(adminClient sarama.ClusterAdmin, topic kafkav1.KafkaTopicDescription) error {
	topics, err := r.getTopics(adminClient)
	if err != nil {
		return err
	}

	if _, exists := topics[topic.Name]; !exists {
		return adminClient.CreateTopic(topic.Name, &sarama.TopicDetail{
			NumPartitions:     topic.Partitions,
			ReplicationFactor: topic.ReplicationFactor,
		}, false)
	}
	return nil
}

func (r *KafkaTopicReconciler) getTopics(adminClient sarama.ClusterAdmin) (map[string]sarama.TopicDetail, error) {
	topics, err := adminClient.ListTopics()
	if err != nil {
		return nil, err
	}

	return topics, nil
}

func (r *KafkaTopicReconciler) getDelay(kt kafkav1.KafkaTopic) time.Duration {
	delay := 10 * time.Second
	if kt.Spec.RequeueDelaySeconds > 0 {
		delay = time.Duration(kt.Spec.RequeueDelaySeconds) * time.Second
	}
	return delay
}

// SetupWithManager sets up the controller with the Manager.
func (r *KafkaTopicReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("kafka-cluster-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&kafkav1.KafkaTopic{}).
		Named("kafkacluster").
		Complete(r)
}
