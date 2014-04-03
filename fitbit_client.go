package main

import (
	"net/http"
	"net/url"
	"io/ioutil"
	"fmt"
	"encoding/base64"
	"encoding/xml"
	"log"
)

const (
	CLIENTUUID = "2ea32002-a079-48f4-8020-0badd22939e3"
	FITBITHOST = "https://client.fitbit.com"
	STARTPATH  = "/device/tracker/uploadData"
)

type FitbitClient struct {
	*FitbitBase `xml:""`
	ResponseInfo Response `xml:"response"`
	RemoteOps []RemoteOp `xml:"device>remoteOps>remoteOp"`
}

type Response struct {
	Host string `xml:"host,attr"`
	Path string `xml:"path,attr"`
}

type RemoteOp struct {
	OpCode string `xml:"opCode"`
	PayloadData string `xml:"payloadData"`
}

func (c *FitbitClient) GetRemoteInfo() error {
	//init_tracker_for_transfer
	v := url.Values{}
	weburl := FITBITHOST + STARTPATH
	ok, err := c.InitTrackerForTransfer()
	log.Println("end init----------")
	if !ok {
		return err
	}
	defer c.CommandSleep()
	client := http.Client{}
	for {
		v.Set("beaconType", "standard")
		v.Set("clientMode", "standard")
		v.Set("clientVersion", "1.0")
		v.Set("os", "libfitbit")
		v.Set("clientId", CLIENTUUID)
		log.Println(weburl, v)
		resp, err := client.PostForm(weburl, v)
		c.ResponseInfo.Host = ""
		c.ResponseInfo.Path = ""
		if err != nil {
			log.Println(err)
			break
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			err = xml.Unmarshal(body, c)
		}
		resp.Body.Close()
		log.Println(string(body))
		for i, op := range c.RemoteOps {
			opcode, err := base64.StdEncoding.DecodeString(op.OpCode)
			payload, err := base64.StdEncoding.DecodeString(op.PayloadData)
			code, err := c.RunOpcode(opcode, payload)
			if err != nil {
				log.Println(err)
			}
			v.Set(fmt.Sprintf("opResponse[%d]", i), base64.StdEncoding.EncodeToString(code))
			v.Set(fmt.Sprintf("opStatus[%d]", i), "success")
		}
		c.RemoteOps = c.RemoteOps[:0]
		if c.ResponseInfo.Host == "" {
			break
		}
		weburl = "http://" + c.ResponseInfo.Host + c.ResponseInfo.Path
	}
	return err
}
