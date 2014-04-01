package main

import (
	"fmt"
	"github.com/kylelemons/gousb/usb"
	"log"
	"reflect"
	"time"
)

type ANT struct {
	channel byte
	reader  usb.Endpoint
	writer  usb.Endpoint
}

func XorSum(data []byte) byte {
	var b byte
	for _, v := range data {
		b = b ^ v
	}
	return b
}

func (f *ANT) SendMessage(data ...interface{}) error {
	var b []byte
	//0xa4
	b = append(b, '\xa4')
	msgLen := 0
	var payload []byte
	for _, v := range data {
		switch d := v.(type) {
		case byte:
			msgLen += 1
			payload = append(payload, d)
		case int32:
			msgLen += 1
			payload = append(payload, byte(d))
		case []byte:
			msgLen += len(d)
			payload = append(payload, d...)
		default:
			log.Println("not support", reflect.TypeOf(v), v)
		}
	}
	b = append(b, byte(msgLen)-1) //data[0] is message type
	b = append(b, payload...)
	b = append(b, XorSum(b))
	size, err := f.writer.Write(b)
	log.Printf("Send: [% #x], Size: %d\n", b, size)
	return err
}
func (f *ANT) ReceiveMessage(size int) ([]byte, error) {
	minlen := 4
	data := make([]byte, size)
	l := 0
	retry := 0
	for {
		n, err := f.reader.Read(data[l:])
		l += n
		if err != nil {
			log.Println(err)
			if retry < 3 {
				retry++
				continue
			}
		}
		log.Printf("receive: [% #x], size: %d\n", data[:l], l)
		if l < minlen {
			continue
		}
		data = f.FindSync(data, 0)
		if data[1] < 0 || data[1] > 32 {
			data = f.FindSync(data, 1)
			continue
		}
		l = int(data[1]) + 4
		if len(data) < l {
			continue
		}
		p := data[0:l]
		if XorSum(p) != '\x00' {
			data = f.FindSync(data, 1)
			continue
		}
		return p, nil
	}
	return data, fmt.Errorf("fail to read message")
}

func (f *ANT) FindSync(data []byte, start int) []byte {
	index := 0
	for i, v := range data {
		if i >= start && (v == '\xa4' || v == '\xa5') {
			index = i
			break
		}
		i += 1
	}
	if index > 0 {
		log.Println("discarding ", data[:index])
	}
	return data[index:]
}

// FitBit ANT protocal
func (f *ANT) CheckResetResponse(status byte) (bool, error) {
	data, err := f.ReceiveMessage(4096)
	if err != nil {
		return false, err
	}
	if len(data) > 3 && data[2] == '\x6f' && data[3] == status {
		return true, nil
	}
	return false, fmt.Errorf("rest failed: % #x", status)
}
func (f *ANT) CheckOkResponse() (bool, error) {
	data, err := f.ReceiveMessage(4096)
	if err != nil {
		return false, err
	}
	if len(data) < 6 {
		return false, fmt.Errorf("lack response data: %x", data)
	}
	if data[2] == '\x40' && data[5] == '\x00' {
		return true, nil
	}
	return false, fmt.Errorf("bad response data: %x", data)
}

func (f *ANT) Reset() (bool, error) {
	err := f.SendMessage('\x4a', '\x00')
	if err != nil {
		log.Println("write err")
		return false, err
	}
	time.Sleep(time.Second)
	return f.CheckResetResponse('\x20')
}
func (f *ANT) SetChannelFrequency(freq ...interface{}) (bool, error) {
	err := f.SendMessage('\x45', f.channel, freq)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetTransmitPower(power ...interface{}) (bool, error) {
	err := f.SendMessage('\x47', '\x00', power)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetSearchTimeout(timeout ...interface{}) (bool, error) {
	err := f.SendMessage('\x44', f.channel, timeout)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetChannelPeriod(period ...interface{}) (bool, error) {
	err := f.SendMessage('\x43', f.channel, period)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetNetworkKey(network byte, key ...interface{}) (bool, error) {
	err := f.SendMessage('\x46', network, key)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetChannelId(id ...interface{}) (bool, error) {
	err := f.SendMessage('\x51', f.channel, id)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) OpenChannel() (bool, error) {
	err := f.SendMessage('\x4b', f.channel)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}
func (f *ANT) CloseChannel() (bool, error) {
	err := f.SendMessage('\x4c', f.channel)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}
func (f *ANT) AssignChannel() (bool, error) {
	err := f.SendMessage('\x42', f.channel, '\x00', '\x00')
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) ReceiveAcknowledgedReply(size int) ([]byte, error) {
	var data []byte
	var err error
	for i := 0; i < 30; i++ {
		data, err = f.ReceiveMessage(size)
		if len(data) > 4 && data[2] == '\x4f' {
			return data[4:len(data)], err
		}
	}
	return data, fmt.Errorf("Failed to receive acknowledged reply: %s", data)
}

func (f *ANT) CheckTxResponse(maxtries int) (bool, error) {
	for i := 0; i < maxtries; i++ {
		status, err := f.ReceiveMessage(4096)
		if err != nil {
			return false, err
		}
		if len(status) > 5 && status[2] == '\x40' {
			if status[5] == '\x0a' {
				// TX Start
				continue
			}
			if status[5] == '\x05' {
				// TX successful
				return true, nil
			}
			if status[5] == '\x06' {
				return false, fmt.Errorf("Transmission Failed: %x", status)
			}
		}
	}
	return false, fmt.Errorf("No Transmission Ack Seen")
}

func (f *ANT) SendBurstData(data []byte) (bool, error) {
	for i := 0; i < 2; i++ {
		err := f.SendMessage('\x72', data)
		if err != nil {
			continue
		}
		if ok, err := f.CheckTxResponse(10); ok {
			return ok, err
		}
	}
	return false, fmt.Errorf("write burst data failed")
}

func (f *ANT) CheckBurstResponse() ([]byte, error) {
	var response []byte
	for i := 0; i < 128; i++ {
		status, err := f.ReceiveMessage(4096)
		if err != nil {
			continue
		}
		if len(status) > 5 && status[2] == '\x40' && status[5] == '\x40' {
			return response, fmt.Errorf("Burst receive failed by event!")
		}
		if len(status) > 4 && status[2] == '\x4f' {
			response = append(response, status[4:]...)
			return response, nil
		}
		if len(status) > 4 && status[2] == '\x50' {
			response = append(response, status[4:]...)
			if status[3]&'\x80' != '\x00' {
				return response, nil
			}
		}
	}
	return response, fmt.Errorf("Burst receive failed to detect end")
}

func (f *ANT) SendAcknowledgedData(l ...interface{}) (bool, error) {
	for i := 0; i < 8; i++ {
		err := f.SendMessage('\x4f', f.channel, l)
		if err != nil {
			continue
		}
		if ok, err := f.CheckTxResponse(10); ok {
			return ok, err
		}
	}
	return false, fmt.Errorf("Failed to send Acknowledged Data")
}
