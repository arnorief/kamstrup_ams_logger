package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/ghostiam/binstruct"
	"github.com/tarm/serial"
)

type dateTime struct {
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

func main() {
	var outputBuffer bytes.Buffer
	// buffer := make([]byte, 1024)

	data, _ := hex.DecodeString("7ea0e22b2113239ae6e7000f000000000c07e4041a07081400ff80000002190a0e4b616d73747275705f563030303109060101000005ff0a103537303635363732303531303234363209060101600101ff0a1236383431313231424e32343331303130343009060101010700ff06000011f409060101020700ff060000000009060101030700ff060000014009060101040700ff0600000000090601011f0700ff060000072809060101330700ff06000005d309060101470700ff060000021009060101200700ff1200e409060101340700ff1200e609060101480700ff1200e9033c7e")
	outputBuffer.Write(data)

	// stream, err := openSerialDevice("/dev/ttyUSB0")
	// if err != nil {
	// 	log.Fatalf("Error opening serial port: %s", err.Error())
	// }

	// defer stream.Close()

	// log.Println("Serial port opened")

	err := decodeData(outputBuffer)
	if err != nil {
		log.Printf("Error decoding data: %v", err)
	}

	// for {
	// 	numBytes, err := stream.Read(buffer)
	// 	if err != nil && err != io.EOF {
	// 		log.Printf("Error reading data from serial device: %v", err)
	// 	} else if err == io.EOF && outputBuffer.Len() > 0 {
	// 		// Last byte received in this stream
	// 		log.Printf("%d bytes received", outputBuffer.Len())

	// 		err := decodeData(outputBuffer)
	// 		if err != nil {
	// 			log.Printf("Error decoding data: %v", err)
	// 		}

	// 		outputBuffer.Reset()
	// 	}

	// 	if numBytes > 0 {
	// 		bytesRead := buffer[0:numBytes]
	// 		outputBuffer.Write(bytesRead)
	// 	}
	// }
}

func readCPUStats() {
	voltage := 230

	r := strings.NewReader(fmt.Sprintf("kv65 voltage=%d", voltage))

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
	clockLen, _ := reader.ReadUint8()
	fmt.Printf("Clock field length: %d\n", clockLen)

	_, b, err = reader.ReadBytes(int(clockLen))
	if err != nil {
		return err
	}

	var clock dateTime
	if err := binstruct.UnmarshalBE(b, &clock); err != nil {
		return err
	}
	log.Printf("Clock: %v", clock)

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

	for i := 1; i <= (int(structLength)-1)/2; i++ {
		// Each OBIS parameter consists of two elements, the identifier and the value.
		typeField, err := reader.ReadUint8()
		if err != nil || int(typeField) != 9 {
			return fmt.Errorf("unexpected type field")
		}

		if err = decodeObisParameter(reader); err != nil {
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

	return nil
}

func decodeObisParameter(reader binstruct.Reader) error {
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
	valueType, err := reader.ReadUint8()
	if err != nil {
		return err
	}

	var value interface{}

	switch valueType {
	case 6: // unsigned, 4 bytes
		_, byteValue, _ := reader.ReadBytes(4)
		value = byteValue
	case 10: // string
		_, str, err := decodeString(reader)
		if err != nil {
			return err
		}
		value = str
	case 18: // unsigned, 2 bytes
		_, byteValue, _ := reader.ReadBytes(2)
		value = byteValue
	}

	log.Printf("Value: %v", value)

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
