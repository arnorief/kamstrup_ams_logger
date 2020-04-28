package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/ghostiam/binstruct"
	"github.com/tarm/serial"
)

type dateTimeT struct {
	Year        uint16
	Month       uint8
	Day         uint8
	Weekday     uint8
	Hour        uint8
	Minute      uint8
	Second      uint8
	Hundreds    uint8
	Deviation   uint16
	ClockStatus uint8
}

type meterDataT struct {
	clock              dateTimeT
	meterID            string
	meterType          string
	activePowerPlus    int
	activePowerMinus   int
	reactivePowerPlus  int
	reactivePowerMinus int
	l1Current          float32
	l2Current          float32
	l3Current          float32
	l1Voltage          int
	l2Voltage          int
	l3Voltage          int
}

var meter meterDataT

func main() {
	var outputBuffer bytes.Buffer
	buffer := make([]byte, 1024)

	stream, err := openSerialDevice("/dev/ttyUSB0")
	if err != nil {
		log.Fatalf("Error opening serial port: %s", err.Error())
	}

	defer stream.Close()

	log.Println("Serial port opened")

	for {
		numBytes, err := stream.Read(buffer)
		if err != nil && err != io.EOF {
			log.Printf("Error reading data from serial device: %v", err)
		} else if err == io.EOF && outputBuffer.Len() > 0 {
			// Last byte received in this stream
			log.Printf("%d bytes received", outputBuffer.Len())

			err := decodeData(outputBuffer)
			if err != nil {
				log.Printf("Error decoding data: %v", err)
			}

			writeToDatabase()
			outputBuffer.Reset()
		}

		if numBytes > 0 {
			bytesRead := buffer[0:numBytes]
			outputBuffer.Write(bytesRead)
		}
	}
}

func writeToDatabase() {
	str := fmt.Sprintf("data,meter=%s active_power_plus=%d\n", meter.meterID, meter.activePowerPlus)
	str += fmt.Sprintf("data,meter=%s active_power_minus=%d\n", meter.meterID, meter.activePowerMinus)
	str += fmt.Sprintf("data,meter=%s reactive_power_plus=%d\n", meter.meterID, meter.reactivePowerPlus)
	str += fmt.Sprintf("data,meter=%s reactive_power_minus=%d\n", meter.meterID, meter.reactivePowerMinus)
	str += fmt.Sprintf("data,meter=%s l1_current=%f\n", meter.meterID, meter.l1Current)
	str += fmt.Sprintf("data,meter=%s l2_current=%f\n", meter.meterID, meter.l2Current)
	str += fmt.Sprintf("data,meter=%s l3_current=%f\n", meter.meterID, meter.l3Current)
	str += fmt.Sprintf("data,meter=%s l1_voltage=%d\n", meter.meterID, meter.l1Voltage)
	str += fmt.Sprintf("data,meter=%s l2_voltage=%d\n", meter.meterID, meter.l2Voltage)
	str += fmt.Sprintf("data,meter=%s l3_voltage=%d\n", meter.meterID, meter.l3Voltage)

	r := strings.NewReader(str)

	resp, err := http.Post("http://localhost:8086/write?db=meter", "application/x-www-form-urlencoded", r)
	if err != nil {
		log.Println(err.Error())
	}

	_ = resp.Close
}

