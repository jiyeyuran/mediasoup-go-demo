package main

import (
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jiyeyuran/mediasoup-go/v2"
)

type Config struct {
	Domain    string          `json:"domain,omitempty"`
	Https     HTTPSConfig     `json:"https,omitempty"`
	Mediasoup MediasoupConfig `json:"mediasoup,omitempty"`
}

type HTTPSConfig struct {
	ListenIp   string    `json:"listenIp,omitempty"`
	ListenPort uint16    `json:"listenPort,omitempty"`
	TLS        TLSConfig `json:"tls,omitempty"`
}

type TLSConfig struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

type MediasoupConfig struct {
	NumWorkers             int                              `json:"numWorkers,omitempty"`
	WorkerSettings         *mediasoup.WorkerSettings        `json:"workerSettings,omitempty"`
	RouterOptions          *mediasoup.RouterOptions         `json:"routerOptions,omitempty"`
	WebRtcTransportOptions *WebRtcTransportOptions          `json:"webRtcTransportOptions,omitempty"`
	PlainTransportOptions  *mediasoup.PlainTransportOptions `json:"plainTransportOptions,omitempty"`
}

type WebRtcTransportOptions struct {
	mediasoup.WebRtcTransportOptions
	MaxIncomingBitrate uint32 `json:"maxIncomingBitrate,omitempty"`
}

func NewDefaultConfig() *Config {
	var dirname, _ = filepath.Abs(filepath.Dir(os.Args[0]))

	return &Config{
		Domain: "localhost",
		Https: HTTPSConfig{
			ListenIp:   "0.0.0.0",
			ListenPort: 4443,
			TLS: TLSConfig{
				Cert: filepath.Join(dirname, "certs", "fullchain.pem"),
				Key:  filepath.Join(dirname, "certs", "privkey.pem"),
			},
		},
		Mediasoup: MediasoupConfig{
			NumWorkers: runtime.NumCPU(),
			WorkerSettings: &mediasoup.WorkerSettings{
				LogLevel: mediasoup.WorkerLogLevelWarn,
				LogTags: []mediasoup.WorkerLogTag{
					"info",
					"ice",
					"dtls",
					"rtp",
					"srtp",
					"rtcp",
					"rtx",
					"bwe",
					"score",
					"simulcast",
					"svc",
					"sctp",
				},
			},
			RouterOptions: &mediasoup.RouterOptions{
				MediaCodecs: []*mediasoup.RtpCodecCapability{
					{
						Kind:      mediasoup.MediaKindAudio,
						MimeType:  "audio/opus",
						ClockRate: 48000,
						Channels:  2,
					},
					{
						Kind:      mediasoup.MediaKindVideo,
						MimeType:  "video/VP8",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							XGoogleStartBitrate: 1000,
						},
					},
					{
						Kind:      mediasoup.MediaKindVideo,
						MimeType:  "video/VP9",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							ProfileId:           2,
							XGoogleStartBitrate: 1000,
						},
					},
					{
						Kind:      mediasoup.MediaKindVideo,
						MimeType:  "video/h264",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							PacketizationMode:     1,
							ProfileLevelId:        "42e01f",
							LevelAsymmetryAllowed: 1,
							XGoogleStartBitrate:   1000,
						},
					},
					{
						Kind:      mediasoup.MediaKindVideo,
						MimeType:  "video/h264",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							PacketizationMode:     1,
							ProfileLevelId:        "4d0032",
							LevelAsymmetryAllowed: 1,
							XGoogleStartBitrate:   1000,
						},
					},
				},
			},
			WebRtcTransportOptions: &WebRtcTransportOptions{
				WebRtcTransportOptions: mediasoup.WebRtcTransportOptions{
					ListenInfos: []mediasoup.TransportListenInfo{
						{
							Ip:               GetOutboundIP(),
							AnnouncedAddress: GetOutboundIP(),
						},
					},
				},
				MaxIncomingBitrate: 5000000,
			},
			PlainTransportOptions: &mediasoup.PlainTransportOptions{
				ListenInfo: mediasoup.TransportListenInfo{
					Ip:               GetOutboundIP(),
					AnnouncedAddress: GetOutboundIP(),
				},
				MaxSctpMessageSize: 262144,
			},
		},
	}
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
