package tool

import (
	"errors"
	gadb2 "trek/pkg/driver/android/gadb"
)

func GetAndroidDevice(client gadb2.Client, serial string) (*gadb2.Device, error) {
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
