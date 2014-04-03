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
	receiveBuf []byte
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
	retry := 0
	l := 0
	var data []byte
	buf := make([]byte, size)
	for {
		if len(f.receiveBuf) < minlen && retry < 3 {
			n, err := f.reader.Read(buf)
			log.Printf("Read: [% #x]\n", buf[:n])
			f.receiveBuf = append(f.receiveBuf, buf[:n]...)
			if err != nil {
				retry ++
			}
			continue
		}
		if len(f.receiveBuf) < minlen {
			return []byte{}, fmt.Errorf("read ant data failed")
		}
		data = f.receiveBuf
		data = f.FindSync(data)
		if ok, err := ANTPackageSum(data); !ok {
			if err.Error() == "length error" {
				retry = 0
				continue
			}
			f.receiveBuf = f.FindSync(data[1:])
			continue
		}
		l = int(data[1]) + 4
		f.receiveBuf = data[l:]
		break
	}
	log.Printf("Get ANT packet: [% #x], Size: %d\nbuf:[% #x]\n", data[:l], l, f.receiveBuf)
	return data[:l], nil
}

func (f *ANT) FindSync(data []byte) []byte {
	index := 0
	for i, v := range data {
		if v == '\xa4' || v == '\xa5' {
			index = i
			break
		}
	}
	if index > 0 {
		log.Printf("discarding:[% #x]\n", data[:index])
	}
	return data[index:]
}

// CheckSum

func ANTPackageSum(data []byte) (bool, error) {
	if len(data) > 4 {
		if data[1] < 0 || data[1] > 32 {
			return false, fmt.Errorf("wrong size")
		}
		l := int(data[1]) + 4
		if len(data) < l {
			return false, fmt.Errorf("length error")
		}
		p := data[0:l]
		if XorSum(p) != '\x00' {
			return false, fmt.Errorf("xor error")
		}
		return true, nil
	}
	return false, fmt.Errorf("length error")
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
	for i:= 0; i< 8; i++ {
		if ok, err := f.CheckResetResponse('\x20'); ok {
			return ok, err
		}
	}
	return false, fmt.Errorf("bad reset")
}
func (f *ANT) SetChannelFrequency(freq byte) (bool, error) {
	err := f.SendMessage('\x45', f.channel, freq)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetTransmitPower(power byte) (bool, error) {
	err := f.SendMessage('\x47', '\x00', power)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetSearchTimeout(timeout byte) (bool, error) {
	err := f.SendMessage('\x44', f.channel, timeout)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetChannelPeriod(period []byte) (bool, error) {
	err := f.SendMessage('\x43', f.channel, period)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetNetworkKey(network byte, key []byte) (bool, error) {
	err := f.SendMessage('\x46', network, key)
	if err != nil {
		return false, err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetChannelId(id []byte) (bool, error) {
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

func (f *ANT) ReceiveAcknowledgedReply() ([]byte, error) {
	var data []byte
	var err error
	for i := 0; i < 30; i++ {
		data, err = f.ReceiveMessage(13)
		if len(data) > 4 && data[2] == '\x4f' {
			log.Println("Get Ack: ", data[4:len(data)-1])
			return data[4:len(data)-1], err
		}
	}
	return data, fmt.Errorf("failed to receive acknowledged reply")
}

func (f *ANT) CheckTxResponse(maxtries int) (bool, error) {
	for i := 0; i < maxtries; i++ {
		status, err := f.ReceiveMessage(4096)
		if err != nil {
			continue
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
	log.Println("no tx")
	return false, fmt.Errorf("No Transmission Ack Seen")
}

func (f *ANT) SendBurstData(data []byte, sleep time.Duration) (bool, error) {
	for i := 0; i < 2; i++ {
		l := 0
		for {
			if (l+9) > len(data) {
				f.SendMessage('\x50', data[l:])
				break
			} else {
				f.SendMessage('\x50', data[l:l+9])
			}
			if sleep > 0 {
				time.Sleep(sleep)
			}
			l += 9
		}
		if ok, err := f.CheckTxResponse(16); ok {
			log.Println("tx ok", ok, err)
			return ok, err
		}
	}

	return false, fmt.Errorf("write burst data failed")
}

func (f *ANT) CheckBurstResponse() ([]byte, error) {
	var response []byte
	log.Println("CheckBurstResponse")
	defer log.Println("CheckBurstResponse End")
	for i := 0; i < 128; i++ {
		status, _ := f.ReceiveMessage(4096)
		if len(status) > 5 && status[2] == '\x40' && status[5] == '\x04' {
			return response, fmt.Errorf("Burst receive failed by event!")
		}
		if len(status) > 4 && status[2] == '\x4f' {
			response = append(response, status[4:len(status)-1]...)
			return response, nil
		}
		if len(status) > 4 && status[2] == '\x50' {
			response = append(response, status[4:len(status)-1]...)
			if (status[3]&'\x80') > 0 {
				return response, nil
			}
		}
	}
	return response, fmt.Errorf("Burst receive failed to detect end")
}

func (f *ANT) SendAcknowledgedData(l []byte) (bool, error) {
	for i := 0; i < 8; i++ {
		err := f.SendMessage('\x4f', f.channel, l)
		if err != nil {
			continue
		}
		if ok, err := f.CheckTxResponse(16); ok {
			return ok, err
		}
	}
	return false, fmt.Errorf("Failed to send Acknowledged Data")
}
