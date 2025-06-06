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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KafkaTopicDescription struct {
	Name              string `json:"name"`
	Partitions        int32  `json:"partitions"`
	ReplicationFactor int16  `json:"replicationFactor"`
}

// KafkaTopicSpec defines the desired state of KafkaTopic.
type KafkaTopicSpec struct {
	// Количество брокеров Kafka
	Replicas int32 `json:"replicas,omitempty"`

	// Время до повторной синхронизации при ошибке
	RequeueDelaySeconds int `json:"requeueDelaySeconds"`

	// Название Kubernetes Service, через который доступна Kafka
	KafkaServiceName string `json:"kafkaServiceName"`

	// Namespace, где этот Service находится
	KafkaServiceNamespace string `json:"kafkaServiceNamespace"`

	// Порт, по которому доступна Kafka (по умолчанию 9092)
	KafkaPort int32 `json:"kafkaPort,omitempty"`

	// Список топиков
	Topics []KafkaTopicDescription `json:"topics,omitempty"`
}

// KafkaTopicStatus defines the observed state of KafkaTopic.
type KafkaTopicStatus struct {
	// Условия состояния (Ready, Failed и т.д.)
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Список созданных топиков
	Topics []string `json:"topics,omitempty"`

	// Сообщение об ошибке
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// KafkaTopic is the Schema for the kafkatopics API.
type KafkaTopic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaTopicSpec   `json:"spec,omitempty"`
	Status KafkaTopicStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KafkaTopicList contains a list of KafkaTopic.
type KafkaTopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaTopic `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaTopic{}, &KafkaTopicList{})
}
