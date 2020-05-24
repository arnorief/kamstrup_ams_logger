# kamstrup_ams_logger

## Purpose

This program reads and decodes data received from a Kamstrup AMS power meter's HAN port. The decoded result is written to an Influx database via HTTP post messages.

The program is written in Golang which makes it usable on various architectures.

## Hardware and software

The program was developed and tested using a Raspberry Pi 3B running Raspbian GNU/Linux 10 (buster). In addition to the Raspberry Pi a M-Bus to USB converter is needed in order to read data from the meter's HAN port using the M-Bus protocol.

For logging of the converted data InfluxDB was used and Grafana for visualization.

## Protocol

The data from the HAN port follows the DLMS (Device Language Message Specification) protocol and is sent inside HDLC frames and contains OBIS (Object Identification System) codes that describes the electricity usage. Everything is part of IEC 62056 which is a set of standards for electricity metering data exchange.

Refer: https://www.kode24.no/guider/smart-meter-part-1-getting-the-meter-data/71287300

## Usage

\<path to executable\>/kamstrup_ams_logger [-device SERIAL_DEVICE] [-url INFLUX_URL] [-dbname DATABSE_NAME] [-log LOGFILE]

The parameters are optional, and their default values are as follows:
* SERIAL_DEVICE: /dev/ttyUSB0
* INFLUX_URL: http://localhost:8086
* DATABASE_NAME: meter
* LOGFILE: stdout

The program logs by default to STDOUT.

## Meter data

The meter data is logged to the specified Influx database, measurement 'data' and key 'meter'. The read meter ID is used as key. The following fields are logged:

    fieldKey             fieldType
    --------             ---------
    active_power_minus   float
    active_power_plus    float
    l1_current           float
    l1_voltage           float
    l2_current           float
    l2_voltage           float
    l3_current           float
    l3_voltage           float
    reactive_power_minus float
    reactive_power_plus  float