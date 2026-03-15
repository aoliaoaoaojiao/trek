package screen

import (
	"fmt"
	"os"
	"testing"
	"time"
	gadb2 "trek/pkg/driver/android/gadb"
	"trek/pkg/driver/android/utils"

	"github.com/yapingcat/gomedia/go-codec"
	"github.com/yapingcat/gomedia/go-mp4"
)

var (
	device *gadb2.Device
)

func init() {
	device, _ = utils.GetDevice("")
}

func TestScrcpy_Start(t *testing.T) {
	scrcpy := NewScrcpy(device)

	mp4Filename := "./test" + ".mp4"

	ps, err := os.OpenFile(mp4Filename, os.O_CREATE|os.O_RDWR, 666)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ps.Close()

	muxer, err := mp4.CreateMp4Muxer(ps)
	if err != nil {
		panic(err)
	}

	pid := muxer.AddVideoTrack(mp4.MP4_CODEC_H264)

	initPts := uint64(0)
	//dts := uint64(0)
	isInit := false
	//start := time.Now()

	scrcpy.SetVideoFrameHandler(func(frameData []byte, oriPTS uint64, isKeyFrame bool) {
		//fmt.Println("video frame size:", len(frameData))

		if !isInit {
			initPts = oriPTS
			isInit = true
		}

		codec.SplitFrameWithStartCode(frameData, func(nalu []byte) bool {

			//pts := uint64(time.Now().UnixMilli() - start.UnixMilli())

			pts := (oriPTS - initPts) / 1000

			muxer.Write(pid, nalu, pts, pts)
			return true
		})

	})

	//scrcpy.SetVideoFrameHandler(func(bytes []byte) {
	//	ps.Write(bytes)
	//})

	err = scrcpy.Start(1000)

	if err != nil {
		panic(err)
	}
	time.Sleep(30 * time.Second)

	err = muxer.WriteTrailer()
	if err != nil {
		panic(err)
	}
}
