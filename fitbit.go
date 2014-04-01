package main

import (
	"errors"
	"github.com/kylelemons/gousb/usb"
	//	"math/rand"
	"log"
)

type FitbitBase struct {
	ctx     *usb.Context
	dev     *usb.Device
	channel []byte
	base    *ANT
}

func (f *FitbitBase) Open() error {
	f.ctx = usb.NewContext()
	f.ctx.Debug(3)
	devs, err := f.ctx.ListDevices(func(desc *usb.Descriptor) bool {
		if desc.Vendor.String() == "10c4" && desc.Product.String() == "84c4" {

			return true
		}
		return false
	})
	if err == nil {
		if len(devs) == 0 {
			err = errors.New("no devices found")
		} else {
			f.dev = devs[0]
		}
		for i := 1; i < len(devs); i++ {
			devs[i].Close()
		}
	}
	f.channel = []byte{'\x00'}
	return err
}

func (f *FitbitBase) SettingUp() error {
	f.dev.Control(64, 0, 65535, 0, []byte{})
	f.dev.Control(64, 1, 8192, 0, []byte{})
	f.dev.Control(64, 0, 0, 0, []byte{})
	f.dev.Control(64, 0, 65535, 0, []byte{})
	f.dev.Control(64, 1, 8192, 0, []byte{})
	f.dev.Control(64, 1, 74, 0, []byte{})
	f.dev.Control(192, 255, 14091, 0, []byte{'\x01'})
	f.dev.Control(64, 3, 2048, 0, []byte{})
	f.dev.Control(64, 19, 0, 0, []byte{
		'\x08', '\x00', '\x00', '\x00',
		'\x40', '\x00', '\x00', '\x00',
		'\x00', '\x00', '\x00', '\x00',
		'\x00', '\x00', '\x00', '\x00'})
	f.dev.Control(64, 18, 12, 0, []byte{})
	err := f.dev.SetConfig(1)
	if err != nil {
		log.Println("setconfig error")
		return err
	}
	err = f.dev.Reset()
	if err != nil {
		log.Println("reset error")
		return err
	}
	err = f.dev.SetConfig(1)
	if err != nil {
		log.Println("setconfig2 error")
		return err
	}
	f.base = &ANT{
		channel: '\x00',
	}
	f.base.reader, err = f.dev.OpenEndpoint(uint8(1), uint8(0), uint8(0), '\x81')
	if err != nil {
		return err
	}
	f.base.writer, err = f.dev.OpenEndpoint(uint8(1), uint8(0), uint8(0), '\x01')
	return err
}

func (f *FitbitBase) Close() {
	f.dev.Close()
	f.ctx.Close()
}

// data transport
func (f *FitbitBase) InitFitbit() {
	f.InitDeviceChannel([]byte{'\xff', '\xff', '\x01', '\x01'})
}

func (f *FitbitBase) InitDeviceChannel(channel []byte) {
	f.base.Reset()
	f.base.SetNetworkKey(0, []byte{0, 0, 0, 0, 0, 0, 0, 0})
	f.base.AssignChannel()
	f.base.SetChannelPeriod([]byte{'\x00', '\x10'})
	f.base.SetChannelFrequency('\x20')
	f.base.SetTransmitPower('\x30')
	f.base.SetSearchTimeout('\xff')
	f.base.SetChannelId(channel)
	f.base.OpenChannel()
}

/*
func (f *FitbitBase) InitTrackerForTransfer() {
	f.InitFitbit()
	f.WaitForBeacon()
	f.ResetTracker()
	// 0x78 0x02 is device id reset. This tells the device the new
	// channel id to hop to for dumpage
	cid := []byte{byte(rand.Intn(254)), byte(rand.Intn(254))}
	f.base.SendAcknowledgedData('\x78', '\x02', cid, '\x00', '\x00', '\x00', '\x00')
	f.base.CloseChannel()
	f.InitDeviceChannel(append(cid, []byte{0x01, 0x01}...))
	f.WaitForBeacon()
	f.PingTracker()
}
*/
func (f *FitbitBase) ResetTracker() {
	f.base.SendAcknowledgedData([]byte{'\x78', '\x01', '\x00', '\x00', '\x00', '\x00', '\x00', '\x00'})
}

func (f *FitbitBase) CommandSleep() {
	f.base.SendAcknowledgedData([]byte{'\x7f', '\x03', '\x00', '\x00', '\x00', '\x00', '\x00', '\x3c'})
}

/*
func (f *FitbitBase)WaitForBeacon() error {
	data := make([]byte, 1024)
	for i := 0 ;i < 75; i ++ {
		data, err := f.base.ReceiveMessage(4096)
		if err == nil && len(data) > 2 && data[2] == '\x4e' {
			return nil
		}
	}
	return errors.New("Failed to see tracker beacon")
}

func (f *FitbitBase) GetTrackerBurst() ([]byte, error){
	d, err := f.CheckBurstResponse()
	if err !=nil || d[1] != '\x81' {
		return d, errors.New("Response received is not tracker burst! Got")
	}
	size := d[3] << 8 | d[2]
	if size == 0 {
		return d[:0], err
	}
	return d[8:8+size], err
}
func (f *FitbitBase) RunOpcode(opcode, payload []byte)([]byte, error) {
	for i := 0; i < 4; i ++ {
		f.SendTrackerPacket(opcode)
		data, err := f.ReceiveAcknowledgedReply()
		if err != nil {
			continue
		}
		if data[0] != f.currentPacketId {
			log.Printf("Tracker Packet IDs don't match! %v %v \n", f.currentPacketId, data[0])
		}
		if data[1] == '\x42' {
			return f.GetDataBank()
		}
		if data[1] == '\61' {
			if len(payload) >0 {
				f.SendTrackerPayload(payload)
				data, err := f.ReceiveAcknowledgedReply()
				data = data[1:]
				return data, nil
			}
		}
		if data[1] == '0x41' {
			data = data[1:]
			return data, nil
		}
	}
	return []byte{}, errors.New("Failed to run opcode")
}

func (f *FitbitBase)SendTrackerPayload(payload []byte)([]byte, error){
	p = []byte{'\x00', f.genPacketId(), '\x80', byte(len(payload)), '\x00', '\x00', '\x00', '\x00'}
	p = append(p, Xor(payload))
}
*/
