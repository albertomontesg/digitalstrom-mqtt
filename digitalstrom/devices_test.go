package digitalstrom

import (
	"encoding/json"
	"testing"
)

var deviceHeater, _ = getJson(`  {
    "name": "hz-infrarot-ez-og",
    "dsid": "303505d7f80017c00006f47c",
    "dSUID": "303505d7f8000000000017c00006f47c00",
    "deviceType": "SW",
    "meterDSID": "303505d7f80002c0000030e6",
    "meterdSUID": "303505d7f8000000000002c0000030e600",
    "metername": "db-hz-kue-ez-wz-dsm#10",
    "zoneId": 16792,
    "outputChannels": [
      "heatingPower"
    ],
    "values": {}
  }`)

var nodSUID, _ = getJson(`  {
    "name": "hz-infrarot-ez-og",
    "dsid": "303505d7f80017c00006f47c",
    "dSUID": "",
    "deviceType": "SW",
    "meterDSID": "303505d7f80002c0000030e6",
    "meterdSUID": "303505d7f8000000000002c0000030e600",
    "metername": "db-hz-kue-ez-wz-dsm#10",
    "zoneId": 16792,
    "outputChannels": [
      "brightness"
    ],
    "values": {}
  }`)

var deviceManager = DevicesManager{}

func TestSupportedDevices(t *testing.T) {
	expectBool(t, deviceManager.supportedDevice(deviceHeater), true, "heater should be supported")
	expectBool(t, deviceManager.supportedDevice(nodSUID), false, "nodSUID should not be supported")
}

func expectBool(t *testing.T, result bool, expect bool, msg string) {
	if expect != result {
		t.Errorf("%s Expected='%t' but got '%t'", msg, expect, result)
	}
}

func getJson(str string) (map[string]interface{}, error) {
	var f map[string]interface{}
	err := json.Unmarshal([]byte(str), &f)
	return f, err
}
