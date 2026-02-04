package driver

import "errors"

var NoADBDeviceErr = errors.New("no adb device found")
var NoUIAClientErr = errors.New("no uia client")
