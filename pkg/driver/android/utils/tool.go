package utils

import (
	"errors"
	"trek/pkg/driver/android/adb"
)

// GetDevice 通过设备序列号获取device实例
func GetDevice(deviceSerial string) (*adb.Device, error) {

	adbClient, err := adb.NewClient()
	if err != nil {
		return nil, err
	}

	devices, err := adbClient.DeviceList()
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, errors.New("adb did not find any available devices")
	}
	if deviceSerial == "" {
		return &devices[0], nil
	} else {
		for _, dev := range devices {
			if dev.Serial() == deviceSerial {
				return &dev, nil
			}
		}
		return nil, errors.New("the device with the specified serial number was not found")
	}
}
