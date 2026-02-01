package driver

import (
	"errors"
	"trek/pkg/gadb"
)

func GetAndroidDevice(client gadb.Client, serial string) (*gadb.Device, error) {
	devices, err := client.DeviceList()
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, errors.New("not connect device")
	}
	if serial == "" {
		return &devices[0], nil
	} else {
		for _, dev := range devices {
			if dev.Serial() == serial {
				return &dev, nil
			}
		}
		return nil, errors.New("not connect device")
	}
}
