package proto

import "github.com/jiyeyuran/mediasoup-go"

type CreateBroadcasterRequest struct {
	Id              string                    `json:"id,omitempty"`
	DisplayName     string                    `json:"displayName,omitempty"`
	Device          DeviceInfo                `json:"device,omitempty"`
	RtpCapabilities mediasoup.RtpCapabilities `json:"rtpCapabilities,omitempty"`
}

type CreateBroadcasterResponse struct {
	Peers []PeerInfo `json:"peers,omitempty"`
}

type DeleteBroadcasterRequest struct {
	BroadcasterId string `json:"broadcasterId,omitempty"`
}

type CreateBroadcasterTransportRequest struct {
	BroadcasterId    string                      `json:"broadcasterId,omitempty"`
	Type             string                      `json:"type,omitempty"`
	RtcpMux          *bool                       `json:"rtcpMux,omitempty"`
	Comedia          bool                        `json:"comedia,omitempty"`
	SctpCapabilities *mediasoup.SctpCapabilities `json:"sctpCapabilities,omitempty"`
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

type CreateBroadcasterProducerRequest struct {
	BroadcasterId string                  `json:"broadcasterId,omitempty"`
	TransportId   string                  `json:"transportId,omitempty"`
	Kind          string                  `json:"kind,omitempty"`
	RtpParameters mediasoup.RtpParameters `json:"rtpParameters,omitempty"`
}

type CreateBroadcasterProducerResponse struct {
	Id string `json:"id,omitempty"`
}

type CreateBroadcasterConsumerRequest struct {
	BroadcasterId string `json:"broadcasterId,omitempty"`
	TransportId   string `json:"transportId,omitempty"`
	ProducerId    string `json:"producerId,omitempty"`
}

type CreateBroadcasterConsumerResponse struct {
	Id            string                  `json:"id,omitempty"`
	ProducerId    string                  `json:"producerId,omitempty"`
	Kind          string                  `json:"kind,omitempty"`
	RtpParameters mediasoup.RtpParameters `json:"rtpParameters,omitempty"`
	Type          string                  `json:"type,omitempty"`
}

type CreateBroadcasterDataConsumerRequest struct {
	BroadcasterId  string `json:"broadcasterId,omitempty"`
	TransportId    string `json:"transportId,omitempty"`
	DataProducerId string `json:"dataProducerId,omitempty"`
}

type CreateBroadcasterDataConsumerResponse struct {
	Id string `json:"id,omitempty"`
}

type CreateBroadcasterDataProducerRequest struct {
	BroadcasterId        string                          `json:"broadcasterId,omitempty"`
	TransportId          string                          `json:"transportId,omitempty"`
	Label                string                          `json:"label,omitempty"`
	Protocol             string                          `json:"protocol,omitempty"`
	SctpStreamParameters *mediasoup.SctpStreamParameters `json:"sctpStreamParameters,omitempty"`
	AppData              interface{}                     `json:"appData,omitempty"`
}

type CreateBroadcasterDataProducerResponse struct {
	Id string `json:"id,omitempty"`
}
