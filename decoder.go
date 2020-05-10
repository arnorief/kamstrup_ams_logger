package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"

	"github.com/ghostiam/binstruct"
)

func decodeData(buf bytes.Buffer) error {
	log.Printf("%s", hex.Dump(buf.Bytes()))

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

	log.Printf("Meter data: %v\n\n", meter)

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
