package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jiyeyuran/mediasoup-demo/internal/proto"
	"github.com/jiyeyuran/mediasoup-go/v2"
)

/**
 * Create a Broadcaster. This is for HTTP API requests (see server.js).
 */
func (r *Room) CreateBroadcaster(request proto.PeerInfo) (rsp H, err error) {
	if _, ok := r.broadcasters.Load(request.Id); ok {
		err = fmt.Errorf(`broadcaster with id "%s" already exists`, request.Id)
		r.logger.Error(err.Error())
		return
	}

	broadcaster := &request
	broadcaster.Data = request.CreatePeerData()

	// Store the Broadcaster into the map.
	r.broadcasters.Store(broadcaster.Id, broadcaster)

	// Notify the new Broadcaster to all Peers.
	for _, peer := range r.getJoinedPeers() {
		peer.Notify("newPeer", H{
			"id":          broadcaster.Id,
			"displayName": broadcaster.Data.DisplayName,
			"device":      broadcaster.Data.Device,
		})
	}

	type ProducerInfo struct {
		Id   string
		Kind string
	}
	type PeerInfo struct {
		proto.PeerInfo
		Producers []ProducerInfo
	}

	peerInfos := []PeerInfo{}

	// Just fill the list of Peers if the Broadcaster provided its rtpCapabilities.
	if request.RtpCapabilities != nil {
		for _, joinedPeer := range r.getJoinedPeers() {
			peerData := joinedPeer.Data().(*proto.PeerData)
			peerInfo := PeerInfo{
				PeerInfo: proto.PeerInfo{
					Id:          joinedPeer.Id(),
					DisplayName: peerData.DisplayName,
					Device:      peerData.Device,
				},
				Producers: []ProducerInfo{},
			}
			for _, producer := range peerData.Producers() {
				// Ignore Producers that the Broadcaster cannot consume.
				if r.router.CanConsume(producer.Id(), request.RtpCapabilities) {
					peerInfos = append(peerInfos, peerInfo)
					peerInfo.Producers = append(peerInfo.Producers, ProducerInfo{
						Id:   producer.Id(),
						Kind: string(producer.Kind()),
					})
				}
			}
			peerInfos = append(peerInfos, peerInfo)
		}
	}

	rsp = H{
		"peers": peerInfos,
	}

	return
}

/**
 * Delete a Broadcaster.
 */
func (r *Room) DeleteBroadcaster(broadcasterId string) (err error) {
	broadcaster, ok := r.broadcasters.LoadAndDelete(broadcasterId)

	if !ok {
		err = fmt.Errorf(`broadcaster with id "%s" does not exists`, broadcasterId)
		r.logger.Error(err.Error())
		return
	}

	for _, transport := range broadcaster.(*proto.PeerInfo).Data.Transports() {
		transport.Close()
	}

	for _, peer := range r.getJoinedPeers() {
		peer.Notify("peerClosed", H{
			"peerId": broadcasterId,
		})
	}

	return
}

/**
 * Create a mediasoup Transport associated to a Broadcaster. It can be a
 * PlainTransport or a WebRtcTransport.
 */
