package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/jiyeyuran/mediasoup-demo/internal/proto"
	"github.com/jiyeyuran/mediasoup-go"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	uuid "github.com/satori/go.uuid"
)

type TransportInfo = proto.CreateBroadcasterTransportResponse

type Broadcaster struct {
	context       context.Context
	cancel        func()
	serverURL     string
	roomID        string
	mediaFile     string
	broadcasterID string
	audioSsrc     uint32
	audioPt       uint8
	videoSsrc     uint32
	videoPt       uint8
	httpClient    http.Client
	logger        zerolog.Logger
	waitGroup     *sync.WaitGroup
}

func NewBroadcaster(opts *Options) *Broadcaster {
	broadcasterID := uuid.NewV4().String()
	context, cancel := context.WithCancel(context.Background())

	generateSsrc := func() uint32 {
		return uint32(rand.Int31n(9000000) + 1000000)
	}

	return &Broadcaster{
		context:       context,
		cancel:        cancel,
		serverURL:     opts.ServerURL,
		roomID:        opts.RoomID,
		mediaFile:     opts.MediaFile,
		broadcasterID: broadcasterID,
		audioSsrc:     generateSsrc(),
		audioPt:       100,
		videoSsrc:     generateSsrc(),
		videoPt:       101,
		httpClient: http.Client{
			Timeout: 3 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: !opts.VerifyCert},
			},
		},
		logger: zerolog.New(os.Stderr).With().Timestamp().
			Str("room", opts.RoomID).
			Str("broadcaster", broadcasterID).
			Logger().Level(zerolog.InfoLevel),
		waitGroup: &sync.WaitGroup{},
	}
}

func (b *Broadcaster) Run() (err error) {
	b.waitGroup.Add(1)
	defer b.waitGroup.Done()

	if err = b.verifyRoom(); err != nil {
		return
	}
	if err = b.createBroadcaster(); err != nil {
		return
	}
	defer b.deleteBroadcaster()

	audioTransport, err := b.createPlainTransport()
	if err != nil {
		return
	}
	videoTransport, err := b.createPlainTransport()
	if err != nil {
		return
	}
	if err = b.createAudioProducer(audioTransport.Id); err != nil {
		return
	}
	if err = b.createVideoProducer(videoTransport.Id); err != nil {
		return
	}

	go func() {
		<-b.context.Done()

		if b.context.Err() != context.Canceled {
			b.logger.Err(b.context.Err()).Msg("ffmpeg failed")
		}
	}()

	err = b.runFFmpeg(audioTransport, videoTransport)

	return
}

func (b *Broadcaster) Stop() {
	b.cancel()
	b.waitGroup.Wait()
}

func (b *Broadcaster) verifyRoom() (err error) {
	b.logger.Info().Msg("verifying the room exists...")

	_, err = b.request(http.MethodGet, "/rooms/"+b.roomID, nil)

	return
}

func (b *Broadcaster) createBroadcaster() (err error) {
	b.logger.Info().Msg("creating Broadcaster...")

	request := proto.PeerInfo{
		Id:          b.broadcasterID,
		DisplayName: fmt.Sprintf("Broadcaster|%s@%s", time.Now().Format("15:04:05"), GetOutboundIP()),
		Device: proto.DeviceInfo{
			Flag: "broadcaster",
			Name: "FFmpeg",
		},
	}
	_, err = b.request(http.MethodPost, fmt.Sprintf("/rooms/%s/broadcasters", b.roomID), request)
	return
}

func (b *Broadcaster) deleteBroadcaster() (err error) {
	b.logger.Info().Msg("deleting Broadcaster...")

	_, err = b.request(http.MethodDelete, fmt.Sprintf("/rooms/%s/broadcasters/%s", b.roomID, b.broadcasterID), nil)
	return
}

func (b *Broadcaster) createPlainTransport() (resp proto.CreateBroadcasterTransportResponse, err error) {
	b.logger.Info().Msg("creating mediasoup PlainTransport for producing audio/video...")

	rtcpMux := false

	request := proto.CreateBroadcasterTransportRequest{
		Type:    "plain",
		Comedia: true,
		RtcpMux: &rtcpMux,
	}

	data, err := b.request(http.MethodPost, fmt.Sprintf("/rooms/%s/broadcasters/%s/transports", b.roomID, b.broadcasterID), request)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &resp)

	return
}