func openSerialDevice(device string) (*serial.Port, error) {
	log.Printf("Trying to open serial port on device %s", device)

	config := &serial.Config{
		Name:        device,
		Baud:        2400,
		Size:        8,
		Parity:      serial.ParityNone,
		StopBits:    serial.Stop1,
		ReadTimeout: 4,
	}

	stream, err := serial.OpenPort(config)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

func decodeData(buf bytes.Buffer) error {
	fmt.Printf("%s", hex.Dump(buf.Bytes()))

	reader := binstruct.NewReaderFromBytes(buf.Bytes(), binary.BigEndian, false)

	// Header
	_, b, err := reader.ReadBytes(8)
	if err != nil {
		log.Fatal(err)
	}
	if hex.EncodeToString(b) != "7ea0e22b2113239a" {
		return fmt.Errorf("invalid header")
	}

	log.Println("Header found")

	// Information header
	_, b, err = reader.ReadBytes(8)
	if err != nil {
		return err
	}
	if hex.EncodeToString(b) != "e6e7000f00000000" {
		return fmt.Errorf("invalid information header")
	}

	log.Println("Information header found")

	// Clock
	var clock dateTimeT
	clockLen, _ := reader.ReadUint8()

	_, b, err = reader.ReadBytes(int(clockLen))
	if err != nil {
		return err
	}
	if err := binstruct.UnmarshalBE(b, &clock); err != nil {
		return err
	}
	log.Printf("Clock: %v", clock)
	meter.clock = clock

	// Struct
	if structInd, _ := reader.ReadUint8(); structInd != 2 {
		return fmt.Errorf("invalid struct indicator")
	}

	structLength, _ := reader.ReadUint8()
	log.Printf("Struct with %d elements", structLength)

	// Version identifier - first element
	if typeField, _ := reader.ReadUint8(); typeField != 10 {
		return fmt.Errorf("unexpected type field")
	}
	length, str, err := decodeString(reader)
	if err != nil {
		return err
	}
	log.Printf("Version identifier (%d): %s", length, str)

	structLength--

	for i := 1; i <= (int(structLength))/2; i++ {
		// Each OBIS parameter consists of two elements, the identifier and the value.
		typeField, err := reader.ReadUint8()
		if err != nil || int(typeField) != 9 {
			return fmt.Errorf("unexpected type field")
		}

		if err = decodeObisField(reader); err != nil {
			return err
		}
	}

	// Frame check sequence
	_, b, err = reader.ReadBytes(2)
	if err != nil {
		return err
	}
	log.Printf("FCS: %s", hex.EncodeToString(b))

	// Frame end flag
	_, b, err = reader.ReadBytes(1)
	if err != nil {
		return err
	}
	if hex.EncodeToString(b) != "7e" {
		return fmt.Errorf("Invalid frame end flag: %s", hex.EncodeToString(b))
	}
	log.Printf("Frame end flag: %s", hex.EncodeToString(b))

	log.Printf("Meter data: %v", meter)

	return nil
}

func decodeObisField(reader binstruct.Reader) error {
	identifierLength, _ := reader.ReadUint8()

	var obisID string

	for i := 1; i <= int(identifierLength); i++ {
		n, err := reader.ReadUint8()
		if err != nil {
			return err
		}
		obisID = obisID + strconv.FormatUint(uint64(n), 10)
		if int(i) < int(identifierLength) {
			obisID = obisID + "."
		}
	}

	log.Printf("OBIS ID: %s", obisID)

	// Value part
	err := decodeObisValue(obisID, reader)
	if err != nil {
		return err
	}

	return nil
}

func decodeObisValue(obisID string, reader binstruct.Reader) error {
	valueType, err := reader.ReadUint8()
	if err != nil {
		return err
	}

	var str string
	var byteValue []byte

	switch valueType {
	case 6: // unsigned, 4 bytes
		_, byteValue, _ = reader.ReadBytes(4)
	case 10: // string
		_, str, err = decodeString(reader)
		if err != nil {
			return err
		}
	case 18: // unsigned, 2 bytes
		_, byteValue, _ = reader.ReadBytes(2)
	}

	valueReader := binstruct.NewReaderFromBytes(byteValue, binary.BigEndian, false)

	switch obisID {
	case "1.1.0.0.5.255": // Meter ID
		meter.meterID = str
		log.Printf("Meter ID: %s", meter.meterID)
	case "1.1.96.1.1.255": // Meter type
		meter.meterType = str
		log.Printf("Meter Type: %s", meter.meterType)
	case "1.1.1.7.0.255": // Active Power +
		v, _ := valueReader.ReadUint32()
		meter.activePowerPlus = int(v)
		log.Printf("Active Power +: %d", meter.activePowerPlus)
	case "1.1.2.7.0.255": // Active Power -
		v, _ := valueReader.ReadUint32()
		meter.activePowerMinus = int(v)
		log.Printf("Active Power -: %d", meter.activePowerMinus)
	case "1.1.3.7.0.255": // Reactive Power +
		v, _ := valueReader.ReadUint32()
		meter.reactivePowerPlus = int(v)
		log.Printf("Reactive Power +: %d", meter.reactivePowerPlus)
	case "1.1.4.7.0.255": // Reactive Power -
		v, _ := valueReader.ReadUint32()
		meter.reactivePowerMinus = int(v)
		log.Printf("Reactive Power -: %d", meter.reactivePowerMinus)
	case "1.1.31.7.0.255": // L1 Current
		v, _ := valueReader.ReadUint32()
		meter.l1Current = float32(v) / 100
		log.Printf("L1 Current: %f", meter.l1Current)
	case "1.1.51.7.0.255": // L2 Current
		v, _ := valueReader.ReadUint32()
		meter.l2Current = float32(v) / 100
		log.Printf("L2 Current: %f", meter.l2Current)
	case "1.1.71.7.0.255": // L3 Current
		v, _ := valueReader.ReadUint32()
		meter.l3Current = float32(v) / 100
		log.Printf("L3 Current: %f", meter.l3Current)
	case "1.1.32.7.0.255": // L1 Voltage
		v, _ := valueReader.ReadUint16()
		meter.l1Voltage = int(v)
		log.Printf("L1 Voltage: %d", meter.l1Voltage)
	case "1.1.52.7.0.255": // L2 Voltage
		v, _ := valueReader.ReadUint16()
		meter.l2Voltage = int(v)
		log.Printf("L2 Voltage: %d", meter.l2Voltage)
	case "1.1.72.7.0.255": // L3 Voltage
		v, _ := valueReader.ReadUint16()
		meter.l3Voltage = int(v)
		log.Printf("L3 Voltage: %d", meter.l3Voltage)
	default:
		log.Println("Unknown OBIS ID")
	}

	return nil
}

func decodeString(reader binstruct.Reader) (int, string, error) {
	length, _ := reader.ReadUint8()

	n, b, err := reader.ReadBytes(int(length))
	if err != nil {
		return 0, "", err
	}

	return n, string(b), nil
}
