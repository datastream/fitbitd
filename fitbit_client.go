package main

const (
	CLIENT_UUID = "2ea32002-a079-48f4-8020-0badd22939e3"
	FITBIT_HOST = "https://client.fitbit.com"
	START_PATH  = "/device/tracker/uploadData"
)

type FitbitClient struct {
	*FitbitBase
	info map[string]string
}

func (c *FitbitClient) Form() {
	c.info = make(map[string]string)
	c.info["beaconType"] = "standard"
	c.info["clientMode"] = "standard"
	c.info["clientVersion"] = "1.0"
	c.info["os"] = "libfitbit"
	c.info["clientId"] = CLIENT_UUID
}

func (c *FitbitClient) Upload() {
	//init_tracker_for_transfer
}
