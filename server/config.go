package main

import (
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jiyeyuran/mediasoup-go"
	"github.com/jiyeyuran/mediasoup-go/h264"
)

type Config struct {
	Domain    string          `json:"domain,omitempty"`
	Https     HTTPSConfig     `json:"https,omitempty"`
	Mediasoup MediasoupConfig `json:"mediasoup,omitempty"`
}

type HTTPSConfig struct {
	ListenIp   string    `json:"listenIP,omitempty"`
	ListenPort uint16    `json:"listenPort,omitempty"`
	TLS        TLSConfig `json:"tls,omitempty"`
}

type TLSConfig struct {
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

type MediasoupConfig struct {
	NumWorkers               int                             `json:"numWorkers,omitempty"`
	WorkerSettings           mediasoup.WorkerSettings        `json:"workerSettings,omitempty"`
	RouterOptions            mediasoup.RouterOptions         `json:"routerOptions,omitempty"`
	WebRtcTransportOptions   WebRtcTransportOptions          `json:"webRtcTransportOptions,omitempty"`
	PlainRtpTransportOptions mediasoup.PlainTransportOptions `json:"plainRtpTransportOptions,omitempty"`
}

type WebRtcTransportOptions struct {
	mediasoup.WebRtcTransportOptions
	// Additional options that are not part of WebRtcTransportOptions.
	MaxIncomingBitrate int `json:"maxIncomingBitrate,omitempty"`
}

var (
	dirname, _    = filepath.Abs(filepath.Dir(os.Args[0]))
	DefaultConfig = Config{
		Domain: "localhost",
		Https: HTTPSConfig{
			ListenIp:   "0.0.0.0",
			ListenPort: 4443,
			TLS: TLSConfig{
				Cert: filepath.Join(dirname, "certs", "localhost.pem"),
				Key:  filepath.Join(dirname, "certs", "localhost.key"),
			},
		},
		Mediasoup: MediasoupConfig{
			NumWorkers: runtime.NumCPU(),
			WorkerSettings: mediasoup.WorkerSettings{
				LogLevel: "warn",
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
				RtcMinPort: 40000,
				RtcMaxPort: 49999,
			},
			RouterOptions: mediasoup.RouterOptions{
				MediaCodecs: []*mediasoup.RtpCodecCapability{
					{
						Kind:      mediasoup.MediaKind_Audio,
						MimeType:  "audio/opus",
						ClockRate: 48000,
						Channels:  2,
					},
					{
						Kind:      mediasoup.MediaKind_Video,
						MimeType:  "video/VP8",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							XGoogleStartBitrate: 1000,
						},
					},
					{
						Kind:      mediasoup.MediaKind_Video,
						MimeType:  "video/VP9",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							ProfileId:           "2",
							XGoogleStartBitrate: 1000,
						},
					},
					{
						Kind:      mediasoup.MediaKind_Video,
						MimeType:  "video/h264",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							RtpParameter: h264.RtpParameter{
								PacketizationMode:     1,
								ProfileLevelId:        "42e01f",
								LevelAsymmetryAllowed: 1,
							},
							XGoogleStartBitrate: 1000,
						},
					},
					{
						Kind:      mediasoup.MediaKind_Video,
						MimeType:  "video/h264",
						ClockRate: 90000,
						Parameters: mediasoup.RtpCodecSpecificParameters{
							RtpParameter: h264.RtpParameter{
								PacketizationMode:     1,
								ProfileLevelId:        "4d0032",
								LevelAsymmetryAllowed: 1,
							},
							XGoogleStartBitrate: 1000,
						},
					},
				},
			},
			WebRtcTransportOptions: WebRtcTransportOptions{
				WebRtcTransportOptions: mediasoup.WebRtcTransportOptions{
					ListenIps: []mediasoup.TransportListenIp{
						{
							Ip:          GetOutboundIP(),
							AnnouncedIp: GetOutboundIP(),
						},
					},
					InitialAvailableOutgoingBitrate: 1000000,
					MaxSctpMessageSize:              262144,
				},
				MaxIncomingBitrate: 1500000,
			},
			PlainRtpTransportOptions: mediasoup.PlainTransportOptions{
				ListenIp: mediasoup.TransportListenIp{
					Ip:          GetOutboundIP(),
					AnnouncedIp: GetOutboundIP(),
				},
				MaxSctpMessageSize: 262144,
			},
		},
	}
)

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
