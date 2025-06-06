package clustrer_admin

import "github.com/IBM/sarama"

type ClusterAdmin interface {
	CreateTopic(topic string, detail *sarama.TopicDetail, validateOnly bool) error
	ListTopics() (map[string]sarama.TopicDetail, error)
	Close() error
}

type AdminFactory interface {
	NewClient(addr []string, config *sarama.Config) (ClusterAdmin, error)
}

type RealAdminFactory struct{}

func (r *RealAdminFactory) NewClient(addr []string, config *sarama.Config) (ClusterAdmin, error) {
	return sarama.NewClusterAdmin(addr, config)
}
