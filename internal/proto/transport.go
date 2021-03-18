package proto

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
