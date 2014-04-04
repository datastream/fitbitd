package main

import (
	"fmt"
	"github.com/kylelemons/gousb/usb"
	"log"
	"math/rand"
	"time"
)

type FitbitBase struct {
	ctx                *usb.Context
	dev                *usb.Device
	base               *ANT
	trackerPacketCount int
	currentPacketId    int
	currentBankId      int
}

func (f *FitbitBase) Open() error {
	f.ctx = usb.NewContext()
	devs, err := f.ctx.ListDevices(func(desc *usb.Descriptor) bool {
		if desc.Vendor.String() == "10c4" && desc.Product.String() == "84c4" {

			return true
		}
		return false
	})
	if err == nil {
		if len(devs) == 0 {
			err = fmt.Errorf("no devices found")
		} else {
			f.dev = devs[0]
		}
		for i := 1; i < len(devs); i++ {
			devs[i].Close()
		}
	}
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
func (f *FitbitBase) InitDeviceChannel(channel []byte) (bool, error) {
	ok, err := f.base.Reset()
	if !ok {
		log.Println("fitbit reset failed")
		return ok, err
	}
	ok, err = f.base.SetNetworkKey(0, []byte{0, 0, 0, 0, 0, 0, 0, 0})
	if !ok {
		log.Println("fitbit set network key failed")
		return ok, err
	}
	ok, err = f.base.AssignChannel()
	if !ok {
		log.Println("fitbit assign channel failed")
		return ok, err
	}
	ok, err = f.base.SetChannelPeriod([]byte{'\x00', '\x10'})
	if !ok {
		log.Println("fitbit set channel period failed")
		return ok, err
	}
	ok, err = f.base.SetChannelFrequency('\x02')
	if !ok {
		log.Println("fitbit set channel frequency failed")
		return ok, err
	}
	ok, err = f.base.SetTransmitPower('\x03')
	if !ok {
		log.Println("fitbit set transmit power failed")
		return ok, err
	}
	ok, err = f.base.SetSearchTimeout('\xff')
	if !ok {
		log.Println("fitbit set search timeout failed")
		return ok, err
	}
	ok, err = f.base.SetChannelId(channel)
	if !ok {
		log.Println("fitbit set channel id failed")
		return ok, err
	}
	ok, err = f.base.OpenChannel()
	if !ok {
		log.Println("fitbit open channel failed")
		return ok, err
	}
	return ok, err
}

func (f *FitbitBase) InitTrackerForTransfer() (bool, error) {
	ok, err := f.InitDeviceChannel([]byte{'\xff', '\xff', '\x01', '\x01'})
	if !ok {
		log.Println("fitbit init failed")
		return ok, err
	}
	err = f.WaitForBeacon()
	if err != nil {
		return false, err
	}
	ok, err = f.ResetTracker()
	if !ok {
		log.Println("fitbit reset tracker failed")
		return ok, err
	}
	cid := []byte{byte(rand.Intn(254)), byte(rand.Intn(254))}
	ok, err = f.base.SendAcknowledgedData(append(append([]byte{'\x78', '\x02'}, cid...), []byte{'\x00', '\x00', '\x00', '\x00'}...))
	if !ok {
		log.Println("fitbit ack ack to tracker failed")
		return ok, err
	}
	ok, err = f.base.CloseChannel()
	if !ok {
		log.Println("fitbit close channel failed")
		return ok, err
	}
	ok, err = f.InitDeviceChannel(append(cid, []byte{0x01, 0x01}...))
	if !ok {
		log.Println("fitbit reinit channel failed")
		return ok, err
	}
	err = f.WaitForBeacon()
	if err != nil {
		return false, err
	}
	ok, err = f.PingTracker()
	if !ok {
		log.Println("fitbit ping tracker failed")
	}
	return ok, err
}

func (f *FitbitBase) ResetTracker() (bool, error) {
	return f.base.SendAcknowledgedData([]byte{'\x78', '\x01', '\x00', '\x00', '\x00', '\x00', '\x00', '\x00'})
}

func (f *FitbitBase) CommandSleep() (bool, error) {
	return f.base.SendAcknowledgedData([]byte{'\x7f', '\x03', '\x00', '\x00', '\x00', '\x00', '\x00', '\x3c'})
}

func (f *FitbitBase) PingTracker() (bool, error) {
	return f.base.SendAcknowledgedData([]byte{'\x78', '\x00', '\x00', '\x00', '\x00', '\x00', '\x00', '\x00'})
}

func (f *FitbitBase) WaitForBeacon() error {
	for i := 0; i < 75; i++ {
		data, err := f.base.ReceiveMessage(4096)
		if err == nil && len(data) > 2 && data[2] == '\x4e' {
			return nil
		}
		log.Println("retry beacon", err)
	}
	return fmt.Errorf("Failed to see tracker beacon")
}

func (f *FitbitBase) RunOpcode(opcode, payload []byte) ([]byte, error) {
	for i := 0; i < 4; i++ {
		ok, err := f.SendTrackerPacket(opcode)
		if !ok {
			continue
		}
		data, err := f.base.ReceiveAcknowledgedReply()
		if err != nil {
			continue
		}
		if data[0] != byte(f.currentPacketId) {
			log.Printf("Tracker Packet IDs don't match! %v %v \n", f.currentPacketId, data[0])
		}
		if data[1] == '\x42' {
			return f.GetDataBank()
		}
		if data[1] == '\x61' {
			if len(payload) > 0 {
				f.SendTrackerPayload(payload)
				data, err := f.base.ReceiveAcknowledgedReply()
				data = data[1:]
				return data, err
			}
		}
		if data[1] == '\x41' {
			data = data[1:]
			return data, nil
		}
	}
	return []byte{}, fmt.Errorf("failed to run opcode")
}
func (f *FitbitBase) SendTrackerPacket(packet []byte) (bool, error) {
	p := append([]byte{byte(f.GenPacketId())}, packet...)
	return f.base.SendAcknowledgedData(p)
}

func (f *FitbitBase) GenPacketId() int {
	f.currentPacketId = '\x38' + f.getTrackerPacketCount()
	return f.currentPacketId
}

func (f *FitbitBase) getTrackerPacketCount() int {
	f.trackerPacketCount++
	if f.trackerPacketCount > 7 {
		f.trackerPacketCount = 0
	}
	return f.trackerPacketCount
}
func (f *FitbitBase) SendTrackerPayload(payload []byte) (bool, error) {
	p := []byte{'\x00', byte(f.GenPacketId()), '\x80', byte(len(payload)), '\x00', '\x00', '\x00', '\x00'}
	p = append(p, XorSum(payload))
	prefix := []byte{'\x20', '\x40', '\x60'}
	i := 0
	index := 0
	for {
		current_prefix := prefix[index%3]
		var plist []byte
		if (i + 8) > len(payload) {
			plist = append(plist, byte((int(current_prefix)+'\x80'))|f.base.channel)
			plist = append(plist, payload[i:]...)
		} else {
			plist = append(plist, current_prefix|f.base.channel)
			plist = append(plist, payload[i:i+8]...)
		}
		for {
			if len(plist) >= 9 {
				break
			}
			plist = append(plist, '\x00')
		}
		p = append(p, plist...)
		i += 8
		if i > len(payload) {
			break
		}
		index++
	}
	return f.base.SendBurstData(p, 10*time.Millisecond)
}
func (f *FitbitBase) GetDataBank() ([]byte, error) {
	var data []byte
	cmd := byte('\x70')
	for i := 0; i < 2000; i++ {
		bank, err := f.CheckTrackerDataBank(f.currentBankId, cmd)
		if err != nil {
			log.Println(err)
		}
		f.currentBankId += 1
		cmd = '\x60' // Send 0x60 on subsequent bursts
		if len(bank) == 0 {
			log.Println("Get dataBank", data)
			return data, nil
		}
		data = append(data, bank...)
	}
	return data, fmt.Errorf("Cannot complete data bank")
}

func (f *FitbitBase) CheckTrackerDataBank(index int, cmd byte) ([]byte, error) {
	f.SendTrackerPacket([]byte{cmd, '\x00', '\x02', byte(index), '\x00', '\x00', '\x00'})
	return f.GetTrackerBurst()
}
func (f *FitbitBase) GetTrackerBurst() ([]byte, error) {
	d, err := f.base.CheckBurstResponse()
	if len(d) > 0 && d[1] != '\x81' {
		return d, fmt.Errorf("Response received is not tracker burst! Got")
	}
	size := d[3]<<8 | d[2]
	log.Println("burst response:", d, size)
	if size == 0 {
		return d[:0], err
	}
	log.Println("get burst:", d[8:8+size])
	return d[8 : 8+size], err
}

func (f *FitbitBase) RunDataBankOpcode(index byte) ([]byte, error) {
	return f.RunOpcode([]byte{'\x22', index, '\x00', '\x00', '\x00', '\x00', '\x00'}, []byte{})
}

func (f *FitbitBase) GetTrackerInfo() ([]byte, error) {
	return f.RunOpcode([]byte{'\x24', '\x00', '\x00', '\x00', '\x00', '\x00', '\x00'}, []byte{})
}
