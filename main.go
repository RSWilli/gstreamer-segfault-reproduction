package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"golang.org/x/sync/semaphore"
)

const iterations = 20

var sem = semaphore.NewWeighted(2)

var elements map[int]*source = make(map[int]*source)

func main() {
	gst.Init(nil)

	pipeline := newPipeline()

	go pipeline.run()

	pipeline.bin.BlockSetState(gst.StatePlaying)

	for i := 0; i < iterations; i++ {
		i := i
		sem.Acquire(context.Background(), 1)

		fmt.Printf("adding element %d\n", i)

		pad := pipeline.GetPad()

		var s *source

		s = NewSource(i, func() {
			fmt.Printf("removing %d on EOS\n", i)
			sem.Release(1)

			pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("source-%d-1", i))

			time.Sleep(1 * time.Second)

			pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("source-%d-2", i))

			s.bin.BlockSetState(gst.StateNull)

			pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("source-%d-3", i))

			time.Sleep(1 * time.Second)

			pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("source-%d-4", i))

			pipeline.Mixer().ReleaseRequestPad(pad)

			pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("source-%d-5", i))

			time.Sleep(1 * time.Second)

			pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("source-%d-6", i))

			s.pad.Unlink(pad)

			pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("source-%d-7", i))

			time.Sleep(1 * time.Second)

			pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("source-%d-8", i))

			// pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("removing-%d", i))
			pipeline.pipeline.Remove(s.bin.Element)

			pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("source-%d-9", i))

			s.bin.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("sourcein--b%d", i))

			time.Sleep(1 * time.Second)

			delete(elements, i)

			runtime.GC()

			if len(elements) == 0 {
				pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, "pipeline-end")

				// pipeline.pipeline.BlockSetState(gst.StateNull)

				// gst.Deinit()

				os.Exit(0)
			} else {
				fmt.Printf("we have %d elements left in the pipeline", len(elements))

				pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("pipeline-%d", i))

			}
		})

		err := pipeline.pipeline.Add(s.bin.Element)

		if err != nil {
			panic(err)
		}

		if s.pad.Link(pad) != gst.PadLinkOK {
			panic("could not link")
		}

		s.setFadeCurve()

		s.bin.SyncStateWithParent()

		elements[i] = s

		// pipeline.pipeline.DebugBinToDotFile(gst.DebugGraphShowAll, fmt.Sprintf("pipeline-%d", i))
	}

	runtime.Goexit()
}

type pipeline struct {
	elementChain
	pipeline *gst.Pipeline
}

func newPipeline() *pipeline {
	var err error
	p := &pipeline{}

	p.pipeline, err = gst.NewPipeline("")

	p.bin = p.pipeline.Bin

	if err != nil {
		panic(err)
	}

	p.AddElement(Must(gst.NewElementWithProperties("audiomixer", map[string]interface{}{
		"force-live":           true,
		"min-upstream-latency": uint64(500 * time.Microsecond),
	})))

	// p.AddElement(Must(gst.NewElementWithProperties("flacenc", map[string]interface{}{
	// 	"quality":     0,
	// 	"seekpoints":  0,
	// 	"hard-resync": true,
	// })))

	// p.AddElement(Must(gst.NewElement("oggmux")))

	// p.AddElement(Must(gst.NewElement("identity")))

	// p.AddElement(Must(gst.NewElementWithProperties("fakesink", map[string]interface{}{
	// 	"sync": true,
	// })))
	p.AddElement(Must(gst.NewElementWithProperties("autoaudiosink", map[string]interface{}{
		"sync": true,
	})))
	// p.AddElement(Must(gst.NewElementWithProperties("filesink", map[string]interface{}{
	// 	"sync":     true,
	// 	"location": "out.ogg",
	// })))

	// p.AddElement(Must(gst.NewElementWithProperties("shout2send", map[string]interface{}{
	// 	"sync":     true,
	// 	"ip":       "127.0.0.1",
	// 	"port":     30000,
	// 	"mount":    "/test",
	// 	"username": "source",
	// 	"password": "xmix",

	// 	// this has to be done by the icecastmetaUpdater, since we do not have a container capable of carrying metadata here
	// 	"send-title-info": false,
	// 	// {VERSION} will be substituted with the gstreamer version as of gstreamer 1.24
	// 	"user-agent": fmt.Sprintf("streaMonkey xMix %s using GStreamer {VERSION}", "test"),
	// })))

	// force stereo and sampling rate
	// caps := gst.NewCapsFromString("audio/x-raw, channels=2, rate=44100")

	// err = p.els[0].LinkFiltered(p.els[1], caps)

	// if err != nil {
	// 	panic(err)
	// }

	// gst.ElementLinkMany(p.els[1:]...)
	gst.ElementLinkMany(p.els...)

	err = p.pipeline.BlockSetState(gst.StatePlaying)

	if err != nil {
		panic(err)
	}

	return p
}

func (p *pipeline) Mixer() *gst.Element {
	return p.els[0]
}

func (p *pipeline) GetPad() *gst.Pad {
	return p.Mixer().GetRequestPad("sink_%u")
}

// run the glib mainloop
func (p *pipeline) run() {
	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)

	p.pipeline.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {
		case gst.MessageEOS:
			// When end-of-stream is received flush the pipeling and stop the main loop
			// this should never happen since our mixer outputs silence when nothing is connected
			fmt.Print("Received EOS, Flushing pipeline and stopping main loop")

			p.pipeline.BlockSetState(gst.StateNull)
			mainLoop.Quit()
		case gst.MessageError:

			fmt.Println("gStreamer pipeline errored")

			gsterr := msg.ParseError()

			errmsg := gsterr.Error()

			if errmsg != "Could not write to resource." && errmsg != "Could not connect to server" {
				fmt.Println("File not found, this may result in silence")
			} else {
				// the gstshout2send errored and we will restart

				fmt.Printf("ERROR: %s", errmsg)

				if debug := gsterr.DebugString(); debug != "" {
					fmt.Printf("DEBUG: %s\n", debug)
				}

				fmt.Print("flushing pipeline")

				err := p.pipeline.BlockSetState(gst.StateNull)

				if err != nil {
					fmt.Printf("could not flush pipeline: %v", err)
					panic("flushing failed")
				}

				// go p.restartDelayed()
			}
		default:
			// All messages implement a Stringer. However, this is
			// typically an expensive thing to do and should be avoided.
			fmt.Printf("%s\n", msg)

			return true
		}

		return true
	})

	fmt.Println("starting mainloop")
	mainLoop.Run()
	fmt.Println("mainloop ended")
}
