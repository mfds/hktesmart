package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alpr777/homekit"
	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/characteristic"
	tesmart "github.com/mfds/tesmart-commands"
)

type config struct {
	host         string
	accessoryPin string
	dbDir        string
}

func getConfig() config {
	var ok bool
	var host string
	var accessoryPin string
	var dbDir string

	if host, ok = os.LookupEnv("TESMART_HOST"); !ok {
		host = "192.168.1.10:5000"
	}

	if accessoryPin, ok = os.LookupEnv("TESMART_PIN"); !ok {
		accessoryPin = "16161616"
	}

	if dbDir, ok = os.LookupEnv("TESMART_DIR"); !ok {
		dbDir = "./db"
	}

	return config{
		host:         host,
		accessoryPin: accessoryPin,
		dbDir:        dbDir,
	}
}

func main() {
	c := getConfig()

	t, err := tesmart.NewTesmartSwitch(c.host)
	if err != nil {
		log.Panicf("Cannot connect to switch: %v", err)
	}

	activeInput, err := t.GetCurrentInput()
	if err != nil {
		log.Panicf("Cannot get current input: %v", err)
	}

	accessoryInfo := accessory.Info{
		Name:             "TESmart 16x1 4K HDMI Switch",
		SerialNumber:     "N/A",
		Manufacturer:     "TESmart",
		Model:            "HSW1601A1U-UKBK",
		FirmwareRevision: "1.0",
	}

	acc := homekit.NewAccessoryTelevision(accessoryInfo)

	// Create input sources
	for i := 1; i <= 16; i++ {
		acc.AddInputSource(i, fmt.Sprintf("HDMI %d", i), characteristic.InputSourceTypeHdmi)
	}

	// Show accessory as always active
	acc.Television.Active.SetValue(1)
	go acc.Television.Active.OnValueRemoteUpdate(func(v int) {
		acc.Television.Active.SetValue(1)
	})

	// Set current active input
	acc.Television.ActiveIdentifier.SetValue(activeInput)

	// Create transport
	transportConfig := hc.Config{
		StoragePath: c.dbDir,
		Pin:         c.accessoryPin,
	}
	transp, err := hc.NewIPTransport(transportConfig, acc.Accessory)
	if err != nil {
		log.Fatalf("accessory [%s / %s]: error creating transport: %v",
			acc.Info.SerialNumber.GetValue(),
			acc.Info.Name.GetValue(),
			err)
	}

	// Handle responses coming from devices
	go func() {
		for {
			newInput, err := tesmart.ExtractInput(<-t.Responses)
			if err != nil {
				log.Panicf("Cannot extract input: %v", err)
			}
			acc.Television.ActiveIdentifier.SetValue(newInput)
		}
	}()

	// Send command to device
	go acc.Television.ActiveIdentifier.OnValueRemoteUpdate(func(v int) {
		t.SwitchInput(v)
	})

	log.Printf("Homekit accessory transport start [%s / %s]",
		acc.Info.SerialNumber.GetValue(),
		acc.Info.Name.GetValue())

	hc.OnTermination(func() {
		<-transp.Stop()
	})
	transp.Start()
}
