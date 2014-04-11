package main

import (
	"fmt"
	"github.com/kylelemons/gousb/usb"
	"log"
	"reflect"
	"time"
)

type ANT struct {
	channel    byte
	receiveBuf []byte
	reader     usb.Endpoint
	writer     usb.Endpoint
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
			msgLen++
			payload = append(payload, d)
		case int32:
			msgLen++
			payload = append(payload, byte(d))
		case []byte:
			msgLen += len(d)
			payload = append(payload, d...)
		default:
			log.Println("not support", reflect.TypeOf(v), v)
		}
	}
	b = append(b, byte(msgLen-1)) //data[0] is message type
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
	data := f.receiveBuf
	buf := make([]byte, size)
	for {
		if len(data) < minlen && retry < 3 {
			n, err := f.reader.Read(buf)
			data = append(data, buf[:n]...)
			if err != nil {
				log.Println(err)
				retry++
			}
			continue
		}
		if len(data) < minlen {
			return []byte{}, fmt.Errorf("read ant data failed")
		}
		data = f.FindSync(data)
		err := ANTPackageSum(data)
		if err != nil && err.Error() == "length error" {
			retry = 0
			n, err := f.reader.Read(buf)
			if err != nil {
				log.Println(err)
				retry++
			}
			data = append(data, buf[:n]...)
			continue
		}
		if err != nil {
			data = f.FindSync(data[1:])
			continue
		}
		l = int(data[1]) + 4
		f.receiveBuf = data[l:]
		break
	}
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

func ANTPackageSum(data []byte) error {
	if len(data) > 4 {
		if data[1] < 0 || data[1] > 32 {
			return fmt.Errorf("wrong size")
		}
		l := int(data[1]) + 4
		if len(data) < l {
			return fmt.Errorf("length error")
		}
		p := data[0:l]
		if XorSum(p) != '\x00' {
			return fmt.Errorf("xor error")
		}
		return nil
	}
	return fmt.Errorf("length error")
}

// FitBit ANT protocal
func (f *ANT) CheckResetResponse(status byte) error {
	data, err := f.ReceiveMessage(4096)
	if err != nil {
		return err
	}
	if len(data) > 3 && data[2] == '\x6f' && data[3] == status {
		return nil
	}
	return fmt.Errorf("rest failed: % #x", status)
}
func (f *ANT) CheckOkResponse() error {
	data, err := f.ReceiveMessage(4096)
	if err != nil {
		return err
	}
	if len(data) < 6 {
		return fmt.Errorf("lack response data: %x", data)
	}
	if data[2] == '\x40' && data[5] == '\x00' {
		return nil
	}
	return fmt.Errorf("bad response data: %x", data)
}

func (f *ANT) Reset() error {
	err := f.SendMessage('\x4a', '\x00')
	if err == nil {
		time.Sleep(time.Second)
		for i := 0; i < 8; i++ {
			if err = f.CheckResetResponse('\x20'); err == nil {
				break
			}
		}
	}
	return err
}
func (f *ANT) SetChannelFrequency(freq byte) error {
	err := f.SendMessage('\x45', f.channel, freq)
	if err != nil {
		return err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetTransmitPower(power byte) error {
	err := f.SendMessage('\x47', '\x00', power)
	if err != nil {
		return err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetSearchTimeout(timeout byte) error {
	err := f.SendMessage('\x44', f.channel, timeout)
	if err != nil {
		return err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetChannelPeriod(period []byte) error {
	err := f.SendMessage('\x43', f.channel, period)
	if err != nil {
		return err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetNetworkKey(network byte, key []byte) error {
	err := f.SendMessage('\x46', network, key)
	if err != nil {
		return err
	}
	return f.CheckOkResponse()
}

func (f *ANT) SetChannelId(id []byte) error {
	err := f.SendMessage('\x51', f.channel, id)
	if err != nil {
		return err
	}
	return f.CheckOkResponse()
}

func (f *ANT) OpenChannel() error {
	err := f.SendMessage('\x4b', f.channel)
	if err != nil {
		return err
	}
	return f.CheckOkResponse()
}
func (f *ANT) CloseChannel() error {
	err := f.SendMessage('\x4c', f.channel)
	if err != nil {
		return err
	}
	return f.CheckOkResponse()
}
func (f *ANT) AssignChannel() error {
	err := f.SendMessage('\x42', f.channel, '\x00', '\x00')
	if err != nil {
		return err
	}
	return f.CheckOkResponse()
}

func (f *ANT) ReceiveAcknowledgedReply() ([]byte, error) {
	var data []byte
	var err error
	for i := 0; i < 30; i++ {
		data, err = f.ReceiveMessage(13)
		if err != nil {
			log.Println(err)
			continue
		}
		if len(data) > 4 && data[2] == '\x4f' {
			l := len(data)
			return data[4 : l-1], err
		}
	}
	return data, fmt.Errorf("failed to receive acknowledged reply")
}

func (f *ANT) CheckTxResponse(maxtries int) error {
	var status []byte
	var err error
	for i := 0; i < maxtries; i++ {
		status, err = f.ReceiveMessage(4096)
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
				return nil
			}
			if status[5] == '\x06' {
				return fmt.Errorf("Transmission Failed: %x", status)
			}
		}
	}
	return err
}

func (f *ANT) SendBurstData(data []byte, sleep time.Duration) error {
	var err error
	for i := 0; i < 4; i++ {
		for l := 0; l < len(data); l += 9 {
			if (l + 9) > len(data) {
				err = f.SendMessage('\x50', data[l:])
			} else {
				err = f.SendMessage('\x50', data[l:l+9])
			}
			if err != nil {
				break
			}
			if sleep > 0 {
				time.Sleep(sleep)
			}
		}
		if err != nil {
			log.Println("failed to send: ", err)
			continue
		}
		if err = f.CheckTxResponse(16); err == nil {
			return nil
		}
	}
	return err
}

func (f *ANT) CheckBurstResponse() ([]byte, error) {
	var response []byte
	for i := 0; i < 128; i++ {
		status, err := f.ReceiveMessage(4096)
		if err != nil {
			continue
		}
		l := len(status)
		if l > 5 && status[2] == '\x40' && status[5] == '\x04' {
			return response, fmt.Errorf("Burst receive failed by event!")
		} else {
			if l > 4 && status[2] == '\x4f' {
				response = append(response, status[4:l-1]...)
				return response, nil
			} else {
				if l > 4 && status[2] == '\x50' {
					response = append(response, status[4:l-1]...)
					if (status[3] & '\x80') != 0 {
						return response, nil
					}
				}
			}
		}
	}
	return response, fmt.Errorf("Burst receive failed to detect end")
}

func (f *ANT) SendAcknowledgedData(l []byte) error {
	for i := 0; i < 8; i++ {
		err := f.SendMessage('\x4f', f.channel, l)
		if err != nil {
			continue
		}
		if err = f.CheckTxResponse(16); err == nil {
			return err
		}
	}
	return fmt.Errorf("Failed to send Acknowledged Data")
}
