package main

import (
	"errors"
	"fmt"

	"github.com/jiyeyuran/mediasoup-demo/internal/proto"
	"github.com/jiyeyuran/mediasoup-go"
)

/**
 * Create a Broadcaster. This is for HTTP API requests (see server.js).
 */
func (r *Room) CreateBroadcaster(request proto.PeerInfo) (rsp H, err error) {
	if _, ok := r.broadcasters.Load(request.Id); ok {
		err = fmt.Errorf(`broadcaster with id "%s" already exists`, request.Id)
		r.logger.Err(err).Send()
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
				if r.mediasoupRouter.CanConsume(producer.Id(), *request.RtpCapabilities) {
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
		r.logger.Err(err).Send()
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
		r.logger.Err(err).Send()
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	switch request.Type {
	case "webrtc":
		webRtcTransportOptions := mediasoup.WebRtcTransportOptions{}
		Clone(&webRtcTransportOptions, r.config.Mediasoup.WebRtcTransportOptions)

		if request.SctpCapabilities != nil {
			webRtcTransportOptions.EnableSctp = true
			webRtcTransportOptions.NumSctpStreams = request.SctpCapabilities.NumStreams
		}
		transport, err := r.mediasoupRouter.CreateWebRtcTransport(webRtcTransportOptions)
		if err != nil {
			return rsp, err
		}

		// Store it.
		broadcaster.Data.AddTransport(transport)

		rsp = H{
			"id":             transport.Id(),
			"iceParameters":  transport.IceParameters(),
			"iceCandidates":  transport.IceCandidates(),
			"dtlsParameters": transport.DtlsParameters(),
			"sctpParameters": transport.SctpParameters(),
		}

	case "plain":
		plainTransportOptions := mediasoup.PlainTransportOptions{}
		Clone(&plainTransportOptions, r.config.Mediasoup.PlainTransportOptions)

		plainTransportOptions.RtcpMux = request.RtcpMux
		plainTransportOptions.Comedia = request.Comedia

		transport, err := r.mediasoupRouter.CreatePlainTransport(plainTransportOptions)
		if err != nil {
			return rsp, err
		}

		// Store it.
		broadcaster.Data.AddTransport(transport)

		rsp = H{
			"id":   transport.Id(),
			"ip":   transport.Tuple().LocalIp,
			"port": transport.Tuple().LocalPort,
		}

		if rtcpTuple := transport.RtcpTuple(); rtcpTuple != nil {
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
		r.logger.Err(err).Send()
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	transport := broadcaster.Data.GetTransport(request.TransportId)
	if transport == nil {
		err = fmt.Errorf(`transport with id "%s" does not exist`, request.BroadcasterId)
		r.logger.Err(err).Send()
		return
	}

	webrtcTransport, ok := transport.(*mediasoup.WebRtcTransport)
	if !ok {
		err = fmt.Errorf(`transport with id "%s" is not a WebRtcTransport`, request.BroadcasterId)
		r.logger.Err(err).Send()
		return
	}

	err = webrtcTransport.Connect(mediasoup.TransportConnectOptions{
		DtlsParameters: &request.DtlsParameters,
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
		r.logger.Err(err).Send()
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	transport := broadcaster.Data.GetTransport(request.TransportId)
	if transport == nil {
		err = fmt.Errorf(`transport with id "%s" does not exist`, request.BroadcasterId)
		r.logger.Err(err).Send()
		return
	}

	producer, err := transport.Produce(mediasoup.ProducerOptions{
		Kind:          mediasoup.MediaKind(request.Kind),
		RtpParameters: request.RtpParameters,
	})
	if err != nil {
		r.logger.Err(err).Str("broadcaster", request.BroadcasterId).Msg("create producer failed")
		return
	}

	// Store it.
	broadcaster.Data.AddProducer(producer)

	producer.OnVideoOrientationChange(func(videoOrientation *mediasoup.ProducerVideoOrientation) {
		r.logger.Debug().
			Str("producerId", producer.Id()).
			Interface("videoOrientation", videoOrientation).
			Msg(`broadcaster producer "videoorientationchange" event`)
	})

	// Optimization: Create a server-side Consumer for each Peer.
	for _, peer := range r.getJoinedPeers() {
		r.createConsumer(peer, broadcaster.Id, producer)
	}

	// Add into the audioLevelObserver.
	if producer.Kind() == mediasoup.MediaKind_Audio {
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
		r.logger.Err(err).Send()
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	if broadcaster.Data.RtpCapabilities == nil {
		err = errors.New("broadcaster does not have rtpCapabilities")
		r.logger.Err(err).Send()
		return
	}

	transport := broadcaster.Data.GetTransport(request.TransportId)
	if transport == nil {
		err = fmt.Errorf(`transport with id "%s" does not exist`, request.BroadcasterId)
		r.logger.Err(err).Send()
		return
	}

	consumer, err := transport.Consume(mediasoup.ConsumerOptions{
		ProducerId:      request.ProducerId,
		RtpCapabilities: *broadcaster.Data.RtpCapabilities,
	})
	if err != nil {
		r.logger.Err(err).Str("broadcaster", request.BroadcasterId).Msg("create consumer failed")
		return
	}

	// Store it.
	broadcaster.Data.AddConsumer(consumer)

	// Set Consumer events.
	consumer.OnClose(func() {
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
		r.logger.Err(err).Send()
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	if broadcaster.Data.RtpCapabilities == nil {
		err = errors.New("broadcaster does not have rtpCapabilities")
		r.logger.Err(err).Send()
		return
	}

	transport := broadcaster.Data.GetTransport(request.TransportId)
	if transport == nil {
		err = fmt.Errorf(`transport with id "%s" does not exist`, request.BroadcasterId)
		r.logger.Err(err).Send()
		return
	}

	dataConsumer, err := transport.ConsumeData(mediasoup.DataConsumerOptions{
		DataProducerId: request.DataProducerId,
	})
	if err != nil {
		r.logger.Err(err).Str("broadcaster", request.BroadcasterId).Msg("create data consumer failed")
		return
	}

	// Store it.
	broadcaster.Data.AddDataConsumer(dataConsumer)

	// Set Consumer events.
	dataConsumer.OnClose(func() {
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
		r.logger.Err(err).Send()
		return
	}
	broadcaster := value.(*proto.PeerInfo)

	transport := broadcaster.Data.GetTransport(request.TransportId)
	if transport == nil {
		err = fmt.Errorf(`transport with id "%s" does not exist`, request.BroadcasterId)
		r.logger.Err(err).Send()
		return
	}

	dataProducer, err := transport.ProduceData(mediasoup.DataProducerOptions{
		SctpStreamParameters: request.SctpStreamParameters,
		Label:                request.Label,
		Protocol:             request.Protocol,
		AppData:              request.AppData,
	})
	if err != nil {
		r.logger.Err(err).Str("broadcaster", request.BroadcasterId).Msg("create producer failed")
		return
	}

	// Store it.
	broadcaster.Data.AddDataProducer(dataProducer)

	dataProducer.On("transportclose", func() {
		// Remove from its map.
		broadcaster.Data.DeleteDataProducer(dataProducer.Id())
	})

	rsp = proto.CreateBroadcasterDataProducerResponse{
		Id: dataProducer.Id(),
	}

	return
}