func (r *Room) CreateBroadcasterTransport(request proto.CreateBroadcasterTransportRequest) (rsp H, err error) {
	value, ok := r.broadcasters.Load(request.BroadcasterId)
	if !ok {
		err = fmt.Errorf(`broadcaster with id "%s" does not exists`, request.BroadcasterId)
		r.logger.Error(err.Error())
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	switch request.Type {
	case "webrtc":
		webRtcTransportOptions := &mediasoup.WebRtcTransportOptions{}
		Clone(&webRtcTransportOptions, r.config.Mediasoup.WebRtcTransportOptions)

		if request.SctpCapabilities != nil {
			webRtcTransportOptions.EnableSctp = true
			webRtcTransportOptions.NumSctpStreams = &request.SctpCapabilities.NumStreams
		}
		transport, err := r.router.CreateWebRtcTransport(webRtcTransportOptions)
		if err != nil {
			return rsp, err
		}

		// Store it.
		broadcaster.Data.AddTransport(transport)

		data := transport.Data().WebRtcTransportData

		rsp = H{
			"id":             transport.Id(),
			"iceParameters":  data.IceParameters,
			"iceCandidates":  data.IceCandidates,
			"dtlsParameters": data.DtlsParameters,
			"sctpParameters": data.SctpParameters,
		}

	case "plain":
		plainTransportOptions := &mediasoup.PlainTransportOptions{}
		Clone(&plainTransportOptions, r.config.Mediasoup.PlainTransportOptions)

		plainTransportOptions.RtcpMux = request.RtcpMux
		plainTransportOptions.Comedia = request.Comedia

		transport, err := r.router.CreatePlainTransport(plainTransportOptions)
		if err != nil {
			return rsp, err
		}

		// Store it.
		broadcaster.Data.AddTransport(transport)

		data := transport.Data().PlainTransportData

		rsp = H{
			"id":   transport.Id(),
			"ip":   data.Tuple.LocalAddress,
			"port": data.Tuple.LocalPort,
		}

		if rtcpTuple := data.RtcpTuple; rtcpTuple != nil {
			rsp["rtcpPort"] = rtcpTuple.LocalPort
		}
	}

	return
}

/**
 * Connect a Broadcaster mediasoup WebRtcTransport.
 */
func (r *Room) ConnectBroadcasterTransport(request proto.ConnectBroadcasterTransportRequest) (err error) {
	value, ok := r.broadcasters.Load(request.BroadcasterId)
	if !ok {
		err = fmt.Errorf(`broadcaster with id "%s" does not exists`, request.BroadcasterId)
		r.logger.Error(err.Error())
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	transport := broadcaster.Data.GetTransport(request.TransportId)
	if transport == nil {
		err = fmt.Errorf(`transport with id "%s" does not exist`, request.BroadcasterId)
		r.logger.Error(err.Error())
		return
	}

	err = transport.Connect(&mediasoup.TransportConnectOptions{
		DtlsParameters: request.DtlsParameters,
	})

	return
}

/**
 * Create a mediasoup Producer associated to a Broadcaster.
 */
func (r *Room) CreateBroadcasterProducer(request proto.CreateBroadcasterProducerRequest) (rsp proto.CreateBroadcasterProducerResponse, err error) {
	value, ok := r.broadcasters.Load(request.BroadcasterId)
	if !ok {
		err = fmt.Errorf(`broadcaster with id "%s" does not exists`, request.BroadcasterId)
		r.logger.Error(err.Error())
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	transport := broadcaster.Data.GetTransport(request.TransportId)
	if transport == nil {
		err = fmt.Errorf(`transport with id "%s" does not exist`, request.BroadcasterId)
		r.logger.Error(err.Error())
		return
	}

	producer, err := transport.Produce(&mediasoup.ProducerOptions{
		Kind:          mediasoup.MediaKind(request.Kind),
		RtpParameters: request.RtpParameters,
	})
	if err != nil {
		r.logger.Error("create producer failed", "broadcaster", request.BroadcasterId, "error", err)
		return
	}

	// Store it.
	broadcaster.Data.AddProducer(producer)

	producer.OnVideoOrientationChange(func(videoOrientation mediasoup.ProducerVideoOrientation) {
		r.logger.Debug(`broadcaster producer "videoorientationchange" event`,
			"producerId", producer.Id(), "videoOrientation", videoOrientation)
	})

	// Optimization: Create a server-side Consumer for each Peer.
	for _, peer := range r.getJoinedPeers() {
		r.createConsumer(peer, broadcaster.Id, producer)
	}

	// Add into the audioLevelObserver.
	if producer.Kind() == mediasoup.MediaKindAudio {
		r.audioLevelObserver.AddProducer(producer.Id())
	}

	rsp = proto.CreateBroadcasterProducerResponse{
		Id: producer.Id(),
	}
	return
}

func (r *Room) CreateBroadcasterConsumer(request proto.CreateBroadcasterConsumerRequest) (rsp proto.CreateBroadcasterConsumerResponse, err error) {
	value, ok := r.broadcasters.Load(request.BroadcasterId)
	if !ok {
		err = fmt.Errorf(`broadcaster with id "%s" does not exists`, request.BroadcasterId)
		r.logger.Error(err.Error())
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	if broadcaster.Data.RtpCapabilities == nil {
		err = errors.New("broadcaster does not have rtpCapabilities")
		r.logger.Error(err.Error())
		return
	}

	transport := broadcaster.Data.GetTransport(request.TransportId)
	if transport == nil {
		err = fmt.Errorf(`transport with id "%s" does not exist`, request.BroadcasterId)
		r.logger.Error(err.Error())
		return
	}

	consumer, err := transport.Consume(&mediasoup.ConsumerOptions{
		ProducerId:      request.ProducerId,
		RtpCapabilities: broadcaster.Data.RtpCapabilities,
	})
	if err != nil {
		r.logger.Error("create consumer failed", "broadcaster", request.BroadcasterId, "error", err)
		return
	}

	// Store it.
	broadcaster.Data.AddConsumer(consumer)

	// Set Consumer events.
	consumer.OnClose(func(ctx context.Context) {
		// Remove from its map.
		broadcaster.Data.DeleteConsumer(consumer.Id())
	})
	rsp = proto.CreateBroadcasterConsumerResponse{
		Id:            consumer.Id(),
		ProducerId:    request.ProducerId,
		Kind:          string(consumer.Kind()),
		RtpParameters: consumer.RtpParameters(),
		Type:          string(consumer.Type()),
	}

	return
}

func (r *Room) CreateBroadcasterDataConsumer(request proto.CreateBroadcasterDataConsumerRequest) (rsp proto.CreateBroadcasterDataConsumerResponse, err error) {
	value, ok := r.broadcasters.Load(request.BroadcasterId)
	if !ok {
		err = fmt.Errorf(`broadcaster with id "%s" does not exists`, request.BroadcasterId)
		r.logger.Error(err.Error())
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	if broadcaster.Data.RtpCapabilities == nil {
		err = errors.New("broadcaster does not have rtpCapabilities")
		r.logger.Error(err.Error())
		return
	}

	transport := broadcaster.Data.GetTransport(request.TransportId)
	if transport == nil {
		err = fmt.Errorf(`transport with id "%s" does not exist`, request.BroadcasterId)
		r.logger.Error(err.Error())
		return
	}

	dataConsumer, err := transport.ConsumeData(&mediasoup.DataConsumerOptions{
		DataProducerId: request.DataProducerId,
	})
	if err != nil {
		r.logger.Error("create data consumer failed", "broadcaster", request.BroadcasterId, "error", err)
		return
	}

	// Store it.
	broadcaster.Data.AddDataConsumer(dataConsumer)

	// Set Consumer events.
	dataConsumer.OnClose(func(ctx context.Context) {
		// Remove from its map.
		broadcaster.Data.DeleteDataConsumer(dataConsumer.Id())
	})

	rsp = proto.CreateBroadcasterDataConsumerResponse{
		Id: dataConsumer.Id(),
	}

	return
}

func (r *Room) CreateBroadcasterDataProducer(request proto.CreateBroadcasterDataProducerRequest) (rsp proto.CreateBroadcasterDataProducerResponse, err error) {
	value, ok := r.broadcasters.Load(request.BroadcasterId)
	if !ok {
		err = fmt.Errorf(`broadcaster with id "%s" does not exists`, request.BroadcasterId)
		r.logger.Error(err.Error())
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	transport := broadcaster.Data.GetTransport(request.TransportId)
	if transport == nil {
		err = fmt.Errorf(`transport with id "%s" does not exist`, request.BroadcasterId)
		r.logger.Error(err.Error())
		return
	}

	dataProducer, err := transport.ProduceData(&mediasoup.DataProducerOptions{
		SctpStreamParameters: request.SctpStreamParameters,
		Label:                request.Label,
		Protocol:             request.Protocol,
		AppData:              request.AppData,
	})
	if err != nil {
		r.logger.Error("create producer failed", "broadcaster", request.BroadcasterId, "error", err)
		return
	}

	// Store it.
	broadcaster.Data.AddDataProducer(dataProducer)
	dataProducer.OnClose(func(ctx context.Context) {
		broadcaster.Data.DeleteDataProducer(dataProducer.Id())
	})

	rsp = proto.CreateBroadcasterDataProducerResponse{
		Id: dataProducer.Id(),
	}

	return
}
