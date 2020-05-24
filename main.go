package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"os"
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

var device *string
var influxURL *string
var dbname *string
var logfile *string

var meter meterDataT

func main() {
	var outputBuffer bytes.Buffer
	buffer := make([]byte, 1024)

	device = flag.String("device", "/dev/ttyUSB0", "serial device name")
	influxURL = flag.String("url", "http://localhost:8086", "InfluxDB URL")
	dbname = flag.String("dbname", "meter", "InfluxDB database name")
	logfile = flag.String("log", "", "Debug log")
	flag.Parse()

	if *logfile != "" {
		f, err := os.OpenFile(*logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Error opening file: %v", err)
		}
		log.SetOutput(f)
		defer f.Close()
	} else {
		log.SetOutput(os.Stdout)
	}

	stream, err := openSerialDevice(*device)
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
