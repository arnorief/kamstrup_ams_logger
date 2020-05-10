package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

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

	resp, err := http.Post(*influxURL+"/write?db="+*dbname, "application/x-www-form-urlencoded", r)
	if err != nil {
		log.Println(err.Error())
	}

	_ = resp.Close
}
