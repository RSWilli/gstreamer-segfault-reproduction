package main

import (
	"fmt"
	"time"

	"github.com/go-gst/go-gst/gst"
)

type source struct {
	elementChain

	pad *gst.GhostPad

	controlSource *gst.InterpolationControlSource
}

var files []string = []string{
	"sine.mp3",
	"sine.wav",
	"sine2channel.mp3",
	"sine2channel.wav",
	"sine2channel48k.mp3",
	"sine2channel48k.wav",
	"wn2channel48k.mp3",
}

func fileLocation(i int) string {
	index := i % len(files)

	return files[index]
}

func NewSource(i int, onEos func()) *source {
	s := &source{}

	offset := 500 * time.Millisecond * time.Duration(i)

	s.bin = gst.NewBin(fmt.Sprintf("bin-%d", i))

	s.AddElement(Must(gst.NewElementWithProperties("filesrc", map[string]interface{}{
		"location": fileLocation(i),
	})))

	s.AddElement(Must(gst.NewElementWithProperties("decodebin", map[string]interface{}{
		// "location": fileLocation(i),
	})))

	s.AddElement(Must(gst.NewElementWithProperties("volume", map[string]interface{}{
		// "num-buffers": 500,
	})))

	s.AddElement(Must(gst.NewElementWithProperties("audioconvert", map[string]interface{}{
		// "num-buffers": 500,
	})))

	s.AddElement(Must(gst.NewElementWithProperties("pitch", map[string]interface{}{
		// "num-buffers": 500,
	})))

	s.AddElement(Must(gst.NewElementWithProperties("audioconvert", map[string]interface{}{
		// "num-buffers": 500,
	})))

	s.AddElement(Must(gst.NewElementWithProperties("audioresample", map[string]interface{}{
		// "num-buffers": 500,
	})))

	s.AddElement(Must(gst.NewElementWithProperties("queue", map[string]interface{}{
		// "num-buffers": 500,
	})))

	gst.ElementLinkMany(s.els[0], s.els[1])

	handle, err := s.els[1].Connect("pad-added", func(e *gst.Element, pad *gst.Pad) {
		fmt.Printf("pad added callback fired for %d\n", i)
		s.els[1].Link(s.els[2])
	})

	if err != nil {
		panic(err)
	}

	gst.ElementLinkMany(s.els[2:]...)

	s.pad = gst.NewGhostPad(fmt.Sprintf("gp-%d", i), s.LastPad())

	s.bin.AddPad(s.pad.Pad)

	s.pad.SetOffset(int64(offset))

	var probe uint64

	probe = s.pad.AddProbe(gst.PadProbeTypeEventDownstream, func(pad *gst.Pad, ppi *gst.PadProbeInfo) gst.PadProbeReturn {
		ev := ppi.GetEvent()
		if ev != nil && ev.Type() == gst.EventTypeEOS {
			go func() {
				s.pad.RemoveProbe(probe)
				s.els[1].HandlerDisconnect(handle)

				onEos()
			}()

		}

		return gst.PadProbeOK
	})

	s.controlSource = gst.NewInterpolationControlSource()
	s.controlSource.SetInterpolationMode(gst.InterpolationModeLinear) // maybe we need to change this?
	binding := gst.NewDirectControlBinding(s.Volume().Object, "volume", s.controlSource)
	s.Volume().AddControlBinding(&binding.ControlBinding)

	return s
}

func (s *source) Volume() *gst.Element {
	return s.els[2]
}

type playbackSettings struct {
	// all markers are relative to the track start and before speedup
	cutIn   time.Duration
	fadeIn  time.Duration
	cutOut  time.Duration
	fadeOut time.Duration
}

// TODO: clear fade values that were set before, this may need a flush of the already processed buffers :/
func (p *source) setFadeCurve() bool {
	// fade curve values:
	//
	//
	// gstreamer doesn't like it when two different volumes are set on the same point. To "simulate"
	// a cutoff without fade aka fade==cut, we perform a fade of 1ns

	markers := playbackSettings{
		cutIn:   200 * time.Millisecond,
		fadeIn:  300 * time.Millisecond,
		fadeOut: 800 * time.Millisecond,
		cutOut:  900 * time.Millisecond,
	}

	cutIn := gst.ClockTime(markers.cutIn)
	fadeIn := gst.ClockTime(markers.fadeIn)
	cutOut := gst.ClockTime(markers.cutOut)
	fadeOut := gst.ClockTime(markers.fadeOut)

	if markers.cutIn == markers.fadeIn {
		// shift the fadein by 1ns
		fadeIn += 1

		// xmixLogger.Logger.Debug("shifting fadeIn because it is equal to cutIn")
	}

	if markers.fadeOut == markers.cutOut {
		// shift the fadeout by -1ns
		fadeOut -= 1

		// xmixLogger.Logger.Debug("shifting fadeIn because it is equal to cutIn")
	}

	// xmixLogger.Logger.Debugf("setting cutIn=%s fadeIn=%s fadeOut=%s cutOut=%s", time.Duration(cutIn), time.Duration(fadeIn), time.Duration(fadeOut), time.Duration(cutOut))

	success := true

	success = success && p.controlSource.SetTimedValue(cutIn, 0)
	success = success && p.controlSource.SetTimedValue(fadeIn, toFaderVolume(1))
	success = success && p.controlSource.SetTimedValue(fadeOut, toFaderVolume(1))
	success = success && p.controlSource.SetTimedValue(cutOut, 0)

	return success
}

// since the volume prop of the gst.volume node has range 0-10 we map 0-1 to it
//
// @param ratio a value between 0 and 1
func toFaderVolume(ratio float64) float64 {
	return ratio / 10
}
