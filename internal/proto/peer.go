package proto

import (
	"sync"

	"github.com/jiyeyuran/mediasoup-go"
)

type PeerInfo struct {
	Id              string                     `json:"id,omitempty"`
	DisplayName     string                     `json:"displayName,omitempty"`
	Device          DeviceInfo                 `json:"device,omitempty"`
	RtpCapabilities *mediasoup.RtpCapabilities `json:"rtpCapabilities,omitempty"`
	Data            *PeerData                  `json:"-,omitempty"`
}

func (p PeerInfo) CreatePeerData() *PeerData {
	return &PeerData{
		DisplayName:     p.DisplayName,
		Device:          p.Device,
		RtpCapabilities: p.RtpCapabilities,
		transports:      make(map[string]mediasoup.ITransport),
		producers:       make(map[string]*mediasoup.Producer),
		consumers:       make(map[string]*mediasoup.Consumer),
		dataProducers:   make(map[string]*mediasoup.DataProducer),
		dataConsumers:   make(map[string]*mediasoup.DataConsumer),
	}
}

type DeviceInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Flag    string `json:"flag,omitempty"`
}

type PeerData struct {
	locker sync.Mutex
	// // Not joined after a custom protoo "join" request is later received.
	Joined           bool
	DisplayName      string
	Device           DeviceInfo
	RtpCapabilities  *mediasoup.RtpCapabilities
	SctpCapabilities *mediasoup.SctpCapabilities

	// // Have mediasoup related maps ready even before the Peer joins since we
	// // allow creating Transports before joining.
	transports    map[string]mediasoup.ITransport
	producers     map[string]*mediasoup.Producer
	consumers     map[string]*mediasoup.Consumer
	dataProducers map[string]*mediasoup.DataProducer
	dataConsumers map[string]*mediasoup.DataConsumer
}

func NewPeerData() *PeerData {
	return &PeerData{
		transports:    make(map[string]mediasoup.ITransport),
		producers:     make(map[string]*mediasoup.Producer),
		consumers:     make(map[string]*mediasoup.Consumer),
		dataProducers: make(map[string]*mediasoup.DataProducer),
		dataConsumers: make(map[string]*mediasoup.DataConsumer),
	}
}

func (p *PeerData) Transports() map[string]mediasoup.ITransport {
	p.locker.Lock()
	defer p.locker.Unlock()

	newTransports := make(map[string]mediasoup.ITransport)

	for id, transport := range p.transports {
		newTransports[id] = transport
	}

	return newTransports
}

func (p *PeerData) Producers() map[string]*mediasoup.Producer {
	p.locker.Lock()
	defer p.locker.Unlock()

	newProducers := make(map[string]*mediasoup.Producer)

	for id, producer := range p.producers {
		newProducers[id] = producer
	}

	return newProducers
}

func (p *PeerData) Consumers() map[string]*mediasoup.Consumer {
	p.locker.Lock()
	defer p.locker.Unlock()

	newConsumers := make(map[string]*mediasoup.Consumer)

	for id, consumer := range p.consumers {
		newConsumers[id] = consumer
	}

	return newConsumers
}

func (p *PeerData) DataProducers() map[string]*mediasoup.DataProducer {
	p.locker.Lock()
	defer p.locker.Unlock()

	newDataProducers := make(map[string]*mediasoup.DataProducer)

	for id, dataProducer := range p.dataProducers {
		newDataProducers[id] = dataProducer
	}

	return newDataProducers
}

func (p *PeerData) DataConsumers() map[string]*mediasoup.DataConsumer {
	p.locker.Lock()
	defer p.locker.Unlock()

	newDataConsumers := make(map[string]*mediasoup.DataConsumer)

	for id, dataConsumer := range p.dataConsumers {
		newDataConsumers[id] = dataConsumer
	}

	return newDataConsumers
}

func (p *PeerData) GetTransport(id string) mediasoup.ITransport {
	p.locker.Lock()
	defer p.locker.Unlock()

	return p.transports[id]
}

func (p *PeerData) AddTransport(transport mediasoup.ITransport) {
	p.locker.Lock()
	defer p.locker.Unlock()

	p.transports[transport.Id()] = transport
}

func (p *PeerData) GetProducer(id string) *mediasoup.Producer {
	p.locker.Lock()
	defer p.locker.Unlock()

	return p.producers[id]
}

func (p *PeerData) AddProducer(producer *mediasoup.Producer) {
	p.locker.Lock()
	defer p.locker.Unlock()

	p.producers[producer.Id()] = producer
}

func (p *PeerData) GetConsumer(id string) *mediasoup.Consumer {
	p.locker.Lock()
	defer p.locker.Unlock()

	return p.consumers[id]
}

func (p *PeerData) AddConsumer(consumer *mediasoup.Consumer) {
	p.locker.Lock()
	defer p.locker.Unlock()

	p.consumers[consumer.Id()] = consumer
}

func (p *PeerData) GetDataProducer(id string) *mediasoup.DataProducer {
	p.locker.Lock()
	defer p.locker.Unlock()

	return p.dataProducers[id]
}

func (p *PeerData) AddDataProducer(dataProducer *mediasoup.DataProducer) {
	p.locker.Lock()
	defer p.locker.Unlock()

	p.dataProducers[dataProducer.Id()] = dataProducer
}

func (p *PeerData) GetDataConsumer(id string) *mediasoup.DataConsumer {
	p.locker.Lock()
	defer p.locker.Unlock()

	return p.dataConsumers[id]
}

func (p *PeerData) AddDataConsumer(dataConsumer *mediasoup.DataConsumer) {
	p.locker.Lock()
	defer p.locker.Unlock()

	p.dataConsumers[dataConsumer.Id()] = dataConsumer
}

func (p *PeerData) DeleteTransport(id string) {
	p.locker.Lock()
	defer p.locker.Unlock()

	delete(p.transports, id)
}

func (p *PeerData) DeleteProducer(id string) {
	p.locker.Lock()
	defer p.locker.Unlock()

	delete(p.producers, id)
}

func (p *PeerData) DeleteConsumer(id string) {
	p.locker.Lock()
	defer p.locker.Unlock()

	delete(p.consumers, id)
}

func (p *PeerData) DeleteDataProducer(id string) {
	p.locker.Lock()
	defer p.locker.Unlock()

	delete(p.dataProducers, id)
}

func (p *PeerData) DeleteDataConsumer(id string) {
	p.locker.Lock()
	defer p.locker.Unlock()

	delete(p.dataConsumers, id)
}
