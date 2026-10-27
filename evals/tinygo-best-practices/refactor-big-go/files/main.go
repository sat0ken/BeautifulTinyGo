// A weather station for the Raspberry Pi Pico, written the way one would
// write a small server. It works on the bench for a few minutes.
package main

import (
	"encoding/json"
	"fmt"
	"machine"
	"strings"
	"time"
)

type Config struct {
	Name     string  `json:"name"`
	Interval int     `json:"interval_ms"`
	Offset   float64 `json:"temp_offset"`
}

var configJSON = `{"name":"balcony","interval_ms":500,"temp_offset":-0.5}`

type Reading struct {
	Kind  string
	Value float64
}

func readTemperature(out chan<- Reading, offset float64) {
	adc := machine.ADC{Pin: machine.ADC0}
	adc.Configure(machine.ADCConfig{})
	for {
		raw := adc.Get()
		volts := float64(raw) / 1023.0 * 3.3
		out <- Reading{"temp", (volts-0.5)*100 + offset}
	}
}

func readLight(out chan<- Reading) {
	adc := machine.ADC{Pin: machine.ADC1}
	adc.Configure(machine.ADCConfig{})
	for {
		out <- Reading{"light", float64(adc.Get()) / 1023.0 * 100}
	}
}

func waitForSensor(ready machine.Pin) {
	for !ready.Get() {
	}
}

func main() {
	var cfg Config
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		panic(err)
	}
	machine.InitADC()

	ready := machine.GP15
	ready.Configure(machine.PinConfig{Mode: machine.PinInput})
	waitForSensor(ready)

	readings := make(chan Reading)
	go readTemperature(readings, cfg.Offset)
	go readLight(readings)

	latest := map[string]float64{}
	history := []string{}
	for {
		r := <-readings
		latest[r.Kind] = r.Value
		line := fmt.Sprintf("%s: temp=%.1fC light=%.0f%%", cfg.Name, latest["temp"], latest["light"])
		history = append(history, line)
		if len(history) > 10 {
			history = history[1:]
		}
		fmt.Println(strings.ToUpper(line))
		time.Sleep(time.Duration(cfg.Interval) * time.Millisecond)
	}
}
