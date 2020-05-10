package main

import (
	"log"

	"github.com/tarm/serial"
)

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
