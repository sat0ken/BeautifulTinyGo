// Package si7021 is a driver for the Silicon Labs Si7021 humidity and
// temperature sensor.
//
// Datasheet: https://www.silabs.com/documents/public/data-sheets/Si7021-A20.pdf
package si7021

import (
	"errors"
	"fmt"
	"machine"
	"time"

	"tinygo.org/x/drivers"
)

// I2C address and commands (datasheet section 5, table 11).
const (
	Address = 0x40

	cmdMeasureRHNoHold = 0xF5
	cmdReadTempFromRH  = 0xE0 // temperature measured during the last RH conversion
	cmdReset           = 0xFE
	cmdReadIDHi        = 0xFC
	cmdReadIDLo        = 0xC9

	deviceIDSi7021 = 0x15 // SNB_3, datasheet section 5.6
)

var errCRC = errors.New("si7021: checksum mismatch")

// Device is an Si7021 on an I2C bus.
type Device struct {
	bus      drivers.I2C
	power    machine.Pin // sensor supply switch, active high
	address  uint16
	heater   bool
	lastTemp float64
}

type Option func(*Device)

func WithAddress(a uint16) Option { return func(d *Device) { d.address = a } }

func WithHeater(on bool) Option { return func(d *Device) { d.heater = on } }

// New returns a driver for the sensor. power switches the sensor's supply.
func New(bus drivers.I2C, power machine.Pin, opts ...Option) *Device {
	d := &Device{bus: bus, power: power, address: Address}
	for _, o := range opts {
		o(d)
	}
	return d
}

// Configure powers the sensor, resets it and checks that it is an Si7021.
func (d *Device) Configure() error {
	d.power.Configure(machine.PinConfig{Mode: machine.PinOutput})
	d.power.High()
	time.Sleep(80 * time.Millisecond) // power-up time, datasheet table 2

	if err := d.bus.Tx(d.address, []byte{cmdReset}, nil); err != nil {
		return fmt.Errorf("si7021: reset at 0x%02x failed: %v", d.address, err)
	}
	time.Sleep(15 * time.Millisecond)

	id := make([]byte, 6)
	if err := d.bus.Tx(d.address, []byte{cmdReadIDHi, cmdReadIDLo}, id); err != nil {
		return fmt.Errorf("si7021: reading id failed: %v", err)
	}
	if id[0] != deviceIDSi7021 {
		return fmt.Errorf("si7021: unexpected device id 0x%02x", id[0])
	}
	return nil
}

// ReadHumidity measures relative humidity in percent. The temperature measured
// during the same conversion is kept and returned by Temperature.
func (d *Device) ReadHumidity() (float64, error) {
	if err := d.bus.Tx(d.address, []byte{cmdMeasureRHNoHold}, nil); err != nil {
		return 0, err
	}
	buf := make([]byte, 3)
	// In no-hold mode the sensor NACKs reads until the conversion is done.
	for d.bus.Tx(d.address, nil, buf) != nil {
	}
	if crc8(buf[:2]) != buf[2] {
		return 0, errCRC
	}
	code := uint16(buf[0])<<8 | uint16(buf[1])
	rh := 125*float64(code)/65536 - 6

	t := make([]byte, 2)
	if err := d.bus.Tx(d.address, []byte{cmdReadTempFromRH}, t); err != nil {
		return 0, err
	}
	tc := uint16(t[0])<<8 | uint16(t[1])
	d.lastTemp = 175.72*float64(tc)/65536 - 46.85
	return rh, nil
}

// Temperature returns the temperature in °C from the last ReadHumidity.
func (d *Device) Temperature() float64 {
	return d.lastTemp
}

// crc8 is the Si7021 checksum: polynomial x^8 + x^5 + x^4 + 1 (0x31), init 0
// (datasheet section 5.1).
func crc8(data []byte) byte {
	var crc byte
	for _, b := range data {
		crc ^= b
		for i := 0; i < 8; i++ {
			if crc&0x80 != 0 {
				crc = crc<<1 ^ 0x31
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
