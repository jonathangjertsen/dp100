package dp100

import (
	"fmt"

	"github.com/sstallion/go-hid"
)

// Declaring these as var so users can edit them if they have an unusual device
var (
	VID                uint16 = 0x2e3c
	PID                uint16 = 0xaf01
	ManufacturerString string = "ALIENTEK"
	ProductString      string = "ATK-MDP100"
	DeviceAddress      uint8  = 251
)

type CommandID uint8

// Copied from https://github.com/scottbez1/webdp100/blob/602f1f237542f8f80b6c1227cda6a63e591f1dce/src/app/dp100/hid-reports.ts#L4

const (
	DeviceInfo       CommandID = 16
	FirmwareInfo     CommandID = 17
	StartTransaction CommandID = 18
	DataTransaction  CommandID = 19
	EndTransaction   CommandID = 20
	DeviceUpgrade    CommandID = 21
	BasicInfo        CommandID = 48
	BasicSet         CommandID = 53
	SystemInfo       CommandID = 64
	SystemSet        CommandID = 69
	ScanOut          CommandID = 80
	SerialOut        CommandID = 85
	Disconnect       CommandID = 128
	None             CommandID = 255
)

func ModbusCrc(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for i := range len(data) {
		crc ^= uint16(data[i])
		for j := 8; j != 0; j-- {
			if (crc & 0x0001) != 0 {
				crc >>= 1
				crc ^= 0xA001
			} else {
				crc >>= 1
			}
		}
	}

	return (crc << 8) | (crc >> 8)
}

func WithModbusCrc(data []byte) []byte {
	crc := ModbusCrc(data)
	return append(data, uint8(crc), uint8(crc>>8))
}

type DP100 struct {
	device *hid.Device
}

func NewDP100() (*DP100, error) {
	// go-hid doc recomments calling this 'for concurrent programs'.
	if err := hid.Init(); err != nil {
		return nil, fmt.Errorf("initializing HID library: %w", err)
	}

	// Open HID device
	device, err := hid.OpenFirst(VID, PID)
	if err != nil {
		return nil, fmt.Errorf("opening device with VID=%d, PID=%d: %w", VID, PID, err)
	}

	// Confirm strings (maybe too pedantic)
	manufacturerString, err := device.GetMfrStr()
	if err != nil {
		return nil, fmt.Errorf("retrieving manufacturer string: %w", err)
	}
	if manufacturerString != ManufacturerString {
		return nil, fmt.Errorf("unexpected manufacturer string, expected '%s' got '%s'", ManufacturerString, manufacturerString)
	}
	productString, err := device.GetProductStr()
	if err != nil {
		return nil, fmt.Errorf("retrieving product string: %w", err)
	}
	if productString != ProductString {
		return nil, fmt.Errorf("unexpected product string, expected '%s' got '%s'", ProductString, productString)
	}

	return &DP100{
		device: device,
	}, nil
}

func (dp *DP100) Exec(cmdID CommandID, payload []byte) error {
	featureReport, err := serialize(cmdID, payload)
	if err != nil {
		return fmt.Errorf("building feature report: %w", err)
	}

	_, err = dp.device.Write(featureReport)
	if err != nil {
		return fmt.Errorf("sending feature report %v: %w", featureReport, err)
	}

	buff := make([]byte, 512)
	n, err := dp.device.Read(buff)
	if err != nil {
		return fmt.Errorf("reading from device: %w", err)
	}

	return deserialize(buff[:n])
}

func serialize(cmdID CommandID, payload []byte) ([]byte, error) {
	payloadLen := len(payload)
	if payloadLen > 255 {
		return []byte{}, fmt.Errorf("payload size=%d too big to fit in a byte", len(payload))
	}
	featureReport := []uint8{
		DeviceAddress,
		uint8(cmdID),
		0, // "sequence" in webdp100
		uint8(payloadLen),
	}
	featureReport = append(featureReport, payload...)
	featureReport = WithModbusCrc(featureReport)
	return featureReport, nil
}

func deserialize(buff []byte) error {
	fmt.Printf("buff=%v\n", buff)
	headerSize := 6
	if len(buff) < headerSize {
		return fmt.Errorf("response is too small to contain even a header: %v", buff)
	}
	address := buff[0]
	functionType := buff[1]
	seq := buff[2]
	length := buff[3]

	_ = address
	_ = functionType
	_ = seq

	expectedSize := headerSize + int(length)
	if len(buff) < expectedSize {
		return fmt.Errorf("length byte %d indicates a frame size of at least %d, but buffer is only %d bytes", length, expectedSize, len(buff))
	}

	buff = buff[:expectedSize]
	computedCRC := ModbusCrc(buff[:expectedSize-2])
	receivedCRC := (uint16(buff[expectedSize-2]) << 8) | uint16(buff[expectedSize-1])
	if computedCRC != receivedCRC {
		return fmt.Errorf("crc error: received %x computed %x", receivedCRC, computedCRC)
	}

	return nil
}
