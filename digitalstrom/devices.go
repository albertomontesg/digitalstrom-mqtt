package digitalstrom

import (
	"errors"
	"github.com/gaetancollaud/digitalstrom-mqtt/utils"
	"github.com/rs/zerolog/log"
	"strconv"
	"strings"
)

type DeviceType string

const (
	Light   DeviceType = "GE"
	Blind              = "GR"
	Joker              = "SW"
	Unknown            = "Unknown"
)

type DeviceStateChanged struct {
	Device   Device
	Channel  string
	NewValue float64
}

type DeviceCommand struct {
	DeviceName string
	Channel    string
	NewValue   float64
}

type Device struct {
	Name           string
	Dsid           string
	Dsuid          string
	DeviceType     DeviceType
	HwInfo         string
	MeterDsid      string
	MeterDsuid     string
	MeterName      string
	ZoneId         int
	OutputChannels []string
	Values         map[string]float64
}

type DevicesManager struct {
	httpClient      *HttpClient
	devices         []Device
	deviceStateChan chan DeviceStateChanged
}

func NewDevicesManager(httpClient *HttpClient) *DevicesManager {
	dm := new(DevicesManager)
	dm.httpClient = httpClient
	dm.deviceStateChan = make(chan DeviceStateChanged)

	return dm
}

func (dm *DevicesManager) Start() {
	dm.reloadAllDevices()
}

func (dm *DevicesManager) reloadAllDevices() {
	response, err := dm.httpClient.get("json/apartment/getDevices")
	if utils.CheckNoErrorAndPrint(err) {
		for _, s := range response.arrayValue {
			m := s.(map[string]interface{})
			if dm.supportedDevice(m) {
				dm.devices = append(dm.devices, Device{
					Dsid:           m["id"].(string),
					Dsuid:          m["dSUID"].(string),
					Name:           m["name"].(string),
					HwInfo:         m["hwInfo"].(string),
					MeterDsid:      m["meterDSID"].(string),
					MeterDsuid:     m["meterDSUID"].(string),
					MeterName:      m["meterName"].(string),
					ZoneId:         int(m["zoneID"].(float64)),
					DeviceType:     extractDeviceType(m),
					OutputChannels: extractOutputChannels(m),
					Values:         make(map[string]float64),
				})
			}
		}

		log.Debug().Str("devices", utils.PrettyPrintArray(dm.devices)).Msg("Devices loaded")
	}
}

func (dm *DevicesManager) supportedDevice(m map[string]interface{}) bool {
	if m["dSUID"] == nil || len(m["dSUID"].(string)) == 0 {
		log.Info().Str("name", m["name"].(string)).Msg("Device not supported because it has no dSUID. Enable debug to see the complete devices")
		log.Debug().Str("device", utils.PrettyPrintMap(m)).Msg("Device not supported because it has no dSUID")
		return false
	}
	return true
}

func extractDeviceType(data map[string]interface{}) DeviceType {
	hwInfo := data["hwInfo"].(string)
	if strings.HasPrefix(hwInfo, "GE") {
		return Light
	}
	if strings.HasPrefix(hwInfo, "GR") {
		return Blind
	}
	if strings.HasPrefix(hwInfo, "SW") {
		return Joker
	}
	return Unknown
}

func extractOutputChannels(data map[string]interface{}) []string {
	outputChannels := data["outputChannels"].([]interface{})

	var outputs []string

	for _, outputChannel := range outputChannels {
		chanObj := outputChannel.(map[string]interface{})
		if chanObj["channelName"] != nil {
			id := chanObj["channelName"].(string)
			outputs = append(outputs, id)
		}
	}
	return outputs
}

func (dm *DevicesManager) getTreeFloat(path string) (float64, error) {
	response, err := dm.httpClient.get("json/property/getFloating?path=" + path)
	if err == nil {
		return response.mapValue["value"].(float64), nil
	}
	return 0, err

}

func (dm *DevicesManager) updateZone(zoneId int) {
	for _, device := range dm.devices {
		if device.ZoneId == zoneId && len(device.OutputChannels) > 0 {
			dm.updateDevice(device)
		}
	}
}

func (dm *DevicesManager) updateDevice(device Device) {
	// device need to be updated
	log.Debug().Str("device", device.Name).Msg("Updating device ")
	for _, channel := range device.OutputChannels {
		newValue, err := dm.getTreeFloat("/apartment/zones/zone" + strconv.Itoa(device.ZoneId) + "/devices/" + device.Dsuid + "/status/outputs/" + channel + "/targetValue")
		if err == nil {
			dm.updateValue(device, channel, newValue)
		} else {
			log.Warn().
				Str("device", device.Name).
				Err(err).
				Msg("Unable to update device")
		}
	}
}

func (dm *DevicesManager) updateValue(device Device, channel string, newValue float64) {
	publishValue := false
	if oldVal, ok := device.Values[channel]; ok {
		//do something here
		if oldVal != newValue {
			device.Values[channel] = newValue
			log.Info().
				Str("device", device.Name).
				Str("channel", channel).
				Float64("oldValue", oldVal).
				Float64("newValue", newValue).
				Msg("Value changed")
			publishValue = true
		}
	} else {
		// new value
		device.Values[channel] = newValue
		log.Info().
			Str("device", device.Name).
			Str("channel", channel).
			Float64("newValue", newValue).
			Msg("New value")
		publishValue = true
	}
	if publishValue {
		dm.deviceStateChan <- DeviceStateChanged{
			Device:   device,
			Channel:  channel,
			NewValue: newValue,
		}
	}
}

func (dm *DevicesManager) SetValue(command DeviceCommand) error {
	deviceFound := false
	channelFound := false
	for _, device := range dm.devices {
		if device.Name == command.DeviceName && len(device.OutputChannels) > 0 {
			deviceFound = true
			for _, c := range device.OutputChannels {
				if c == command.Channel {
					channelFound = true

					log.Info().
						Str("device", command.DeviceName).
						Str("channel", command.Channel).
						Float64("value", command.NewValue).
						Msg("Setting value ")
					strValue := strconv.Itoa(int(command.NewValue))
					_, err := dm.httpClient.get("json/device/setOutputChannelValue?dsid=" + device.Dsid + "&channelvalues=" + c + "=" + strValue + "&applyNow=1")
					if utils.CheckNoErrorAndPrint(err) {
						dm.updateValue(device, command.Channel, command.NewValue)
					}
				}
			}
		}
	}
	if !deviceFound {
		return errors.New("No device '" + command.DeviceName + "' found")
	}
	if !channelFound {
		return errors.New("No channel '" + command.Channel + "' found on device '" + command.DeviceName + "'")
	}
	return nil
}
