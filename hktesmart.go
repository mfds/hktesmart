package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alpr777/homekit"
	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/hc/characteristic"
	tesmart "github.com/mfds/tesmart-commands"
)

type config struct {
	host         string
	port         string
	accessoryPin string
	dbDir        string
	inputNames   []string
}

func getConfig() config {
	var ok bool
	var host string
	var port string
	var accessoryPin string
	var dbDir string
	var inputNames []string

	if host, ok = os.LookupEnv("TESMART_HOST"); !ok {
		host = "192.168.1.10"
	}

	if port, ok = os.LookupEnv("TESMART_PORT"); !ok {
		port = "5000"
	}

	if accessoryPin, ok = os.LookupEnv("TESMART_PIN"); !ok {
		accessoryPin = "16161616"
	}

	if dbDir, ok = os.LookupEnv("TESMART_DIR"); !ok {
		dbDir = "./db"
	}

	if names, ok := os.LookupEnv("TESMART_INPUTS"); !ok {
		inputNames = make([]string, 16)
		for i := 0; i < len(inputNames); i++ {
			inputNames[i] = fmt.Sprintf("HDMI %d", i+1)
		}
	} else {
		inputNames = strings.Split(names, ",")
	}

	if len(inputNames) != 16 {
		log.Fatalf("Wrong number of inputs")
	}

	return config{
		host:         host,
		port:         port,
		accessoryPin: accessoryPin,
		dbDir:        dbDir,
		inputNames:   inputNames,
	}
}

func main() {
	c := getConfig()

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
		name := c.inputNames[i-1]
		if name == "" {
			name = fmt.Sprintf("HDMI %d", i)
		}
		acc.AddInputSource(i, name, characteristic.InputSourceTypeHdmi)
	}

	// Show accessory as always active
	acc.Television.Active.SetValue(1)
	go acc.Television.Active.OnValueRemoteUpdate(func(v int) {
		acc.Television.Active.SetValue(1)
	})

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
	receiverFunc := func(data []byte) {
		newInput, err := tesmart.ExtractInput(data)
		if err != nil {
			log.Printf("Cannot extract input: %v %d", err, int(data[5]-data[4]))
			return
		}
		acc.Television.ActiveIdentifier.SetValue(newInput)
	}

	t, err := tesmart.NewTesmartSwitch(c.host, c.port, receiverFunc)
	if err != nil {
		log.Fatalf("Cannot connect to switch: %v", err)
	}

	err = t.SendGetCurrentInput()
	if err != nil {
		log.Fatalf("Cannot get current input: %v", err)
	}

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
