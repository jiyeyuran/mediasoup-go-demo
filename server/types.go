package main

import (
	"github.com/jiyeyuran/mediasoup-go"
)

type CreateBroadcasterRequest struct {
	Id              string                    `json:"id,omitempty"`
	DisplayName     string                    `json:"displayName,omitempty"`
	Device          DeviceInfo                `json:"device,omitempty"`
	RtpCapabilities mediasoup.RtpCapabilities `json:"rtpCapabilities,omitempty"`
}

type CreateBroadcasterTransportRequest struct {
	BroadcasterId    string                     `json:"broadcasterId,omitempty"`
	Type             string                     `json:"type,omitempty"`
	RtcpMux          bool                       `json:"rtcpMux,omitempty"`
	Comedia          *bool                      `json:"comedia,omitempty"`
	SctpCapabilities mediasoup.SctpCapabilities `json:"sctpCapabilities,omitempty"`
}

type CreateBroadcasterTransportResponse struct {
	Id       string `json:"id,omitempty"`
	Ip       string `json:"ip,omitempty"`
	Port     uint16 `json:"port,omitempty"`
	RtcpPort uint16 `json:"rtcpPort,omitempty"`
}

type ConnectBroadcasterTransportRequest struct {
	BroadcasterId  string                   `json:"broadcasterId,omitempty"`
	TransportId    string                   `json:"transportId,omitempty"`
	DtlsParameters mediasoup.DtlsParameters `json:"dtlsParameters,omitempty"`
}

type ConnectBroadcasterTransportResponse struct {
}

type CreateBroadcasterProducerRequest struct {
	BroadcasterId string                  `json:"broadcasterId,omitempty"`
	TransportId   string                  `json:"transportId,omitempty"`
	Kind          string                  `json:"kind,omitempty"`
	RtpParameters mediasoup.RtpParameters `json:"rtpParameters,omitempty"`
}

type CreateBroadcasterProducerResponse struct {
}

type CreateBroadcasterConsumerRequest struct {
	BroadcasterId string `json:"broadcasterId,omitempty"`
	TransportId   string `json:"transportId,omitempty"`
	ProducerId    string `json:"producerId,omitempty"`
}

type CreateBroadcasterConsumerResponse struct {
}

type CreateBroadcasterDataConsumerRequest struct {
	BroadcasterId  string `json:"broadcasterId,omitempty"`
	TransportId    string `json:"transportId,omitempty"`
	DataProducerId string `json:"dataProducerId,omitempty"`
}

type CreateBroadcasterDataConsumerResponse struct {
}

type CreateBroadcasterDataProducerRequest struct {
	BroadcasterId        string                         `json:"broadcasterId,omitempty"`
	TransportId          string                         `json:"transportId,omitempty"`
	Label                string                         `json:"label,omitempty"`
	Protocol             string                         `json:"protocol,omitempty"`
	SctpStreamParameters mediasoup.SctpStreamParameters `json:"sctpStreamParameters,omitempty"`
	AppData              H                              `json:"appData,omitempty"`
}

type CreateBroadcasterDataProducerResponse struct {
}

type PeerData struct {
	// // Not joined after a custom protoo "join" request is later received.
	Joined           bool
	DisplayName      string
	Device           DeviceInfo
	RtpCapabilities  *mediasoup.RtpCapabilities
	SctpCapabilities *mediasoup.SctpCapabilities

	// // Have mediasoup related maps ready even before the Peer joins since we
	// // allow creating Transports before joining.
	Transports    map[string]mediasoup.ITransport
	Producers     map[string]*mediasoup.Producer
	Consumers     map[string]*mediasoup.Consumer
	DataProducers map[string]*mediasoup.DataProducer
	DataConsumers map[string]*mediasoup.DataConsumer
}

type DeviceInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Flag    string `json:"flag,omitempty"`
}

type PeerInfo struct {
	Id          string     `json:"id,omitempty"`
	DisplayName string     `json:"displayName,omitempty"`
	Device      DeviceInfo `json:"device,omitempty"`
}

type H map[string]interface{}

type TransportData struct {
	Producing bool
	Consuming bool
}

type TransportTraceInfo struct {
	Type                    string
	DesiredBitrate          uint32
	EffectiveDesiredBitrate uint32
	MinBitrate              uint32
	MaxBitrate              uint32
	StartBitrate            uint32
	MaxPaddingBitrate       uint32
	AvailableBitrate        uint32
}
