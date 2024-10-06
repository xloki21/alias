package base

type BrokerConfig struct {
	Name string `mapstructure:"name"`
	Type string `mapstructure:"type"`
	URI  string `mapstructure:"uri"`
}

type ProducerConfig struct {
	Name    string         `mapstructure:"name"`
	Brokers []BrokerConfig `mapstructure:"brokers"`
	Topic   string         `mapstructure:"topic"`
}

func (c ProducerConfig) GetBrokersURI() []string {
	uris := make([]string, len(c.Brokers))
	for i, broker := range c.Brokers {
		uris[i] = broker.URI
	}
	return uris
}

type ConsumerConfig struct {
	Name    string         `mapstructure:"name"`
	GroupID string         `mapstructure:"group_id"`
	Brokers []BrokerConfig `mapstructure:"brokers"`
	Topic   string         `mapstructure:"topic"`
}

func (c ConsumerConfig) GetBrokersURI() []string {
	uris := make([]string, len(c.Brokers))
	for i, broker := range c.Brokers {
		uris[i] = broker.URI
	}
	return uris
}
