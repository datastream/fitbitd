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
func (f *FitbitBase) InitDeviceChannel(channel []byte) error {
	err := f.base.Reset()
	if err != nil {
		log.Println("fitbit reset failed")
		return err
	}
	err = f.base.SetNetworkKey(0, []byte{0, 0, 0, 0, 0, 0, 0, 0})
	if err != nil {
		log.Println("fitbit set network key failed")
		return err
	}
	err = f.base.AssignChannel()
	if err != nil {
		log.Println("fitbit assign channel failed")
		return err
	}
	err = f.base.SetChannelPeriod([]byte{'\x00', '\x10'})
	if err != nil {
		log.Println("fitbit set channel period failed")
		return err
	}
	err = f.base.SetChannelFrequency('\x02')
	if err != nil {
		log.Println("fitbit set channel frequency failed")
		return err
	}
	err = f.base.SetTransmitPower('\x03')
	if err != nil {
		log.Println("fitbit set transmit power failed")
		return err
	}
	err = f.base.SetSearchTimeout('\xff')
	if err != nil {
		log.Println("fitbit set search timeout failed")
		return err
	}
	err = f.base.SetChannelId(channel)
	if err != nil {
		log.Println("fitbit set channel id failed")
		return err
	}
	err = f.base.OpenChannel()
	if err != nil {
		log.Println("fitbit open channel failed")
		return err
	}
	return err
}

func (f *FitbitBase) InitTrackerForTransfer() error {
	err := f.InitDeviceChannel([]byte{'\xff', '\xff', '\x01', '\x01'})
	if err != nil {
		log.Println("fitbit init failed")
		return err
	}
	err = f.WaitForBeacon()
	if err != nil {
		return err
	}
	err = f.ResetTracker()
	if err != nil {
		log.Println("fitbit reset tracker failed")
		return err
	}
	cid := []byte{byte(rand.Intn(254)), byte(rand.Intn(254))}
	err = f.base.SendAcknowledgedData(append(append([]byte{'\x78', '\x02'}, cid...), []byte{'\x00', '\x00', '\x00', '\x00'}...))
	if err != nil {
		log.Println("fitbit ack ack to tracker failed")
		return err
	}
	err = f.base.CloseChannel()
	if err != nil {
		log.Println("fitbit close channel failed")
		return err
	}
	err = f.InitDeviceChannel(append(cid, []byte{0x01, 0x01}...))
	if err != nil {
		log.Println("fitbit reinit channel failed")
		return err
	}
	err = f.WaitForBeacon()
	if err != nil {
		return err
	}
	err = f.PingTracker()
	if err != nil {
		log.Println("fitbit ping tracker failed")
	}
	return err
}

func (f *FitbitBase) ResetTracker() error {
	return f.base.SendAcknowledgedData([]byte{'\x78', '\x01', '\x00', '\x00', '\x00', '\x00', '\x00', '\x00'})
}

func (f *FitbitBase) CommandSleep() error {
	return f.base.SendAcknowledgedData([]byte{'\x7f', '\x03', '\x00', '\x00', '\x00', '\x00', '\x00', '\x3c'})
}

func (f *FitbitBase) PingTracker() error {
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
		err := f.SendTrackerPacket(opcode)
		if err != nil {
			log.Println(err)
			continue
		}
		data, err := f.base.ReceiveAcknowledgedReply()
		if err != nil {
			log.Println(err)
			continue
		}
		if data[0] != byte(f.currentPacketId) {
			log.Printf("Tracker Packet IDs don't match! %v %v \n", f.currentPacketId, data[0])
			continue
		}
		if data[1] == '\x42' {
			log.Println("Start DataBank")
			return f.GetDataBank()
		}
		if data[1] == '\x61' {
			if len(payload) > 0 {
				if err := f.SendTrackerPayload(payload); err != nil {
					log.Println("payload failed", err)
					break
				}
				data, err := f.base.ReceiveAcknowledgedReply()
				log.Println("payload", err)
				if len(data) > 0 {
					return data[1:], err
				}
				return data, err
			}
		}
		if data[1] == '\x41' {
			return data[1:], nil
		}
	}
	return []byte{}, fmt.Errorf("failed to run opcode")
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
func (f *FitbitBase) SendTrackerPayload(payload []byte) error {
	p := []byte{'\x00', byte(f.GenPacketId()), '\x80', byte(len(payload)), '\x00', '\x00', '\x00', '\x00'}
	p = append(p, XorSum(payload))
	prefix := []byte{'\x20', '\x40', '\x60'}
	index := 0
	for i := 0; i < len(payload); i += 8 {
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
			continue
		}
		f.currentBankId++
		cmd = '\x60' // Send 0x60 on subsequent bursts
		if len(bank) == 0 {
			return data, nil
		}
		data = append(data, bank...)
	}
	return data, fmt.Errorf("Cannot complete data bank")
}

func (f *FitbitBase) CheckTrackerDataBank(index int, cmd byte) ([]byte, error) {
	err := f.SendTrackerPacket([]byte{cmd, '\x00', '\x02', byte(index), '\x00', '\x00', '\x00'})
	var data []byte
	if err == nil {
		data, err = f.GetTrackerBurst()
	}
	return data, err
}
func (f *FitbitBase) SendTrackerPacket(packet []byte) error {
	p := append([]byte{byte(f.GenPacketId())}, packet...)
	log.Println("SendAcknowledgedData:", p)
	return f.base.SendAcknowledgedData(p)
}
func (f *FitbitBase) GetTrackerBurst() ([]byte, error) {
	var data []byte
	d, err := f.base.CheckBurstResponse()
	if err != nil || len(d) < 1 {
		return data, err
	}
	if d[1] != '\x81' {
		return d, fmt.Errorf("Response received is not tracker burst! Got")
	}
	size := (int(d[3]) << 8) | int(d[2])
	if size == 0 {
		return data, err
	}
	datalen := 8 + size
	if datalen < len(d) {
		data = d[8:datalen]
	} else {
		data = d[8:]
	}
	return data, err
}

func (f *FitbitBase) RunDataBankOpcode(index byte) ([]byte, error) {
	return f.RunOpcode([]byte{'\x22', index, '\x00', '\x00', '\x00', '\x00', '\x00'}, []byte{})
}

func (f *FitbitBase) GetTrackerInfo() ([]byte, error) {
	return f.RunOpcode([]byte{'\x24', '\x00', '\x00', '\x00', '\x00', '\x00', '\x00'}, []byte{})
}