func (b *Broadcaster) createAudioProducer(transportID string) (err error) {
	b.logger.Info().Msg("creating mediasoup audio Producer...")

	request := proto.CreateBroadcasterProducerRequest{
		Kind: "audio",
		RtpParameters: mediasoup.RtpParameters{
			Codecs: []*mediasoup.RtpCodecParameters{
				{
					MimeType:    "audio/opus",
					PayloadType: b.audioPt,
					ClockRate:   48000,
					Channels:    2,
					Parameters: mediasoup.RtpCodecSpecificParameters{
						SpropStereo: 1,
					},
				},
			},
			Encodings: []mediasoup.RtpEncodingParameters{
				{
					Ssrc: b.audioSsrc,
				},
			},
		},
	}

	_, err = b.request(
		http.MethodPost,
		fmt.Sprintf("/rooms/%s/broadcasters/%s/transports/%s/producers", b.roomID, b.broadcasterID, transportID),
		request)

	return
}

func (b *Broadcaster) createVideoProducer(transportID string) (err error) {
	b.logger.Info().Msg("creating mediasoup video Producer...")

	request := proto.CreateBroadcasterProducerRequest{
		Kind: "video",
		RtpParameters: mediasoup.RtpParameters{
			Codecs: []*mediasoup.RtpCodecParameters{
				{
					MimeType:    "video/vp8",
					PayloadType: b.videoPt,
					ClockRate:   90000,
				},
			},
			Encodings: []mediasoup.RtpEncodingParameters{
				{
					Ssrc: b.videoSsrc,
				},
			},
		},
	}

	_, err = b.request(
		http.MethodPost,
		fmt.Sprintf("/rooms/%s/broadcasters/%s/transports/%s/producers", b.roomID, b.broadcasterID, transportID),
		request)

	return
}

func (b *Broadcaster) runFFmpeg(audioTransport, videoTransport TransportInfo) (err error) {
	args := []string{}

	if strings.HasSuffix(b.mediaFile, ".webm") {
		args = []string{
			"-re",
			"-v", "info",
			"-stream_loop", "-1",
			"-i", b.mediaFile,
			"-map", "0:a:0",
			"-acodec", "libopus", "-ab", "128k", "-ac", "2", "-ar", "48000",
			"-map", "0:v:0",
			"-vcodec", "copy",
		}
	} else {
		args = []string{
			"-re",
			"-v", "info",
			"-stream_loop", "-1",
			"-i", b.mediaFile,
			"-map", "0:a:0",
			"-acodec", "libopus", "-ab", "128k", "-ac", "2", "-ar", "48000",
			"-map", "0:v:0",
			"-pix_fmt", "yuv420p", "-c:v", "libvpx", "-b:v", "1000k", "-deadline", "realtime", "-cpu-used", "4",
		}
	}

	args = append(args, "-f", "tee", fmt.Sprintf(
		"[select=a:f=rtp:ssrc=%d:payload_type=%d]rtp://%s:%d?rtcpport=%d|[select=v:f=rtp:ssrc=%d:payload_type=%d]rtp://%s:%d?rtcpport=%d",
		b.audioSsrc, b.audioPt, audioTransport.Ip, audioTransport.Port, audioTransport.RtcpPort,
		b.videoSsrc, b.videoPt, videoTransport.Ip, videoTransport.Port, videoTransport.RtcpPort,
	))

	b.logger.Info().Msgf("runing ffmpeg %s", strings.Join(args, " "))

	cmd := exec.CommandContext(b.context, "ffmpeg", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return errors.WithStack(cmd.Run())
}

func (b *Broadcaster) request(method string, url string, data interface{}) (result []byte, err error) {
	var body io.Reader

	if data != nil {
		data, _ := json.Marshal(data)
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, b.serverURL+url, body)
	if err != nil {
		err = errors.WithStack(err)
		return
	}

	req.WithContext(b.context)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	defer resp.Body.Close()

	result, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		err = errors.WithStack(err)
		return
	}

	if resp.StatusCode >= 400 {
		var errResult struct {
			Error string
		}
		json.Unmarshal(result, &errResult)

		err = errors.WithStack(errors.New(errResult.Error))
	}

	return
}

// Get preferred outbound ip of this machine
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}
