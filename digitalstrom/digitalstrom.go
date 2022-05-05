package digitalstrom

import (
	"time"

	"github.com/gaetancollaud/digitalstrom-mqtt/digitalstrom/client"
	"github.com/gaetancollaud/digitalstrom-mqtt/pkg/config"
	"github.com/rs/zerolog/log"
)

type Digitalstrom struct {
	config         *config.Config
	cron           DigitalstromCron
	httpClient     client.DigitalStromClient
	eventsManager  *EventsManager
	devicesManager *DevicesManager
	circuitManager *CircuitsManager
	sceneManager   *SceneManager
}

type DigitalstromCron struct {
	ticker     *time.Ticker
	tickerDone chan bool
}

func New(config *config.Config) *Digitalstrom {
	ds := new(Digitalstrom)
	ds.config = config
	// First create the event manager in order to create a good callback for
	// events in the DigitalStrom client.
	ds.eventsManager = NewDigitalstromEvents()
	clientOptions := client.NewClientOptions().
		SetHost(config.Digitalstrom.Host).
		SetPort(config.Digitalstrom.Port).
		SetUsername(config.Digitalstrom.Username).
		SetPassword(config.Digitalstrom.Password).
		SetEventsToSubscribe([]client.EventType{
			client.EventCallScene,
			client.EventButtonClick,
			client.EventModelReady,
		}).
		SetOnEventHandler(func(c client.DigitalStromClient, event client.Event) {
			ds.eventsManager.events <- event
		})
	ds.httpClient = client.NewClient(clientOptions)
	ds.httpClient.Connect()
	// ds.httpClient = client.NewDigitalStromClient(&config.Digitalstrom)
	ds.devicesManager = NewDevicesManager(ds.httpClient, config.InvertBlindsPosition)
	ds.circuitManager = NewCircuitManager(ds.httpClient)
	ds.sceneManager = NewSceneManager(ds.httpClient)
	return ds
}

func (ds *Digitalstrom) Start() {
	log.Info().Msg("Staring digitalstrom")
	ds.cron.ticker = time.NewTicker(30 * time.Second)
	ds.cron.tickerDone = make(chan bool)
	go ds.digitalstromCron()

	ds.circuitManager.Start()
	ds.devicesManager.Start()

	go ds.circuitManager.UpdateCircuitsValue()

	go ds.eventReceived(ds.eventsManager.events)

	if ds.config.RefreshAtStart {
		go ds.refreshAllDevices()
	}
}

func (ds *Digitalstrom) Stop() {
	log.Info().Msg("Stopping digitalstrom")
	if ds.cron.ticker != nil {
		ds.cron.ticker.Stop()
		ds.cron.tickerDone <- true
		ds.cron.ticker = nil
	}
	ds.httpClient.Disconnect()
}

func (ds *Digitalstrom) digitalstromCron() {
	for {
		select {
		case <-ds.cron.tickerDone:
			return
		case <-ds.cron.ticker.C:
			log.Debug().Msg("Updating circuits values")
			ds.circuitManager.UpdateCircuitsValue()
		}
	}
}

func (ds *Digitalstrom) eventReceived(events chan client.Event) {
	for event := range events {
		log.Info().
			Int("SceneId", event.Properties.SceneId).
			Int("GroupId", event.Properties.GroupId).
			Int("ZoneId", event.Properties.ZoneId).
			Bool("isApartment", event.Source.IsApartment).
			Bool("isDevice", event.Source.IsDevice).
			Bool("isGroup", event.Source.IsGroup).
			Msg("Event received, updating devices")

		ds.sceneManager.EventReceived(event)

		ds.devicesManager.updateZone(event.Properties.ZoneId)

		if event.Source.IsGroup && event.Properties.GroupId >= 10 {
			// event is from a group, and it's not a build in groups
			ds.devicesManager.updateGroup(event.Properties.GroupId)
		}

		time.AfterFunc(2*time.Second, func() {
			// update again because maybe the three was not up to date yet
			ds.devicesManager.updateZone(event.Properties.ZoneId)

			if event.Source.IsGroup && event.Properties.GroupId >= 10 {
				ds.devicesManager.updateGroup(event.Properties.GroupId)
			}
		})
	}
}

func (ds *Digitalstrom) GetDeviceChangeChannel() chan DeviceStateChanged {
	return ds.devicesManager.deviceStateChan
}

func (ds *Digitalstrom) GetCircuitChangeChannel() chan CircuitValueChanged {
	return ds.circuitManager.circuitValuesChan
}

func (ds *Digitalstrom) GetSceneEventsChannel() chan SceneEvent {
	return ds.sceneManager.sceneEvent
}

func (ds *Digitalstrom) SetDeviceValue(command DeviceCommand) error {
	return ds.devicesManager.SetValue(command)
}

func (ds *Digitalstrom) refreshAllDevices() {
	log.Info().
		Int("size", len(ds.devicesManager.devices)).
		Msg("Refreshing all devices")
	for _, device := range ds.devicesManager.devices {
		ds.devicesManager.updateDevice(device)
	}
}

func (ds *Digitalstrom) GetAllDevices() []client.Device {
	return ds.devicesManager.devices
}

func (ds *Digitalstrom) GetAllCircuits() []client.Circuit {
	return ds.circuitManager.circuits
}
